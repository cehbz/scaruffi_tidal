"""DiscogsMetadata: MetadataProvider backed by the SQLite Discogs mirror."""

from __future__ import annotations

import sqlite3

from ..core.album import Album, TrackRef
from ..core.identifiers import ISRC, MBID, DiscogsMasterId, ExternalIds, Source
from ..core.recording import Candidate, Recording
from .mirror import MirrorDB


def _escape_fts(text: str) -> str:
    """Wrap text in double quotes for FTS5 phrase matching, escaping embedded quotes."""
    return '"' + text.replace('"', '""') + '"'


def _parse_duration(s: str | None) -> int | None:
    """Parse "M:SS" or "H:MM:SS" duration text → integer seconds. None on empty/malformed."""
    if not s:
        return None
    parts = s.strip().split(":")
    try:
        if len(parts) == 2:
            return int(parts[0]) * 60 + int(parts[1])
        if len(parts) == 3:
            return int(parts[0]) * 3600 + int(parts[1]) * 60 + int(parts[2])
    except (ValueError, IndexError):
        return None
    return None


class DiscogsMetadata:
    """MetadataProvider port backed by the local Discogs SQLite mirror (attached as `dc`)."""

    def __init__(self, db: MirrorDB, *, limit: int = 25) -> None:
        self._db = db
        self._limit = limit

    def albums_for(self, candidate: Candidate) -> list[Album]:
        con = self._db.connect()
        try:
            title_match = "title:" + _escape_fts(candidate.title)
            sql = """
                SELECT m.id, m.title, m.year, m.main_release_id
                FROM dc.master_fts f
                JOIN dc.master m ON m.id = f.rowid
                JOIN dc.master_artist ma ON ma.master_id = m.id
                JOIN dc.artist a ON a.id = ma.artist_id
                WHERE master_fts MATCH ?
                  AND a.name = ?
                GROUP BY m.id
                ORDER BY rank
                LIMIT ?
            """
            rows = con.execute(sql, (title_match, candidate.artist, self._limit)).fetchall()

            albums: list[Album] = []
            for row in rows:
                master_id = row["id"]
                albums.append(Album(
                    artist=candidate.artist,
                    title=row["title"],
                    ids=ExternalIds(
                        discogs_master_id=DiscogsMasterId(master_id),
                        sources=frozenset({Source.DISCOGS}),
                    ),
                    first_released=row["year"],
                    styles=self._styles(con, master_id),
                    tracklist=self._tracklist(con, row["main_release_id"]),
                ))
            return albums
        finally:
            con.close()

    def recordings_for(self, candidate: Candidate) -> list[Recording]:
        """Discogs has no recording-level identifier. Deferred — returns empty list."""
        return []

    # ------------------------------------------------------------------
    # private helpers
    # ------------------------------------------------------------------

    def _styles(self, con: sqlite3.Connection, master_id: int) -> frozenset[str]:
        rows = con.execute("""
            SELECT genre AS value FROM dc.master_genre WHERE master_id = ?
            UNION ALL
            SELECT style FROM dc.master_style WHERE master_id = ?
        """, (master_id, master_id)).fetchall()
        return frozenset(row[0] for row in rows)

    def _tracklist(self, con: sqlite3.Connection, main_release_id: int) -> tuple[TrackRef, ...]:
        if main_release_id is None:
            return ()
        rows = con.execute("""
            SELECT seq, position, title, duration
            FROM dc.track
            WHERE release_id = ?
            ORDER BY seq
        """, (main_release_id,)).fetchall()
        return tuple(
            TrackRef(
                position=row["seq"],
                title=row["title"],
                isrc=None,
                mbid=None,
                duration_s=_parse_duration(row["duration"]),
            )
            for row in rows
        )
