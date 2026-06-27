"""MusicBrainzMetadata: MetadataProvider backed by the SQLite MusicBrainz mirror."""

from __future__ import annotations

import sqlite3

from ..core.album import Album, ReleaseTrait, TrackRef
from ..core.identifiers import ISRC, MBID, DiscogsMasterId, ExternalIds, Source
from ..core.recording import Candidate, Credit, Performance, Recording
from .mirror import MirrorDB


def _escape_fts(text: str) -> str:
    """Wrap text in double quotes for FTS5 phrase matching, escaping embedded quotes."""
    return '"' + text.replace('"', '""') + '"'


class MusicBrainzMetadata:
    """MetadataProvider port backed by the local MusicBrainz SQLite mirror."""

    def __init__(self, db: MirrorDB, *, limit: int = 25) -> None:
        self._db = db
        self._limit = limit

    def recordings_for(self, candidate: Candidate) -> list[Recording]:
        con = self._db.connect()
        try:
            artist_id = self._resolve_artist(con, candidate)
            if artist_id is None:
                return []
            return self._query_recordings(con, candidate, artist_id)
        finally:
            con.close()

    def albums_for(self, candidate: Candidate) -> list[Album]:
        con = self._db.connect()
        try:
            artist_id = self._resolve_artist(con, candidate)
            if artist_id is None:
                return []

            title_match = _escape_fts(candidate.title)
            sql = """
                SELECT rg.id, rg.gid, rg.name, rg.discogs_master_id,
                       rgm.first_release_date_year AS first_released
                FROM release_group_fts f
                JOIN release_group rg ON rg.id = f.rowid
                JOIN artist_credit_name acn ON acn.artist_credit = rg.artist_credit
                LEFT JOIN release_group_meta rgm ON rgm.id = rg.id
                WHERE release_group_fts MATCH ?
                  AND acn.artist = ?
                GROUP BY rg.id
                ORDER BY rank
                LIMIT ?
            """
            rows = con.execute(sql, (title_match, artist_id, self._limit)).fetchall()

            albums: list[Album] = []
            for row in rows:
                rg_id = row["id"]
                raw_dmid = row["discogs_master_id"]
                discogs_master_id = DiscogsMasterId(raw_dmid) if raw_dmid is not None else None
                tracklist = self._canonical_tracklist(con, rg_id)
                albums.append(Album(
                    artist=candidate.artist,
                    title=row["name"],
                    ids=ExternalIds(
                        mbid=MBID(row["gid"]),
                        discogs_master_id=discogs_master_id,
                        sources=frozenset({Source.MUSICBRAINZ}),
                    ),
                    first_released=row["first_released"],
                    traits=self._traits(con, rg_id),
                    tracklist=tracklist,
                ))
            return albums
        finally:
            con.close()

    # ------------------------------------------------------------------
    # private helpers
    # ------------------------------------------------------------------

    def _canonical_tracklist(self, con: sqlite3.Connection, release_group_id: int) -> tuple[TrackRef, ...]:
        # Step 2: recording gids on this release-group, deterministic order
        recording_rows = con.execute("""
            SELECT r.gid
            FROM release rel
            JOIN medium m ON m.release = rel.id
            JOIN track t ON t.medium = m.id
            JOIN recording r ON r.id = t.recording
            WHERE rel.release_group = ?
            ORDER BY m.position, t.position
        """, (release_group_id,)).fetchall()
        if not recording_rows:
            return ()

        # Step 3: find the first recording gid that has a canonical release entry
        release_mbid: str | None = None
        for rec_row in recording_rows:
            row = con.execute(
                "SELECT release_mbid FROM canonical_musicbrainz_data WHERE recording_mbid = ? LIMIT 1",
                (rec_row["gid"],),
            ).fetchone()
            if row is not None:
                release_mbid = row["release_mbid"]
                break
        if release_mbid is None:
            return ()

        # Step 4: ordered tracklist for the canonical release
        rows = con.execute("""
            SELECT m.position AS disc,
                   t.position AS pos,
                   t.name     AS track_title,
                   r.gid      AS recording_gid,
                   t.length   AS duration_ms,
                   GROUP_CONCAT(i.isrc, ', ') AS isrcs
            FROM release rel
            JOIN medium    m ON m.release   = rel.id
            JOIN track     t ON t.medium    = m.id
            JOIN recording r ON r.id        = t.recording
            LEFT JOIN isrc i ON i.recording = r.id
            WHERE rel.gid = ?
            GROUP BY t.id
            ORDER BY m.position, t.position
        """, (release_mbid,)).fetchall()

        tracks: list[TrackRef] = []
        for row in rows:
            duration_ms = row["duration_ms"]
            duration_s = duration_ms // 1000 if duration_ms is not None else None

            isrcs_raw = row["isrcs"]
            first_isrc: ISRC | None = None
            if isrcs_raw:
                first_isrc = ISRC(isrcs_raw.split(",")[0].strip())

            tracks.append(TrackRef(
                position=row["pos"],
                title=row["track_title"],
                mbid=MBID(row["recording_gid"]),
                isrc=first_isrc,
                duration_s=duration_s,
            ))
        return tuple(tracks)

    def _traits(self, con: sqlite3.Connection, rg_id: int) -> frozenset[ReleaseTrait]:
        rows = con.execute("""
            SELECT st.name FROM release_group_secondary_type_join j
            JOIN release_group_secondary_type st ON st.id = j.secondary_type
            WHERE j.release_group = ?
        """, (rg_id,)).fetchall()
        _map = {"Compilation": ReleaseTrait.COMPILATION, "Live": ReleaseTrait.LIVE}
        return frozenset(_map[r["name"]] for r in rows if r["name"] in _map)

    def _resolve_artist(self, con: sqlite3.Connection, candidate: Candidate) -> int | None:
        if candidate.artist_mbid:
            row = con.execute(
                "SELECT id FROM artist WHERE gid = ?", (str(candidate.artist_mbid),)
            ).fetchone()
            return row["id"] if row else None

        phrase = _escape_fts(candidate.artist)
        row = con.execute(
            "SELECT rowid FROM artist_fts WHERE artist_fts MATCH ? ORDER BY rank LIMIT 1",
            (phrase,),
        ).fetchone()
        return row["rowid"] if row else None

    def _query_recordings(self, con: sqlite3.Connection, candidate: Candidate, artist_id: int) -> list[Recording]:
        title_match = "title:" + _escape_fts(candidate.title)
        sql = """
            SELECT r.id, r.gid, r.name, r.length, r.comment,
                   GROUP_CONCAT(i.isrc, ', ') AS isrcs
            FROM recording_fts f
            JOIN recording r ON r.id = f.rowid
            JOIN artist_credit_name acn ON acn.artist_credit = r.artist_credit
            LEFT JOIN isrc i ON i.recording = r.id
            WHERE recording_fts MATCH ?
              AND acn.artist = ?
            GROUP BY r.id
            ORDER BY rank
            LIMIT ?
        """
        rows = con.execute(sql, (title_match, artist_id, self._limit)).fetchall()
        results: list[Recording] = []
        for row in rows:
            length_ms = row["length"]
            duration_s = length_ms // 1000 if length_ms is not None else None

            isrcs_raw = row["isrcs"]
            first_isrc: ISRC | None = None
            if isrcs_raw:
                first_isrc = ISRC(isrcs_raw.split(",")[0].strip())

            performance = (
                Performance.LIVE
                if "live" in (row["comment"] or "").lower()
                else Performance.UNKNOWN
            )

            results.append(
                Recording(
                    artist=candidate.artist,
                    title=row["name"],
                    mbid=MBID(row["gid"]),
                    isrc=first_isrc,
                    duration_s=duration_s,
                    performance=performance,
                    credits=(Credit(candidate.artist, "performer"),),
                )
            )
        return results
