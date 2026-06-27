"""Integration tests for Metadata composite over the real SQLite mirrors."""
import os
import pytest

from tidalist.config import AppConfig
from tidalist.core.recording import Candidate


@pytest.mark.integration
def test_composite_albums_for_jbmd_reconciles():
    cfg = AppConfig.load()
    if not (os.path.exists(cfg.musicbrainz_db) and os.path.exists(cfg.discogs_db)):
        pytest.skip("mirror DBs not present")

    from tidalist.metadata.mirror import MirrorDB
    from tidalist.metadata.mb_mirror import MusicBrainzMetadata
    from tidalist.metadata.discogs_mirror import DiscogsMetadata
    from tidalist.metadata.composite import Metadata
    from tidalist.core.identifiers import Source

    db = MirrorDB(cfg.musicbrainz_db, cfg.discogs_db)
    svc = Metadata(MusicBrainzMetadata(db), DiscogsMetadata(db))

    candidate = Candidate("Traffic", "John Barleycorn Must Die")
    albums = svc.albums_for(candidate)

    assert albums, "expected at least one album"
    top = albums[0]
    assert top.ids.mbid is not None, "expected mbid from MusicBrainz"
    assert top.ids.discogs_master_id is not None, "expected discogs_master_id from Discogs"
    assert Source.MUSICBRAINZ in top.ids.sources
    assert Source.DISCOGS in top.ids.sources
    assert len(top.tracklist) > 0, "expected non-empty tracklist"
