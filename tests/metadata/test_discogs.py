from types import SimpleNamespace

from tidalist.core.album import Album
from tidalist.core.recording import Candidate, Performance
from tidalist.metadata.discogs import recording_from_discogs, DiscogsMetadata, album_from_discogs

CAND = Candidate("Traffic", "Glad", album="John Barleycorn Must Die")


def _result(artists=("Traffic",), formats=None, year=1970, title="John Barleycorn Must Die"):
    return SimpleNamespace(
        artists=[SimpleNamespace(name=a) for a in artists],
        formats=formats if formats is not None else [{"name": "Vinyl", "descriptions": ["LP", "Album"]}],
        year=year,
        title=title,
    )


# --- recording_from_discogs (release-level: title/artist come from the candidate) ---

def test_title_and_artist_come_from_candidate():
    rec = recording_from_discogs(_result(), CAND)
    assert rec.title == "Glad" and rec.artist == "Traffic"


def test_album_from_release_title():
    assert recording_from_discogs(_result(title="John Barleycorn Must Die"), CAND).album \
        == "John Barleycorn Must Die"


def test_maps_release_artists_to_performer_credits():
    rec = recording_from_discogs(_result(artists=("Traffic", "Steve Winwood")), CAND)
    assert [c.artist for c in rec.credits] == ["Traffic", "Steve Winwood"]
    assert all(c.role == "performer" for c in rec.credits)


def test_first_released_from_int_year():
    assert recording_from_discogs(_result(year=1970), CAND).first_released == 1970


def test_first_released_from_string_year():
    assert recording_from_discogs(_result(year="1970"), CAND).first_released == 1970


def test_missing_year_is_none():
    assert recording_from_discogs(_result(year=0), CAND).first_released is None


def test_live_format_sets_performance_live():
    r = _result(formats=[{"name": "CD", "descriptions": ["Album", "Live"]}])
    assert recording_from_discogs(r, CAND).performance is Performance.LIVE


def test_non_live_format_is_unknown_performance():
    assert recording_from_discogs(_result(), CAND).performance is Performance.UNKNOWN


def test_isrc_is_none():
    assert recording_from_discogs(_result(), CAND).isrc is None


# --- DiscogsMetadata.recordings_for ---

class _FakeClient:
    def __init__(self, results):
        self._results = results
        self.queries = []

    def search(self, query, type=None):
        self.queries.append((query, type))
        return self._results


def test_recordings_for_maps_every_result_without_selecting():
    client = _FakeClient([_result(year=1970), _result(year=2005)])
    recs = DiscogsMetadata(client).recordings_for(Candidate("Traffic", "Glad"))
    assert [r.first_released for r in recs] == [1970, 2005]
    assert client.queries == [("Traffic Glad", "release")]


def test_recordings_for_empty_when_no_results():
    assert DiscogsMetadata(_FakeClient([])).recordings_for(Candidate("X", "Y")) == []


def test_recordings_for_bounds_results_and_does_not_walk_every_page():
    # A real discogs_client search is a lazily-paginated list; recordings_for must stop
    # at the limit, not iterate (and page-fetch) the entire result set.
    consumed = []

    def lazy_pages():
        for i in range(1000):
            consumed.append(i)
            yield _result(year=2000 + (i % 50))

    class _LazyClient:
        def search(self, query, type=None):
            return lazy_pages()

    recs = DiscogsMetadata(_LazyClient(), limit=5).recordings_for(Candidate("X", "Y"))
    assert len(recs) == 5
    assert len(consumed) == 5


def test_recordings_for_waits_on_the_limiter():
    calls = []

    class _Limiter:
        def wait(self):
            calls.append(1)

    DiscogsMetadata(_FakeClient([_result()]), limiter=_Limiter()).recordings_for(
        Candidate("Traffic", "Glad"))
    assert calls == [1]


# --- album_from_discogs ---


def _master(year=1970):
    return SimpleNamespace(year=year)


def test_album_from_discogs_artist_from_candidate():
    album = album_from_discogs(_master(), CAND)
    assert album.artist == "Traffic"


def test_album_from_discogs_title_from_candidate():
    album = album_from_discogs(_master(), CAND)
    assert album.title == "Glad"


def test_album_from_discogs_mbid_is_none():
    assert album_from_discogs(_master(), CAND).ids.mbid is None


def test_album_from_discogs_year_from_master():
    assert album_from_discogs(_master(year=1970), CAND).first_released == 1970


# --- DiscogsMetadata.albums_for ---

def test_albums_for_maps_masters_to_albums():
    client = _FakeClient([_master(1970), _master(1971)])
    albums = DiscogsMetadata(client).albums_for(CAND)
    assert len(albums) == 2
    assert all(a.ids.mbid is None for a in albums)
    assert albums[0].first_released == 1970


def test_albums_for_candidate_artist_and_title_in_query():
    client = _FakeClient([_master()])
    DiscogsMetadata(client).albums_for(CAND)
    assert client.queries == [("Traffic Glad", "master")]


def test_albums_for_islice_bounded_lazy():
    consumed = []

    def lazy_masters():
        for i in range(1000):
            consumed.append(i)
            yield _master(year=2000 + (i % 50))

    class _LazyClient:
        def search(self, query, type=None):
            return lazy_masters()

    albums = DiscogsMetadata(_LazyClient(), limit=3).albums_for(CAND)
    assert len(albums) == 3
    assert len(consumed) == 3


def test_albums_for_waits_on_limiter():
    calls = []

    class _Limiter:
        def wait(self):
            calls.append(1)

    DiscogsMetadata(_FakeClient([_master()]), limiter=_Limiter()).albums_for(CAND)
    assert calls == [1]
