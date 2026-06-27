"""Read-only SQLite connection over the MusicBrainz mirror with the Discogs mirror ATTACHed."""

import sqlite3
from pathlib import Path
from urllib.parse import quote


def _ro_uri(p: Path) -> str:
    return f"file:{quote(str(p), safe='/')}?mode=ro&immutable=1"


class MirrorDB:
    """Opens the MusicBrainz mirror read-only and ATTACHes the Discogs mirror as `dc`."""

    def __init__(self, musicbrainz_db, discogs_db):
        self._mb = Path(musicbrainz_db)
        self._dc = Path(discogs_db)

    def connect(self) -> sqlite3.Connection:
        for label, p in (("musicbrainz", self._mb), ("discogs", self._dc)):
            if not p.exists():
                raise FileNotFoundError(f"{label} mirror not found: {p}")
        con = sqlite3.connect(_ro_uri(self._mb), uri=True)
        con.row_factory = sqlite3.Row
        con.execute(f"ATTACH DATABASE '{_ro_uri(self._dc)}' AS dc")
        return con
