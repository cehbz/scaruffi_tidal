"""Live integration tests for MusicBrainzMetadata (mirror-backed).

Requires the SSD at /Volumes/Crucial X10 to be mounted with the SQLite databases.
Skipped automatically when the DB paths do not exist.
"""
import os

import pytest

from tidalist.config import AppConfig
from tidalist.core.recording import Candidate, Kind


@pytest.mark.integration
def test_recordings_for_glad_finds_studio_take():
    cfg = AppConfig.load()
    if not os.path.exists(cfg.musicbrainz_db):
        pytest.skip(f"MusicBrainz mirror not mounted: {cfg.musicbrainz_db}")
    from tidalist.metadata.mirror import MirrorDB
    from tidalist.metadata.mb_mirror import MusicBrainzMetadata

    db = MirrorDB(cfg.musicbrainz_db, cfg.discogs_db)
    meta = MusicBrainzMetadata(db)
    recordings = meta.recordings_for(Candidate("Traffic", "Glad"))

    assert recordings, "expected at least one recording for 'Glad' by Traffic"
    titles = [r.title for r in recordings]
    assert any("Glad" in t for t in titles), f"no 'Glad' in {titles}"
    glad = next(r for r in recordings if "Glad" in r.title)
    assert glad.mbid, "recording must have a non-empty mbid"


@pytest.mark.integration
def test_albums_for_jbmd_has_canonical_tracklist():
    cfg = AppConfig.load()
    if not os.path.exists(cfg.musicbrainz_db):
        pytest.skip(f"MusicBrainz mirror not mounted: {cfg.musicbrainz_db}")
    from tidalist.metadata.mirror import MirrorDB
    from tidalist.metadata.mb_mirror import MusicBrainzMetadata

    db = MirrorDB(cfg.musicbrainz_db, cfg.discogs_db)
    meta = MusicBrainzMetadata(db)
    albums = meta.albums_for(Candidate("Traffic", "John Barleycorn Must Die", kind=Kind.ALBUM))

    assert albums, "expected at least one album for 'John Barleycorn Must Die' by Traffic"
    top = albums[0]
    assert top.tracklist, "canonical tracklist must not be empty"
    assert len(top.tracklist) >= 6, (
        f"expected 6+ tracks, got {len(top.tracklist)}: {[t.title for t in top.tracklist]}"
    )
    assert top.tracklist[0].title == "Glad", (
        f"expected first track 'Glad', got '{top.tracklist[0].title}'"
    )
