"""MusicBrainzMetadata: MetadataProvider backed by the SQLite MusicBrainz mirror."""

from __future__ import annotations

from ..core.identifiers import ISRC, MBID
from ..core.recording import Candidate, Credit, Recording
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

    def albums_for(self, candidate: Candidate) -> list:
        return []

    # ------------------------------------------------------------------
    # private helpers
    # ------------------------------------------------------------------

    def _resolve_artist(self, con, candidate: Candidate) -> int | None:
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

    def _query_recordings(self, con, candidate: Candidate, artist_id: int) -> list[Recording]:
        title_match = "title:" + _escape_fts(candidate.title)
        sql = """
            SELECT r.id, r.gid, r.name, r.length,
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

            results.append(
                Recording(
                    artist=candidate.artist,
                    title=row["name"],
                    mbid=MBID(row["gid"]),
                    isrc=first_isrc,
                    duration_s=duration_s,
                    credits=(Credit(candidate.artist, "performer"),),
                )
            )
        return results
