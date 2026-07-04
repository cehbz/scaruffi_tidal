from tidalist.core.album import Album
from tidalist.core.catalog import Track
from tidalist.core.identifiers import ISRC
from tidalist.core.recording import Candidate
from tests.fakes import FakePlatform, FakeMetadataProvider


def test_fake_catalog_finds_by_isrc():
    t = Track(id="1", title="Glad", artists=("Traffic",), isrc=ISRC("X"))
    assert FakePlatform([t]).track_by_isrc(ISRC("X")) is t


def test_fake_catalog_search_matches_all_query_words():
    t = Track(id="1", title="Glad", artists=("Traffic",))
    assert FakePlatform([t]).search_tracks("Traffic Glad") == [t]
    assert FakePlatform([t]).search_tracks("Cream Glad") == []


def test_fake_catalog_create_and_add():
    cat = FakePlatform([])
    pid = cat.create_playlist("p")
    cat.add_tracks(pid, ["1", "2"])
    assert cat.playlists[pid] == ["1", "2"]


def test_fake_catalog_playlist_tracks_returns_seeded_tracks():
    t = Track(id="1", title="Glad", artists=("Traffic",))
    cat = FakePlatform([], playlist_tracks_map={"PL1": [t]})
    assert cat.playlist_tracks("PL1") == [t]


def test_fake_catalog_playlist_tracks_unknown_id_returns_empty():
    assert FakePlatform([]).playlist_tracks("nonexistent") == []


def test_fake_metadata_provider_albums_for_returns_seeded_album():
    album = Album(artist="Talk Talk", title="Spirit of Eden")
    candidate = Candidate(artist="Talk Talk", title="Spirit of Eden")
    provider = FakeMetadataProvider(albums={"Spirit of Eden": [album]})
    assert provider.albums_for(candidate) == [album]


def test_fake_metadata_provider_albums_for_case_insensitive():
    album = Album(artist="Talk Talk", title="Spirit of Eden")
    candidate = Candidate(artist="Talk Talk", title="SPIRIT OF EDEN")
    provider = FakeMetadataProvider(albums={"Spirit of Eden": [album]})
    assert provider.albums_for(candidate) == [album]


def test_fake_metadata_provider_albums_for_unknown_returns_empty():
    provider = FakeMetadataProvider(albums={"Spirit of Eden": [Album(artist="Talk Talk", title="Spirit of Eden")]})
    assert provider.albums_for(Candidate(artist="Talk Talk", title="Laughing Stock")) == []


def test_fake_metadata_provider_recordings_positional_still_works():
    from tidalist.core.recording import Recording
    rec = Recording(title="Desire", artist="Talk Talk")
    candidate = Candidate(artist="Talk Talk", title="Desire")
    provider = FakeMetadataProvider({"Desire": rec})
    assert provider.recordings_for(candidate) == [rec]


# --- FakePlatform.album_editions ---

def _make_catalog_album(id_str: str, title: str) -> "PlatformAlbum":
    from tidalist.core.catalog import PlatformAlbum
    from tidalist.core.identifiers import TrackId
    return PlatformAlbum(id=TrackId(id_str), title=title, artists=("Artist",))


def test_fake_catalog_album_editions_returns_seeded_editions():
    from tidalist.core.identifiers import TrackId
    anchor = _make_catalog_album("42", "Abbey Road")
    remaster = _make_catalog_album("99", "Abbey Road (2019 Remaster)")
    cat = FakePlatform([], album_editions_map={"42": [anchor, remaster]})
    assert cat.album_editions(TrackId("42")) == [anchor, remaster]


def test_fake_catalog_album_editions_unknown_id_returns_empty():
    from tidalist.core.identifiers import TrackId
    cat = FakePlatform([])
    assert cat.album_editions(TrackId("999")) == []


def test_fake_catalog_album_editions_no_map_returns_empty():
    from tidalist.core.identifiers import TrackId
    cat = FakePlatform([], album_editions_map=None)
    assert cat.album_editions(TrackId("1")) == []


def test_fake_catalog_search_albums_and_editions_are_independent():
    """search_albums sees only the 'searchable' album; album_editions returns all editions."""
    from tidalist.core.catalog import PlatformAlbum
    from tidalist.core.identifiers import TrackId
    searchable = _make_catalog_album("10", "Rumours")
    remaster = _make_catalog_album("11", "Rumours (2004 Remaster)")
    cat = FakePlatform(
        [],
        albums=[searchable],
        album_editions_map={"10": [searchable, remaster]},
    )
    assert cat.search_albums("Rumours") == [searchable]
    assert cat.album_editions(TrackId("10")) == [searchable, remaster]
