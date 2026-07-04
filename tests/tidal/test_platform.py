from datetime import datetime
from types import SimpleNamespace

import pytest
from requests.exceptions import HTTPError
from tidalapi.exceptions import ObjectNotFound

from tidalist.tidal import platform as platform_module
from tidalist.tidal.platform import TidalPlatform
from tidalist.core.catalog import PlatformAlbum


def _track(id, title="t", artist="a"):
    return SimpleNamespace(id=id, name=title, artists=[SimpleNamespace(name=artist)],
                           isrc=None, album=SimpleNamespace(name="A", year=1970),
                           tidal_release_date=datetime(1970, 1, 1), duration=100)


class _FakeUser:
    def __init__(self):
        self.created = []

    def create_playlist(self, title, description):
        self.created.append((title, description))
        return SimpleNamespace(id="PL1")


class _FakePlaylist:
    def __init__(self, tracks=()):
        self.added = []
        self._tracks = list(tracks)
        self.track_calls: list[tuple[int, int]] = []

    def add(self, ids):
        self.added.append(ids)

    def tracks(self, limit=None, offset=0):
        self.track_calls.append((limit, offset))
        if limit is None:
            return self._tracks[offset:]
        return self._tracks[offset:offset + limit]


def _tidal_album(id, name="John Barleycorn Must Die", artist="Traffic", year=1970, num_tracks=6):
    return SimpleNamespace(
        id=id,
        name=name,
        artists=[SimpleNamespace(name=artist)],
        year=year,
        num_tracks=num_tracks,
    )


class _FakeAlbumObj:
    """Represents what session.album(id) returns — has a .tracks() method."""
    def __init__(self, tracks):
        self._tracks = tracks

    def tracks(self):
        return self._tracks


def _tidal_album_full(id, name, artist_name="Traffic", year=1970, num_tracks=6, discography=None):
    """Fake tidalapi album with .artist that has .get_albums()."""
    discography = discography or []

    class _FakeArtist:
        def __init__(self, name, albums):
            self.name = name
            self._albums = albums

        def get_albums(self):
            return self._albums

    fake_artist = _FakeArtist(artist_name, discography)
    return SimpleNamespace(
        id=id,
        name=name,
        artists=[SimpleNamespace(name=artist_name)],
        artist=fake_artist,
        year=year,
        num_tracks=num_tracks,
    )


class _FakeSession:
    def __init__(self, tracks=(), isrc_hits=(), albums=(), album_tracks_map=None,
                 album_obj_map=None):
        self._tracks = list(tracks)
        self._isrc_hits = list(isrc_hits)
        self._albums = list(albums)
        self._album_tracks_map = album_tracks_map or {}
        self._album_obj_map = album_obj_map or {}
        self.user = _FakeUser()
        self.pl = _FakePlaylist()
        self.searched = []

    def search(self, query, models=None, limit=50):
        self.searched.append((query, limit))
        if models and hasattr(models[0], "__name__") and models[0].__name__ == "Album":
            return {"albums": self._albums}
        return {"tracks": self._tracks}

    def get_tracks_by_isrc(self, isrc):
        return self._isrc_hits

    def playlist(self, pid):
        return self.pl

    def album(self, album_id):
        sid = str(album_id)
        if sid in self._album_obj_map:
            return self._album_obj_map[sid]
        return _FakeAlbumObj(self._album_tracks_map.get(sid, []))


def test_search_tracks_clamps_query_to_tidal_bound():
    """Tidal /v1/search returns 500 for queries over 256 chars (measured 2026-07-02);
    the adapter clamps at a token boundary before the wire."""
    session = _FakeSession()
    long_query = " ".join(["word"] * 80)  # 399 chars
    TidalPlatform(session).search_tracks(long_query)
    sent = session.searched[0][0]
    assert len(sent) <= 256
    assert long_query.startswith(sent)
    assert long_query[len(sent)] == " "  # cut lands on a token boundary


def test_search_albums_clamps_query_to_tidal_bound():
    session = _FakeSession()
    long_query = " ".join(["word"] * 80)
    TidalPlatform(session).search_albums(long_query)
    sent = session.searched[0][0]
    assert len(sent) <= 256
    assert long_query.startswith(sent)
    assert long_query[len(sent)] == " "


def test_search_tracks_clamp_hard_cuts_unbroken_query():
    """A query with no whitespace inside the bound still gets clamped."""
    session = _FakeSession()
    TidalPlatform(session).search_tracks("x" * 300)
    assert session.searched[0][0] == "x" * 256


def test_search_tracks_clamp_never_sends_empty_query():
    """A leading space before one giant token must not clamp to the empty string."""
    session = _FakeSession()
    TidalPlatform(session).search_tracks(" " + "x" * 300)
    sent = session.searched[0][0]
    assert sent
    assert len(sent) <= 256


def test_search_tracks_maps_and_limits():
    session = _FakeSession(tracks=[_track(1, "Glad"), _track(2, "Empty Pages")])
    out = TidalPlatform(session).search_tracks("Traffic", limit=1)
    assert [t.title for t in out] == ["Glad"]
    assert session.searched == [("Traffic", 1)]


def test_track_by_isrc_returns_first_hit_mapped():
    session = _FakeSession(isrc_hits=[_track(9, "Glad")])
    t = TidalPlatform(session).track_by_isrc("GBABC1234567")
    assert t.id == "9" and t.title == "Glad"


def test_track_by_isrc_none_when_no_hit():
    assert TidalPlatform(_FakeSession()).track_by_isrc("ZZ") is None


def test_track_by_isrc_none_when_object_not_found():
    """tidalapi raises ObjectNotFound for an ISRC not in the catalog; must return None."""
    class _RaisingSession:
        def get_tracks_by_isrc(self, isrc):
            raise ObjectNotFound("not in catalog")

    assert TidalPlatform(_RaisingSession()).track_by_isrc("USRE10201236") is None


def test_track_by_isrc_propagates_other_errors():
    """Only ObjectNotFound is swallowed; real errors (network/auth) propagate."""
    class _RaisingSession:
        def get_tracks_by_isrc(self, isrc):
            raise RuntimeError("network down")

    with pytest.raises(RuntimeError):
        TidalPlatform(_RaisingSession()).track_by_isrc("USRE10201236")


def test_create_playlist_returns_id():
    session = _FakeSession()
    pid = TidalPlatform(session).create_playlist("Winwood", "desc")
    assert pid == "PL1"
    assert session.user.created == [("Winwood", "desc")]


def test_add_tracks_passes_string_ids():
    session = _FakeSession()
    TidalPlatform(session).add_tracks("PL1", ["1", "2"])
    assert session.pl.added == [["1", "2"]]


# --- playlist_tracks ---

def test_playlist_tracks_returns_all_tracks_on_a_single_page():
    tracks = [_track(i, f"T{i}") for i in range(5)]
    session = _FakeSession()
    session.pl = _FakePlaylist(tracks)
    out = TidalPlatform(session).playlist_tracks("PL1")
    assert [t.title for t in out] == [f"T{i}" for i in range(5)]
    assert [t.id for t in out] == [str(i) for i in range(5)]


def test_playlist_tracks_pages_through_a_large_playlist():
    """A real playlist can hold 2000+ tracks; the adapter must loop until a
    short page signals the end, rather than trusting a single call."""
    tracks = [_track(i, f"T{i}") for i in range(250)]
    session = _FakeSession()
    session.pl = _FakePlaylist(tracks)
    out = TidalPlatform(session).playlist_tracks("PL1")
    assert len(out) == 250
    assert [t.title for t in out] == [f"T{i}" for i in range(250)]
    assert session.pl.track_calls == [(100, 0), (100, 100), (100, 200)]


def test_playlist_tracks_handles_exact_multiple_of_page_size():
    """When the last page is exactly full, one more (empty) request confirms the end."""
    tracks = [_track(i, f"T{i}") for i in range(200)]
    session = _FakeSession()
    session.pl = _FakePlaylist(tracks)
    out = TidalPlatform(session).playlist_tracks("PL1")
    assert len(out) == 200
    assert session.pl.track_calls == [(100, 0), (100, 100), (100, 200)]


def test_playlist_tracks_empty_playlist_returns_empty_list():
    session = _FakeSession()
    session.pl = _FakePlaylist([])
    assert TidalPlatform(session).playlist_tracks("PL1") == []


def test_search_albums_maps_to_catalog_album():
    ta = _tidal_album(42, name="John Barleycorn Must Die", artist="Traffic", year=1970, num_tracks=6)
    session = _FakeSession(albums=[ta])
    results = TidalPlatform(session).search_albums("Traffic John Barleycorn Must Die", limit=5)
    assert len(results) == 1
    a = results[0]
    assert isinstance(a, PlatformAlbum)
    assert a.id == "42"
    assert a.title == "John Barleycorn Must Die"
    assert a.artists == ("Traffic",)
    assert a.year == 1970
    assert a.num_tracks == 6
    assert session.searched == [("Traffic John Barleycorn Must Die", 5)]


def test_search_albums_limits_results():
    albums = [_tidal_album(i) for i in range(5)]
    session = _FakeSession(albums=albums)
    results = TidalPlatform(session).search_albums("Traffic", limit=2)
    assert len(results) == 2


def test_album_tracks_returns_mapped_tracks():
    t1 = _track(1, "Glad")
    t2 = _track(2, "Freedom Rider")
    session = _FakeSession(album_tracks_map={"99": [t1, t2]})
    tracks = TidalPlatform(session).album_tracks("99")
    assert [t.title for t in tracks] == ["Glad", "Freedom Rider"]
    assert [t.id for t in tracks] == ["1", "2"]


def test_album_tracks_returns_empty_for_unknown_id():
    session = _FakeSession()
    assert TidalPlatform(session).album_tracks("nonexistent") == []


# --- album_editions ---

def _make_discography():
    """Return a discography with two editions of Mr. Fantasy and one unrelated album."""
    mr_fantasy_10 = SimpleNamespace(
        id=101, name="Mr. Fantasy", artists=[SimpleNamespace(name="Traffic")],
        artist=None, year=1967, num_tracks=10,
    )
    mr_fantasy_22 = SimpleNamespace(
        id=102, name="Mr. Fantasy (Deluxe Edition)", artists=[SimpleNamespace(name="Traffic")],
        artist=None, year=1967, num_tracks=22,
    )
    shoot_out = SimpleNamespace(
        id=103, name="Shoot Out At The Fantasy Factory", artists=[SimpleNamespace(name="Traffic")],
        artist=None, year=1973, num_tracks=5,
    )
    return [mr_fantasy_10, mr_fantasy_22, shoot_out]


def test_album_editions_returns_sibling_editions_by_title():
    discography = _make_discography()
    anchor = _tidal_album_full(101, "Mr. Fantasy", discography=discography)
    session = _FakeSession(album_obj_map={"101": anchor})
    editions = TidalPlatform(session).album_editions("101")
    titles = [e.title for e in editions]
    assert "Mr. Fantasy" in titles
    assert "Mr. Fantasy (Deluxe Edition)" in titles
    assert "Shoot Out At The Fantasy Factory" not in titles


def test_album_editions_maps_num_tracks_correctly():
    discography = _make_discography()
    anchor = _tidal_album_full(101, "Mr. Fantasy", discography=discography)
    session = _FakeSession(album_obj_map={"101": anchor})
    editions = TidalPlatform(session).album_editions("101")
    by_id = {e.id: e for e in editions}
    assert by_id["101"].num_tracks == 10
    assert by_id["102"].num_tracks == 22


def test_album_editions_returns_empty_list_on_api_error():
    class _BrokenSession:
        def album(self, album_id):
            raise RuntimeError("API error")

    assert TidalPlatform(_BrokenSession()).album_editions("999") == []


# --- transient 5xx retry on search ---

def _http_error(status_code):
    """Build an HTTPError carrying a stub response, mirroring how requests attaches it."""
    response = SimpleNamespace(status_code=status_code)
    return HTTPError(response=response)


class _FlakySession:
    """Session whose search() raises a scripted sequence of exceptions before succeeding."""

    def __init__(self, raises=(), tracks=(), albums=()):
        self._raises = list(raises)
        self._tracks = list(tracks)
        self._albums = list(albums)
        self.calls = 0

    def search(self, query, models=None, limit=50):
        self.calls += 1
        if self._raises:
            raise self._raises.pop(0)
        if models and hasattr(models[0], "__name__") and models[0].__name__ == "Album":
            return {"albums": self._albums}
        return {"tracks": self._tracks}


def test_search_tracks_retries_transient_5xx_then_succeeds(monkeypatch):
    sleeps = []
    monkeypatch.setattr(platform_module, "_SLEEP", lambda s: sleeps.append(s))
    session = _FlakySession(raises=[_http_error(500), _http_error(500)], tracks=[_track(1, "Glad")])
    out = TidalPlatform(session).search_tracks("Traffic")
    assert [t.title for t in out] == ["Glad"]
    assert session.calls == 3
    assert sleeps == [1, 4]


def test_search_tracks_exhausts_retries_on_persistent_5xx(monkeypatch):
    sleeps = []
    monkeypatch.setattr(platform_module, "_SLEEP", lambda s: sleeps.append(s))
    session = _FlakySession(raises=[_http_error(500), _http_error(500), _http_error(503)])
    with pytest.raises(HTTPError):
        TidalPlatform(session).search_tracks("Traffic")
    assert session.calls == 3
    assert sleeps == [1, 4]


def test_search_tracks_propagates_4xx_without_retry(monkeypatch):
    sleeps = []
    monkeypatch.setattr(platform_module, "_SLEEP", lambda s: sleeps.append(s))
    session = _FlakySession(raises=[_http_error(404)])
    with pytest.raises(HTTPError):
        TidalPlatform(session).search_tracks("Traffic")
    assert session.calls == 1
    assert sleeps == []


def test_search_tracks_propagates_non_http_error_without_retry(monkeypatch):
    sleeps = []
    monkeypatch.setattr(platform_module, "_SLEEP", lambda s: sleeps.append(s))
    session = _FlakySession(raises=[RuntimeError("boom")])
    with pytest.raises(RuntimeError):
        TidalPlatform(session).search_tracks("Traffic")
    assert session.calls == 1
    assert sleeps == []


def test_search_albums_retries_transient_5xx_then_succeeds(monkeypatch):
    sleeps = []
    monkeypatch.setattr(platform_module, "_SLEEP", lambda s: sleeps.append(s))
    ta = _tidal_album(42)
    session = _FlakySession(raises=[_http_error(500), _http_error(500)], albums=[ta])
    out = TidalPlatform(session).search_albums("Traffic")
    assert len(out) == 1
    assert session.calls == 3
    assert sleeps == [1, 4]
