import pytest

from tidalist.metadata.mirror import MirrorDB
from tests.metadata.mirror_fixture import build_mirror_fixture


def test_mirrordb_connects_and_attaches_discogs(tmp_path):
    mb, dc = build_mirror_fixture(tmp_path)
    con = MirrorDB(mb, dc).connect()
    dbs = {row[1] for row in con.execute("PRAGMA database_list")}
    assert "main" in dbs and "dc" in dbs
    name = con.execute("SELECT name FROM artist WHERE id=9133").fetchone()[0]
    assert name == "Traffic"
    con.close()


def test_mirrordb_missing_db_fails_loudly(tmp_path):
    _, dc = build_mirror_fixture(tmp_path)
    with pytest.raises(FileNotFoundError) as exc:
        MirrorDB(str(tmp_path / "nope.db"), dc).connect()
    assert "nope.db" in str(exc.value)


def test_recordings_for_finds_studio_glad_with_isrc(tmp_path):
    from tidalist.metadata.mb_mirror import MusicBrainzMetadata
    from tidalist.core.recording import Candidate
    mb, dc = build_mirror_fixture(tmp_path)
    recs = MusicBrainzMetadata(MirrorDB(mb, dc)).recordings_for(Candidate("Traffic", "Glad"))
    assert len(recs) == 1
    r = recs[0]
    assert r.title == "Glad"
    assert r.mbid == "53bb54ac-1020-4cd4-83ce-362a58e1ec17"  # Recording keeps mbid/isrc (Slice 1)
    assert r.isrc == "GBUM71030667"
    assert r.duration_s == 419  # 419093 ms // 1000


def test_recordings_for_unknown_artist_returns_empty(tmp_path):
    from tidalist.metadata.mb_mirror import MusicBrainzMetadata
    from tidalist.core.recording import Candidate
    mb, dc = build_mirror_fixture(tmp_path)
    recs = MusicBrainzMetadata(MirrorDB(mb, dc)).recordings_for(Candidate("Nobody", "Glad"))
    assert recs == []


def test_albums_for_returns_rg_with_canonical_tracklist(tmp_path):
    from tidalist.metadata.mb_mirror import MusicBrainzMetadata
    from tidalist.core.recording import Candidate
    mb, dc = build_mirror_fixture(tmp_path)
    albums = MusicBrainzMetadata(MirrorDB(mb, dc)).albums_for(
        Candidate("Traffic", "John Barleycorn Must Die"))
    assert len(albums) == 1
    a = albums[0]
    assert a.ids.mbid == "3770d5ce-e0e1-3389-9acf-cd38f0722baf"
    assert a.ids.discogs_master_id == 69017
    from tidalist.core.identifiers import Source
    assert Source.MUSICBRAINZ in a.ids.sources
    assert [t.position for t in a.tracklist] == [1, 2, 3, 4, 5, 6]
    assert a.tracklist[0].title == "Glad"
    assert a.tracklist[0].isrc == "GBUM71030667"
    assert a.tracklist[0].duration_s == 419
    assert a.first_released == 1970


def test_recordings_for_live_take_marks_performance_live(tmp_path):
    from tidalist.metadata.mb_mirror import MusicBrainzMetadata
    from tidalist.core.recording import Candidate, Performance
    mb, dc = build_mirror_fixture(tmp_path)
    recs = MusicBrainzMetadata(MirrorDB(mb, dc)).recordings_for(
        Candidate("Traffic", "Pearly Queen"))
    assert len(recs) == 1
    assert recs[0].performance == Performance.LIVE


def test_albums_for_sets_first_released(tmp_path):
    from tidalist.metadata.mb_mirror import MusicBrainzMetadata
    from tidalist.core.recording import Candidate
    mb, dc = build_mirror_fixture(tmp_path)
    albums = MusicBrainzMetadata(MirrorDB(mb, dc)).albums_for(
        Candidate("Traffic", "John Barleycorn Must Die"))
    assert len(albums) == 1
    a = albums[0]
    assert a.first_released == 1970
    assert a.traits == frozenset()


def test_albums_for_compilation_has_trait(tmp_path):
    from tidalist.metadata.mb_mirror import MusicBrainzMetadata
    from tidalist.core.recording import Candidate
    from tidalist.core.album import ReleaseTrait
    mb, dc = build_mirror_fixture(tmp_path)
    albums = MusicBrainzMetadata(MirrorDB(mb, dc)).albums_for(
        Candidate("Traffic", "Best of Traffic"))
    assert len(albums) == 1
    album = albums[0]
    assert ReleaseTrait.COMPILATION in album.traits
    assert album.tracklist == ()
