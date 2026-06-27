"""Offline tests for Metadata composite provider backed by the mirror fixture."""
import pytest

from tidalist.core.identifiers import Source
from tidalist.core.recording import Candidate
from tidalist.metadata.mirror import MirrorDB
from tidalist.metadata.mb_mirror import MusicBrainzMetadata
from tidalist.metadata.discogs_mirror import DiscogsMetadata
from tidalist.metadata.composite import Metadata


@pytest.fixture()
def db(tmp_path):
    from tests.metadata.mirror_fixture import build_mirror_fixture
    mb, dc = build_mirror_fixture(tmp_path)
    return MirrorDB(mb, dc)


@pytest.fixture()
def svc(db):
    return Metadata(MusicBrainzMetadata(db), DiscogsMetadata(db))


# --- albums_for ---

def test_albums_for_returns_one_album(svc):
    albums = svc.albums_for(Candidate("Traffic", "John Barleycorn Must Die"))
    assert len(albums) == 1


def test_albums_for_has_mbid(svc):
    albums = svc.albums_for(Candidate("Traffic", "John Barleycorn Must Die"))
    assert albums[0].ids.mbid is not None


def test_albums_for_has_discogs_master_id(svc):
    albums = svc.albums_for(Candidate("Traffic", "John Barleycorn Must Die"))
    assert albums[0].ids.discogs_master_id == 69017


def test_albums_for_sources_both(svc):
    albums = svc.albums_for(Candidate("Traffic", "John Barleycorn Must Die"))
    assert albums[0].ids.sources == {Source.MUSICBRAINZ, Source.DISCOGS}


def test_albums_for_tracklist_from_mb(svc):
    """MB canonical tracklist is kept (tracks have mbid)."""
    albums = svc.albums_for(Candidate("Traffic", "John Barleycorn Must Die"))
    tracklist = albums[0].tracklist
    assert len(tracklist) == 6
    assert all(t.mbid is not None for t in tracklist)


def test_albums_for_styles_from_discogs(svc):
    albums = svc.albums_for(Candidate("Traffic", "John Barleycorn Must Die"))
    assert "Folk Rock" in albums[0].styles


# --- recordings_for ---

def test_recordings_for_glad_returns_mb_recording(svc):
    recordings = svc.recordings_for(Candidate("Traffic", "Glad"))
    assert len(recordings) >= 1
    assert recordings[0].mbid is not None


def test_recordings_for_delegates_to_mb(svc):
    """Composite recordings_for returns MB results, not empty (Discogs stub)."""
    recordings = svc.recordings_for(Candidate("Traffic", "Glad"))
    assert all(r.mbid is not None for r in recordings)
