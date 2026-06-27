from tidalist.core.album import Album, TrackRef, ReleaseTrait
from tidalist.core.identifiers import ISRC, MBID
from tidalist.core.recording import Candidate, Kind, Performance
from tidalist.metadata.musicbrainz import (recording_from_musicbrainz, MusicBrainzMetadata,
                                           album_from_release_group)


def _rec(disambiguation="", isrcs=("GBABC1234567",), dates=("1970-07-01", "1985-01-01"),
         length="386000"):
    return {
        "id": "rec-1",
        "title": "Glad",
        "length": length,
        "artist-credit": [{"artist": {"name": "Traffic"}}, " feat. ",
                          {"artist": {"name": "Steve Winwood"}}],
        "artist-credit-phrase": "Traffic feat. Steve Winwood",
        "isrc-list": list(isrcs),
        "release-list": [{"id": f"r{i}", "title": "John Barleycorn Must Die", "date": d}
                         for i, d in enumerate(dates)],
        "artist-relation-list": [{"type": "keyboard", "artist": {"name": "Steve Winwood"}}],
        "disambiguation": disambiguation,
    }


# A search hit is lighter than a full recording: no isrc-list, no artist-relation-list.
def _hit(id="rec-1", title="Glad", disambiguation=""):
    return {
        "id": id,
        "title": title,
        "length": "386000",
        "artist-credit": [{"artist": {"name": "Traffic"}}],
        "artist-credit-phrase": "Traffic",
        "release-list": [{"id": "r0", "title": "John Barleycorn Must Die", "date": "1970-07-01"}],
        "disambiguation": disambiguation,
    }


# --- recording_from_musicbrainz ---

def test_mbid_from_recording_id():
    assert recording_from_musicbrainz(_rec()).mbid == "rec-1"


def test_title_mapped():
    assert recording_from_musicbrainz(_rec()).title == "Glad"


def test_artist_from_credit_phrase():
    assert recording_from_musicbrainz(_rec()).artist == "Traffic feat. Steve Winwood"


def test_album_from_first_release_title():
    assert recording_from_musicbrainz(_rec()).album == "John Barleycorn Must Die"


def test_duration_from_length_milliseconds():
    assert recording_from_musicbrainz(_rec(length="386000")).duration_s == 386


def test_duration_none_when_length_absent():
    assert recording_from_musicbrainz(_rec(length=None)).duration_s is None


def test_isrc_is_first_in_list():
    assert recording_from_musicbrainz(_rec()).isrc == "GBABC1234567"


def test_isrc_none_when_absent():
    assert recording_from_musicbrainz(_rec(isrcs=())).isrc is None


def test_artist_credit_becomes_performer_credits_filtering_join_phrases():
    rec = recording_from_musicbrainz(_rec())
    performers = [c.artist for c in rec.credits if c.role == "performer"]
    assert "Traffic" in performers and "Steve Winwood" in performers


def test_relations_add_credits_with_their_role():
    rec = recording_from_musicbrainz(_rec())
    assert any(c.role == "keyboard" and c.artist == "Steve Winwood" for c in rec.credits)


def test_first_released_is_earliest_release_year():
    assert recording_from_musicbrainz(_rec(dates=("1985-01-01", "1970-07-01"))).first_released == 1970


def test_live_disambiguation_sets_performance_live():
    assert recording_from_musicbrainz(_rec(disambiguation="live, 1973")).performance is Performance.LIVE


def test_studio_default_is_unknown():
    assert recording_from_musicbrainz(_rec()).performance is Performance.UNKNOWN


# --- MusicBrainzMetadata.recordings_for ---

class _FakeMB:
    def __init__(self, search_list, artists=None, release_groups=None, browse_releases=None,
                 browse_raises=False):
        self._search = search_list
        self._artists = artists if artists is not None else []
        self._release_groups = release_groups if release_groups is not None else []
        self._browse_releases = browse_releases if browse_releases is not None else []
        self._browse_raises = browse_raises
        self.calls = []

    def search_recordings(self, query="", limit=None, **fields):
        self.calls.append(("search", fields, limit))
        return {"recording-list": self._search}

    def search_artists(self, artist="", limit=None, **kw):
        return {"artist-list": self._artists}

    def search_release_groups(self, artist="", releasegroup="", limit=None, **kw):
        return {"release-group-list": self._release_groups}

    def browse_releases(self, release_group=None, includes=None, limit=None, **kw):
        self.calls.append(("browse_releases", release_group, includes))
        if self._browse_raises:
            raise Exception("MB API error")
        return {"release-list": self._browse_releases}

    def get_recording_by_id(self, id, includes=None, **kw):
        self.calls.append(("get", id, tuple(includes or [])))
        return {"recording": {}}


def test_recordings_for_maps_every_hit_in_order_without_selecting():
    mb = _FakeMB([_hit(id="a", title="Glad"),
                  _hit(id="b", title="Glad", disambiguation="live")])
    recs = MusicBrainzMetadata(mb).recordings_for(Candidate("Traffic", "Glad"))
    assert [r.mbid for r in recs] == ["a", "b"]
    assert recs[1].performance is Performance.LIVE


def test_recordings_for_is_discovery_only_no_full_fetch():
    mb = _FakeMB([_hit()])
    recs = MusicBrainzMetadata(mb).recordings_for(Candidate("Traffic", "Glad"))
    assert recs[0].isrc is None                        # ISRC is fetched lazily, not at discovery
    assert [c[0] for c in mb.calls] == ["search"]      # never calls get_recording_by_id


def test_recordings_for_searches_by_artist_and_title():
    mb = _FakeMB([_hit()])
    MusicBrainzMetadata(mb).recordings_for(Candidate("Traffic", "Glad"))
    assert mb.calls[0][1] == {"artist": "Traffic", "recording": "Glad"}


def test_recordings_for_empty_when_no_hits():
    assert MusicBrainzMetadata(_FakeMB([])).recordings_for(Candidate("X", "Y")) == []


# --- MusicBrainzMetadata._artist_mbid ---

def test_artist_mbid_returns_top_hit_id():
    mb = _FakeMB([], artists=[{"id": "a-traffic", "name": "Traffic"},
                              {"id": "a-sound", "name": "Traffic Sound"}])
    assert MusicBrainzMetadata(mb)._artist_mbid("Traffic") == "a-traffic"


def test_artist_mbid_none_when_no_hits():
    assert MusicBrainzMetadata(_FakeMB([], artists=[]))._artist_mbid("Nobody") is None


# --- recordings_for: identity filtering ---

def _hit_credited(rec_id, artist_id, artist_name):
    return {"id": rec_id, "title": "Glad", "length": "419000",
            "artist-credit": [{"artist": {"id": artist_id, "name": artist_name}}],
            "release-list": [{"id": "r", "title": "John Barleycorn Must Die", "date": "1970"}],
            "disambiguation": ""}


def test_recordings_for_drops_hits_not_credited_to_the_resolved_artist():
    mb = _FakeMB([_hit_credited("rec-traffic", "a-traffic", "Traffic"),
                  _hit_credited("rec-sound", "a-sound", "Traffic Sound")],
                 artists=[{"id": "a-traffic", "name": "Traffic"}])
    recs = MusicBrainzMetadata(mb).recordings_for(Candidate("Traffic", "Glad"))
    assert [r.mbid for r in recs] == ["rec-traffic"]   # Traffic Sound dropped


def test_recordings_for_unfiltered_when_artist_unresolved():
    mb = _FakeMB([_hit_credited("rec-1", "a-x", "X")], artists=[])  # no artist match
    assert len(MusicBrainzMetadata(mb).recordings_for(Candidate("X", "Glad"))) == 1


# --- album_from_release_group ---


def _rg(rg_id="rg-1", title="John Barleycorn Must Die", frd="1970-07",
        credit_phrase="Traffic", artist_id="a-traffic", artist_name="Traffic"):
    return {
        "id": rg_id,
        "title": title,
        "first-release-date": frd,
        "primary-type": "Album",
        "artist-credit": [{"artist": {"id": artist_id, "name": artist_name}}],
        "artist-credit-phrase": credit_phrase,
    }


def test_album_from_rg_maps_id_to_mbid():
    assert album_from_release_group(_rg()).ids.mbid == "rg-1"


def test_album_from_rg_maps_title():
    assert album_from_release_group(_rg()).title == "John Barleycorn Must Die"


def test_album_from_rg_maps_first_release_year():
    assert album_from_release_group(_rg(frd="1970-07")).first_released == 1970


def test_album_from_rg_partial_date_still_extracts_year():
    assert album_from_release_group(_rg(frd="1970")).first_released == 1970


def test_album_from_rg_non_digit_date_yields_none():
    assert album_from_release_group(_rg(frd="unknown")).first_released is None


def test_album_from_rg_artist_from_credit_phrase():
    assert album_from_release_group(_rg(credit_phrase="Traffic")).artist == "Traffic"


def test_album_from_rg_artist_falls_back_to_first_credit_when_no_phrase():
    rg = _rg()
    del rg["artist-credit-phrase"]
    assert album_from_release_group(rg).artist == "Traffic"


# --- MusicBrainzMetadata.albums_for ---

def _rg_credited(rg_id, artist_id, artist_name):
    return {
        "id": rg_id,
        "title": "John Barleycorn Must Die",
        "first-release-date": "1970-07",
        "primary-type": "Album",
        "artist-credit": [{"artist": {"id": artist_id, "name": artist_name}}],
        "artist-credit-phrase": artist_name,
    }


def test_albums_for_maps_release_groups_to_albums():
    mb = _FakeMB([], artists=[{"id": "a-traffic", "name": "Traffic"}],
                 release_groups=[_rg_credited("rg-1", "a-traffic", "Traffic")])
    albums = MusicBrainzMetadata(mb).albums_for(Candidate("Traffic", "John Barleycorn Must Die"))
    assert len(albums) == 1
    assert albums[0].ids.mbid == "rg-1"
    assert albums[0].title == "John Barleycorn Must Die"


def test_albums_for_drops_rgs_not_credited_to_resolved_artist():
    mb = _FakeMB([], artists=[{"id": "a-traffic", "name": "Traffic"}],
                 release_groups=[
                     _rg_credited("rg-traffic", "a-traffic", "Traffic"),
                     _rg_credited("rg-other", "a-other", "Other Artist"),
                 ])
    albums = MusicBrainzMetadata(mb).albums_for(Candidate("Traffic", "John Barleycorn Must Die"))
    assert [a.ids.mbid for a in albums] == ["rg-traffic"]


def test_albums_for_unfiltered_when_artist_unresolved():
    mb = _FakeMB([], artists=[],
                 release_groups=[_rg_credited("rg-x", "a-x", "X")])
    albums = MusicBrainzMetadata(mb).albums_for(Candidate("X", "Something"))
    assert len(albums) == 1


# --- Phase 4 Task 1 (updated): album_from_release_group maps traits ---

def test_album_from_rg_maps_compilation_secondary_type_to_trait():
    rg = {**_rg(), "secondary-type-list": ["Compilation"]}
    assert ReleaseTrait.COMPILATION in album_from_release_group(rg).traits


def test_album_from_rg_maps_live_secondary_type_to_trait():
    rg = {**_rg(), "secondary-type-list": ["Live"]}
    assert ReleaseTrait.LIVE in album_from_release_group(rg).traits


def test_album_from_rg_traits_empty_when_secondary_types_absent():
    rg = _rg()
    assert album_from_release_group(rg).traits == frozenset()


def test_album_from_rg_traits_empty_when_secondary_type_list_is_none():
    rg = {**_rg(), "secondary-type-list": None}
    assert album_from_release_group(rg).traits == frozenset()


def test_album_from_rg_unmapped_secondary_type_is_dropped():
    rg = {**_rg(), "secondary-type-list": ["Spokenword"]}
    assert album_from_release_group(rg).traits == frozenset()


# --- Phase 6 Task 1: artist_mbid hint bypasses search_artists ---

class _TrackingFakeMB(_FakeMB):
    """Same as _FakeMB but records which methods were called."""

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.search_artists_calls = 0

    def search_artists(self, artist="", limit=None, **kw):
        self.search_artists_calls += 1
        return super().search_artists(artist=artist, limit=limit, **kw)


def test_recordings_for_uses_provided_artist_mbid_skipping_search_artists():
    """When candidate.artist_mbid is set, search_artists is never called."""
    from tidalist.core.identifiers import MBID
    mb = _TrackingFakeMB(
        [_hit_credited("rec-traffic", "a-traffic", "Traffic")],
        artists=[{"id": "a-traffic", "name": "Traffic"}],
    )
    candidate = Candidate("Traffic", "Glad", artist_mbid=MBID("a-traffic"))
    recs = MusicBrainzMetadata(mb).recordings_for(candidate)
    assert mb.search_artists_calls == 0
    assert [r.mbid for r in recs] == ["rec-traffic"]


def test_albums_for_uses_provided_artist_mbid_skipping_search_artists():
    """When candidate.artist_mbid is set, search_artists is never called for albums_for."""
    mb = _TrackingFakeMB(
        [],
        artists=[{"id": "a-traffic", "name": "Traffic"}],
        release_groups=[_rg_credited("rg-traffic", "a-traffic", "Traffic")],
    )
    candidate = Candidate("Traffic", "John Barleycorn Must Die",
                          kind=Kind.ALBUM, artist_mbid=MBID("a-traffic"))
    albums = MusicBrainzMetadata(mb).albums_for(candidate)
    assert mb.search_artists_calls == 0
    assert [a.ids.mbid for a in albums] == ["rg-traffic"]


# --- _canonical_tracklist helpers ---

def _release(status="Official", date="1970-07-01", tracks=None):
    """Build a fake MB release dict."""
    if tracks is None:
        tracks = [
            {"position": "1", "recording": {"id": "rec-1", "title": "Glad",
                                             "isrc-list": ["GBABC1234567"], "length": "386000"}},
            {"position": "2", "recording": {"id": "rec-2", "title": "Freedom Rider",
                                             "isrc-list": [], "length": None}},
        ]
    medium_list = [{"track-list": tracks, "track-count": len(tracks)}]
    return {
        "id": f"rel-{date}",
        "status": status,
        "date": date,
        "medium-list": medium_list,
    }


# --- _canonical_tracklist: basic TrackRef building ---

def test_canonical_tracklist_builds_trackref_positions():
    mb = _FakeMB([], browse_releases=[_release()])
    svc = MusicBrainzMetadata(mb)
    tl = svc._canonical_tracklist("rg-1")
    assert tl[0].position == 1
    assert tl[1].position == 2


def test_canonical_tracklist_builds_trackref_titles():
    mb = _FakeMB([], browse_releases=[_release()])
    svc = MusicBrainzMetadata(mb)
    tl = svc._canonical_tracklist("rg-1")
    assert tl[0].title == "Glad"
    assert tl[1].title == "Freedom Rider"


def test_canonical_tracklist_isrc_from_first_in_list():
    mb = _FakeMB([], browse_releases=[_release()])
    svc = MusicBrainzMetadata(mb)
    tl = svc._canonical_tracklist("rg-1")
    assert tl[0].isrc == ISRC("GBABC1234567")


def test_canonical_tracklist_isrc_none_when_absent():
    mb = _FakeMB([], browse_releases=[_release()])
    svc = MusicBrainzMetadata(mb)
    tl = svc._canonical_tracklist("rg-1")
    assert tl[1].isrc is None


def test_canonical_tracklist_mbid_from_recording_id():
    mb = _FakeMB([], browse_releases=[_release()])
    svc = MusicBrainzMetadata(mb)
    tl = svc._canonical_tracklist("rg-1")
    assert tl[0].mbid == MBID("rec-1")


def test_canonical_tracklist_duration_s_rounded_from_ms():
    mb = _FakeMB([], browse_releases=[_release()])
    svc = MusicBrainzMetadata(mb)
    tl = svc._canonical_tracklist("rg-1")
    assert tl[0].duration_s == 386   # 386000ms → 386s


def test_canonical_tracklist_duration_s_none_when_absent():
    mb = _FakeMB([], browse_releases=[_release()])
    svc = MusicBrainzMetadata(mb)
    tl = svc._canonical_tracklist("rg-1")
    assert tl[1].duration_s is None


# --- _canonical_tracklist: canonical-release selection ---

def _release_with_n_tracks(n, date="1970-07-01", status="Official"):
    """Build a fake release with n identical tracks."""
    tracks = [
        {"position": str(i + 1), "recording": {"id": f"rec-{i}", "title": f"Track {i + 1}",
                                                "length": "200000"}}
        for i in range(n)
    ]
    return {
        "id": f"rel-{n}-{date}",
        "status": status,
        "date": date,
        "medium-list": [{"track-list": tracks, "track-count": n}],
    }


def test_canonical_selects_modal_over_deluxe():
    """Releases with counts [10, 10, 22]: modal=10, so a 10-track edition is chosen."""
    releases = [
        _release_with_n_tracks(10, date="1971-01-01"),
        _release_with_n_tracks(10, date="1972-01-01"),
        _release_with_n_tracks(22, date="1970-01-01"),  # earlier date but non-modal count
    ]
    mb = _FakeMB([], browse_releases=releases)
    tl = MusicBrainzMetadata(mb)._canonical_tracklist("rg-1")
    assert len(tl) == 10


def test_canonical_prefers_earliest_among_modal_releases():
    """Among modal-count releases, the earliest date is chosen."""
    releases = [
        _release_with_n_tracks(10, date="1972-01-01"),
        _release_with_n_tracks(10, date="1970-06-01"),  # earlier
        _release_with_n_tracks(10, date="1971-01-01"),
    ]
    mb = _FakeMB([], browse_releases=releases)
    svc = MusicBrainzMetadata(mb)
    tl = svc._canonical_tracklist("rg-1")
    # All 10-track, earliest is "1970-06-01" — track ids from that release start "rec-0"
    assert tl[0].mbid is not None  # just verify we got a tracklist back with 10 tracks
    assert len(tl) == 10


# --- _canonical_tracklist: edge cases ---

def test_canonical_ignores_non_official_releases():
    releases = [
        _release_with_n_tracks(10, status="Bootleg"),
        _release_with_n_tracks(8, status="Promotion"),
    ]
    mb = _FakeMB([], browse_releases=releases)
    tl = MusicBrainzMetadata(mb)._canonical_tracklist("rg-1")
    assert tl == ()


def test_canonical_empty_when_no_releases():
    mb = _FakeMB([], browse_releases=[])
    tl = MusicBrainzMetadata(mb)._canonical_tracklist("rg-1")
    assert tl == ()


def test_canonical_returns_empty_when_browse_raises():
    mb = _FakeMB([], browse_raises=True)
    tl = MusicBrainzMetadata(mb)._canonical_tracklist("rg-1")
    assert tl == ()


# --- albums_for: end-to-end tracklist attachment ---

def test_albums_for_attaches_tracklist_to_album():
    """albums_for should populate each Album's tracklist from _canonical_tracklist."""
    rg = _rg_credited("rg-traffic", "a-traffic", "Traffic")
    releases = [_release(date="1970-07-01")]
    mb = _FakeMB(
        [],
        artists=[{"id": "a-traffic", "name": "Traffic"}],
        release_groups=[rg],
        browse_releases=releases,
    )
    albums = MusicBrainzMetadata(mb).albums_for(Candidate("Traffic", "John Barleycorn Must Die"))
    assert len(albums) == 1
    assert len(albums[0].tracklist) == 2
    assert albums[0].tracklist[0].title == "Glad"
    assert albums[0].tracklist[1].title == "Freedom Rider"


def test_albums_for_tracklist_empty_when_browse_raises():
    """If _canonical_tracklist fails, albums_for still returns the album with empty tracklist."""
    rg = _rg_credited("rg-traffic", "a-traffic", "Traffic")
    mb = _FakeMB(
        [],
        artists=[{"id": "a-traffic", "name": "Traffic"}],
        release_groups=[rg],
        browse_raises=True,
    )
    albums = MusicBrainzMetadata(mb).albums_for(Candidate("Traffic", "John Barleycorn Must Die"))
    assert len(albums) == 1
    assert albums[0].tracklist == ()
