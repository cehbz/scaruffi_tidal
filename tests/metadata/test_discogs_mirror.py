"""Offline tests for DiscogsMetadata backed by the SQLite mirror fixture."""
import pytest

from tidalist.core.identifiers import Source
from tidalist.core.recording import Candidate
from tidalist.metadata.mirror import MirrorDB


@pytest.fixture()
def mirror(tmp_path):
    from tests.metadata.mirror_fixture import build_mirror_fixture
    mb, dc = build_mirror_fixture(tmp_path)
    return MirrorDB(mb, dc)


@pytest.fixture()
def svc(mirror):
    from tidalist.metadata.discogs_mirror import DiscogsMetadata
    return DiscogsMetadata(mirror)


# --- albums_for ---

def test_albums_for_returns_one_album_for_jbmd(svc):
    albums = svc.albums_for(Candidate("Traffic", "John Barleycorn Must Die"))
    assert len(albums) == 1


def test_albums_for_discogs_master_id(svc):
    albums = svc.albums_for(Candidate("Traffic", "John Barleycorn Must Die"))
    assert albums[0].ids.discogs_master_id == 69017


def test_albums_for_source_discogs(svc):
    albums = svc.albums_for(Candidate("Traffic", "John Barleycorn Must Die"))
    assert Source.DISCOGS in albums[0].ids.sources


def test_albums_for_first_released(svc):
    albums = svc.albums_for(Candidate("Traffic", "John Barleycorn Must Die"))
    assert albums[0].first_released == 1970


def test_albums_for_styles_includes_genre(svc):
    albums = svc.albums_for(Candidate("Traffic", "John Barleycorn Must Die"))
    assert "Rock" in albums[0].styles


def test_albums_for_styles_includes_style(svc):
    albums = svc.albums_for(Candidate("Traffic", "John Barleycorn Must Die"))
    assert "Folk Rock" in albums[0].styles


def test_albums_for_tracklist_positions(svc):
    albums = svc.albums_for(Candidate("Traffic", "John Barleycorn Must Die"))
    assert [t.position for t in albums[0].tracklist] == [1, 2, 3, 4, 5, 6]


def test_albums_for_tracklist_first_title(svc):
    albums = svc.albums_for(Candidate("Traffic", "John Barleycorn Must Die"))
    assert albums[0].tracklist[0].title == "Glad"


def test_albums_for_tracklist_first_duration(svc):
    albums = svc.albums_for(Candidate("Traffic", "John Barleycorn Must Die"))
    assert albums[0].tracklist[0].duration_s == 392  # "6:32" → 6*60+32


def test_albums_for_tracklist_first_mbid_is_none(svc):
    albums = svc.albums_for(Candidate("Traffic", "John Barleycorn Must Die"))
    assert albums[0].tracklist[0].mbid is None


def test_albums_for_unknown_artist_returns_empty(svc):
    albums = svc.albums_for(Candidate("Nobody", "John Barleycorn Must Die"))
    assert albums == []


def test_albums_for_unknown_title_returns_empty(svc):
    albums = svc.albums_for(Candidate("Traffic", "No Such Album"))
    assert albums == []


# --- recordings_for (deferred stub) ---

def test_recordings_for_returns_empty(svc):
    assert svc.recordings_for(Candidate("Traffic", "Glad")) == []
