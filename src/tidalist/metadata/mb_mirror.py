"""MusicBrainzMetadata: MetadataProvider backed by the SQLite MusicBrainz mirror."""

from __future__ import annotations

from ..core.album import Album, TrackRef
from ..core.identifiers import ISRC, MBID, DiscogsMasterId, ExternalIds, Source
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

    def albums_for(self, candidate: Candidate) -> list[Album]:
        con = self._db.connect()
        try:
            artist_id = self._resolve_artist(con, candidate)
            if artist_id is None:
                return []

            title_match = _escape_fts(candidate.title)
            sql = """
                SELECT rg.id, rg.gid, rg.name, rg.discogs_master_id
                FROM release_group_fts f
                JOIN release_group rg ON rg.id = f.rowid
                JOIN artist_credit_name acn ON acn.artist_credit = rg.artist_credit
                WHERE release_group_fts MATCH ?
                  AND acn.artist = ?
                ORDER BY rank
                LIMIT ?
            """
            rows = con.execute(sql, (title_match, artist_id, self._limit)).fetchall()

            albums: list[Album] = []
            for row in rows:
                raw_dmid = row["discogs_master_id"]
                discogs_master_id = DiscogsMasterId(raw_dmid) if raw_dmid is not None else None
                tracklist = self._canonical_tracklist(con, row["id"])
                albums.append(Album(
                    artist=candidate.artist,
                    title=row["name"],
                    ids=ExternalIds(
                        mbid=MBID(row["gid"]),
                        discogs_master_id=discogs_master_id,
                        sources=frozenset({Source.MUSICBRAINZ}),
                    ),
                    first_released=None,
                    tracklist=tracklist,
                ))
            return albums
        finally:
            con.close()

    # ------------------------------------------------------------------
    # private helpers
    # ------------------------------------------------------------------

    def _canonical_tracklist(self, con, release_group_id: int) -> tuple[TrackRef, ...]:
        # Step 2: any recording gid on this release-group (all joins indexed)
        row = con.execute("""
            SELECT r.gid
            FROM release rel
            JOIN medium m ON m.release = rel.id
            JOIN track t ON t.medium = m.id
            JOIN recording r ON r.id = t.recording
            WHERE rel.release_group = ?
            LIMIT 1
        """, (release_group_id,)).fetchone()
        if row is None:
            return ()

        recording_gid = row["gid"]

        # Step 3: canonical release_mbid for this recording (indexed entry point)
        row = con.execute(
            "SELECT release_mbid FROM canonical_musicbrainz_data WHERE recording_mbid = ?",
            (recording_gid,),
        ).fetchone()
        if row is None:
            return ()

        release_mbid = row["release_mbid"]

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
