from dataclasses import replace

from tidalist.core.identifiers import ISRC, TrackId
from tidalist.core.recording import Recording, Credit
from tidalist.core.catalog import Track, PlatformAlbum
from tidalist.core.album import Album, TrackRef
from tidalist.core.edition import EditionPreference, EditionPolicy
from tidalist.core.render import MatchQuality, PlatformItem
from tidalist.render.tidal import TidalRenderer, _fold, _name_in_catalog, _title_match_album
from tests.fakes import FakePlatform


def _rec(title="Glad", artist="Traffic", isrc=None, album=None, duration_s=None,
         performer="Steve Winwood"):
    return Recording(artist=artist, title=title, isrc=isrc, album=album,
                     duration_s=duration_s, credits=(Credit(performer, "performer"),))


def _track(id, title="Glad", artists=("Traffic",), isrc=None, album=None, duration_s=None):
    return Track(id=TrackId(id), title=title, artists=artists, isrc=isrc, album=album,
                 duration_s=duration_s)


def test_resolve_by_isrc_takes_precedence_with_isrc_quality():
    target = _track("T-isrc", isrc=ISRC("GB1"))
    cat = FakePlatform([target, _track("T-decoy")])
    item, _ = TidalRenderer(cat).resolve(_rec(isrc=ISRC("GB1")))
    assert item.ref == "T-isrc" and item.quality is MatchQuality.ISRC


def test_resolve_falls_back_to_closest_search_hit():
    right = _track("T-right", title="Glad", artists=("Traffic",))
    looser = _track("T-loose", title="Glad Rag Doll", artists=("Traffic",))
    cat = FakePlatform([looser, right])
    item, _ = TidalRenderer(cat).resolve(_rec())
    assert item.ref == "T-right" and item.quality is MatchQuality.STRONG


def test_resolve_returns_none_when_search_finds_nothing():
    item, comps = TidalRenderer(FakePlatform([])).resolve(_rec())
    assert item is None and comps == ()


def test_resolve_prefers_closer_duration_among_equal_hits():
    a = _track("T-a", title="Glad", artists=("Traffic",), duration_s=200)
    b = _track("T-b", title="Glad", artists=("Traffic",), duration_s=386)
    cat = FakePlatform([a, b])
    item, _ = TidalRenderer(cat).resolve(_rec(duration_s=386))
    assert item.ref == "T-b"


def test_resolve_marks_a_title_mismatch_weak():
    only = _track("T-x", title="Glad Rag Doll", artists=("Traffic",))
    item, _ = TidalRenderer(FakePlatform([only])).resolve(_rec())
    assert item.ref == "T-x" and item.quality is MatchQuality.WEAK


def test_resolve_substitutes_a_live_take_and_reports_the_compromise():
    from tidalist.core.recording import Performance
    rec = Recording(artist="Traffic", title="Dear Mr. Fantasy",
                    performance=Performance.STUDIO,
                    credits=(Credit("Traffic", "performer"),))
    live = _track("T-live", title="Dear Mr. Fantasy (Live)", artists=("Traffic",))
    cat = FakePlatform([live])
    item, comps = TidalRenderer(cat).resolve(rec)
    assert item.ref == "T-live"
    assert len(comps) == 1
    assert comps[0].facet == "performance"
    assert comps[0].note == "studio take unavailable; used a live version"


def test_resolve_prefers_right_song_live_over_wrong_song_studio():
    from tidalist.core.recording import Performance
    rec = Recording(artist="Traffic", title="Glad", performance=Performance.STUDIO,
                    credits=(Credit("Traffic", "performer"),), duration_s=200)
    live = _track("T-live", title="Glad (Live)", artists=("Traffic",), duration_s=200)
    wrong = _track("T-wrong", title="Glad Rag Doll", artists=("Traffic",), duration_s=200)
    cat = FakePlatform([wrong, live])
    item, comps = TidalRenderer(cat).resolve(rec)
    assert item.ref == "T-live"                       # right song wins over wrong-song studio
    assert any(c.facet == "performance" for c in comps)   # and the live substitution is reported


def test_resolve_studio_match_reports_no_compromise():
    from tidalist.core.recording import Performance
    rec = Recording(artist="Traffic", title="Glad", performance=Performance.STUDIO,
                    credits=(Credit("Traffic", "performer"),), duration_s=200)
    studio = _track("T-studio", title="Glad", artists=("Traffic",), duration_s=200)
    item, comps = TidalRenderer(FakePlatform([studio])).resolve(rec)
    assert item.ref == "T-studio"
    assert comps == ()    # studio hit, performance unobserved -> no spurious compromise


def test_resolve_prefers_hi_res_among_tied_hits():
    # Two hits, same song/artist/duration (tied identity); pick the hi-res one.
    # "T-aaa" sorts before "T-hires", so ref-only tiebreak would pick the lossy one.
    lossy = _track("T-aaa", title="Glad", artists=("Traffic",), duration_s=200)
    hires = _track("T-hires", title="Glad", artists=("Traffic",), duration_s=200)
    lossy = replace(lossy, audio_quality="LOW")
    hires = replace(hires, audio_quality="HI_RES_LOSSLESS")
    cat = FakePlatform([lossy, hires])
    item, _ = TidalRenderer(cat).resolve(_rec(duration_s=200))
    assert item.ref == "T-hires"     # quality beats the lexicographically-smaller "T-aaa"


def test_emit_creates_a_playlist_and_adds_the_item_refs():
    cat = FakePlatform([])
    items = [PlatformItem(ref="T1", title="Glad", artists=("Traffic",)),
             PlatformItem(ref="T2", title="Dear Mr Fantasy", artists=("Traffic",))]
    ref = TidalRenderer(cat).emit("Winwood", items)
    assert cat.playlists[ref] == ["T1", "T2"]


# --- resolve_album tests ---

def _album(id="A1", title="John Barleycorn Must Die", artists=("Traffic",), year=1970):
    return PlatformAlbum(id=TrackId(id), title=title, artists=artists, year=year)


def _album_track(id, title, artists=("Traffic",)):
    return Track(id=TrackId(id), title=title, artists=artists)


def _domain_album(artist="Traffic", title="John Barleycorn Must Die"):
    return Album(artist=artist, title=title)


def test_resolve_album_drops_wrong_artist():
    wrong_artist = _album(id="A-wrong", title="John Barleycorn Must Die",
                          artists=("Some Other Band",))
    cat = FakePlatform(
        [],
        albums=[wrong_artist],
        album_track_map={"A-wrong": [_album_track("T1", "Glad")]},
    )
    items, compromise, gap_reason = TidalRenderer(cat).resolve_album(
        _domain_album(), EditionPolicy.default()
    )
    assert items == []
    assert compromise == ()
    assert gap_reason is not None


def test_resolve_album_picks_original_over_deluxe():
    original = _album(id="A-orig", title="John Barleycorn Must Die", year=1970)
    deluxe = _album(id="A-deluxe", title="John Barleycorn Must Die (Deluxe Edition)", year=2004)
    tracks = [
        _album_track("T1", "Glad"),
        _album_track("T2", "Freedom Rider"),
    ]
    cat = FakePlatform(
        [],
        albums=[deluxe, original],
        album_track_map={"A-orig": tracks, "A-deluxe": tracks[:1]},
    )
    items, compromise, gap_reason = TidalRenderer(cat).resolve_album(
        _domain_album(), EditionPolicy.default()
    )
    assert [i.ref for i in items] == ["T1", "T2"]
    assert all(i.quality is MatchQuality.STRONG for i in items)
    assert gap_reason is None


def test_resolve_album_returns_tracks_in_order():
    album = _album(id="A1")
    ordered_tracks = [
        _album_track("T1", "Glad"),
        _album_track("T2", "Freedom Rider"),
        _album_track("T3", "Empty Pages"),
    ]
    cat = FakePlatform(
        [],
        albums=[album],
        album_track_map={"A1": ordered_tracks},
    )
    items, _, _ = TidalRenderer(cat).resolve_album(_domain_album(), EditionPolicy.default())
    assert [i.ref for i in items] == ["T1", "T2", "T3"]


def test_resolve_album_returns_empty_when_nothing_matches():
    cat = FakePlatform([], albums=[], album_track_map={})
    items, compromise, gap_reason = TidalRenderer(cat).resolve_album(
        _domain_album(), EditionPolicy.default()
    )
    assert items == []
    assert compromise == ()
    assert gap_reason is not None


# --- Edition-distance / discography-enumeration tests ---

def _track_ref(position, title, isrc=None):
    return TrackRef(position=position, title=title, isrc=isrc)


def _golden_album(artist="Traffic", title="Mr. Fantasy", first_released=1967,
                  tracklist=()):
    return Album(artist=artist, title=title, first_released=first_released,
                 tracklist=tracklist)


def test_resolve_album_prefers_edition_nearest_golden_tracklist():
    """The Mr. Fantasy scenario: search returns one anchor edition; album_editions yields
    a 10-track and a 22-track; with a 10-track golden tracklist the 10-track edition
    must win (lower track-count and missing-track penalty).
    """
    # Build a golden with 10 canonical tracks.
    golden_tracks = tuple(
        _track_ref(i, f"Track {i}") for i in range(1, 11)
    )
    golden = _golden_album(tracklist=golden_tracks)

    anchor_id = "A-anchor"
    edition_10_id = "A-10track"
    edition_22_id = "A-22track"

    anchor = _album(id=anchor_id, title="Mr. Fantasy", artists=("Traffic",), year=1967)
    ed10 = _album(id=edition_10_id, title="Mr. Fantasy", artists=("Traffic",), year=1967)
    ed22 = _album(id=edition_22_id, title="Mr. Fantasy (Expanded)", artists=("Traffic",), year=2001)

    tracks_10 = [_album_track(f"T{i}", f"Track {i}") for i in range(1, 11)]
    tracks_22 = [_album_track(f"E{i}", f"Track {i}") for i in range(1, 23)]

    cat = FakePlatform(
        [],
        albums=[anchor],
        album_track_map={
            edition_10_id: tracks_10,
            edition_22_id: tracks_22,
        },
        album_editions_map={
            anchor_id: [ed10, ed22],
        },
    )
    items, compromise, gap_reason = TidalRenderer(cat).resolve_album(golden, EditionPolicy.default())
    assert [i.ref for i in items] == [f"T{n}" for n in range(1, 11)]
    assert all(i.quality is MatchQuality.STRONG for i in items)
    assert gap_reason is None


def test_resolve_album_multi_query_finds_via_the_stripped_artist():
    """When the verbatim artist+title search yields nothing, the The-stripped query
    should find the anchor and still resolve.
    """
    anchor = _album(id="A1", title="John Barleycorn Must Die",
                    artists=("Traffic",), year=1970)
    tracks = [_album_track("T1", "Glad"), _album_track("T2", "Freedom Rider")]
    # Only the the-stripped query ("Traffic John Barleycorn Must Die") would match here
    # if the verbatim artist were "The Traffic". We simulate: only albums matching
    # "traffic john barleycorn" — "the traffic" stripped of "the " becomes "traffic".
    the_traffic_album = Album(artist="The Traffic", title="John Barleycorn Must Die")
    cat = FakePlatform(
        [],
        albums=[anchor],
        album_track_map={"A1": tracks},
    )
    items, _, _ = TidalRenderer(cat).resolve_album(the_traffic_album, EditionPolicy.default())
    assert [i.ref for i in items] == ["T1", "T2"]


def test_resolve_album_title_only_query_finds_anchor_when_artist_queries_fail():
    """When the artist+title queries find nothing, the title-only fallback (still
    artist-filtered) finds the anchor. The domain artist 'unknown traffic' has a token
    ('unknown') absent from the catalog, so the verbatim query fails; the title-only
    query matches, and the artist filter passes since 'traffic' ⊆ 'unknown traffic'.
    """
    anchor = _album(id="A1", title="John Barleycorn Must Die",
                    artists=("Traffic",), year=1970)
    cat = FakePlatform([], albums=[anchor],
                      album_track_map={"A1": [_album_track("T1", "Glad")]})
    album = Album(artist="unknown traffic", title="John Barleycorn Must Die")
    items, _, _ = TidalRenderer(cat).resolve_album(album, EditionPolicy.default())
    assert [i.ref for i in items] == ["T1"]


def test_resolve_album_editions_empty_falls_back_to_survivors():
    """When album_editions returns empty, resolve_album falls back to search survivors
    (same as old behaviour) — existing edge case must remain green.
    """
    original = _album(id="A-orig", title="John Barleycorn Must Die", year=1970)
    tracks = [_album_track("T1", "Glad")]
    # album_editions_map is empty → falls back to survivors
    cat = FakePlatform(
        [],
        albums=[original],
        album_track_map={"A-orig": tracks},
    )
    items, _, _ = TidalRenderer(cat).resolve_album(_domain_album(), EditionPolicy.default())
    assert [i.ref for i in items] == ["T1"]


# --- Track-level assembly fallback tests ---

def test_resolve_album_assembles_from_tracks_when_album_absent():
    golden = Album(artist="Captain Beefheart", title="Trout Mask Replica",
                   tracklist=(_track_ref(1, "Frownland"),
                              _track_ref(2, "The Dust Blows Forward"),
                              _track_ref(3, "Dachau Blues")))
    # No album matches the search; tracks for positions 1 and 3 exist individually.
    t1 = _album_track("T1", "Frownland", artists=("Captain Beefheart",))
    t3 = _album_track("T3", "Dachau Blues", artists=("Captain Beefheart",))
    cat = FakePlatform([t1, t3], albums=[])      # search_albums empty -> assembly path
    items, comps, gap_reason = TidalRenderer(cat).resolve_album(golden, EditionPolicy.default())
    assert [i.ref for i in items] == ["T1", "T3"]
    assert len(comps) == 1 and comps[0].facet == "album-source"
    assert "2/3" in comps[0].note
    assert "missing positions: 2" in comps[0].note   # missing position 2 reported
    assert gap_reason is None   # partial assembly is not a gap


def test_resolve_album_gaps_when_no_tracks_assemble():
    golden = Album(artist="X", title="Absent Album", tracklist=(_track_ref(1, "Nope"),))
    cat = FakePlatform([], albums=[])
    items, comps, gap_reason = TidalRenderer(cat).resolve_album(golden, EditionPolicy.default())
    assert items == [] and comps == ()
    assert gap_reason == "track-fallback: 0/1 tracks found"


def test_resolve_album_no_tracklist_gaps():
    golden = Album(artist="X", title="Absent Album")   # no tracklist -> cannot assemble
    cat = FakePlatform([], albums=[])
    items, comps, gap_reason = TidalRenderer(cat).resolve_album(golden, EditionPolicy.default())
    assert items == [] and comps == ()
    assert gap_reason == "no-edition-matched: tried 2 anchor queries, 0 survivors"


# --- Task 3: credit-anchored search + any-credit survivor match + gap reasons ---

class _QuerySpyPlatform(FakePlatform):
    """A FakePlatform that records every query passed to search_albums, in call order."""

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.album_queries: list[str] = []

    def search_albums(self, query, limit=25):
        self.album_queries.append(query)
        return super().search_albums(query, limit)


def test_anchor_queries_prefers_conductor_credit_first():
    """A conductor credit anchors the very first search query, ahead of the compound
    artist+title fallback."""
    album = Album(artist="Wiener Philharmoniker", title="Symphony No. 5",
                  credits=(Credit("Carlos Kleiber", "conductor"),))
    cat = _QuerySpyPlatform([], albums=[])
    TidalRenderer(cat).resolve_album(album, EditionPolicy.default())
    assert cat.album_queries[0] == "Carlos Kleiber Symphony No. 5"


def test_resolve_album_survivor_matches_via_credit_name():
    """A candidate credited only to the orchestra (not the compound album.artist)
    still survives the filter because it substring-matches a credit name."""
    candidate = _album(id="A1", title="Symphony No. 5", artists=("Wiener Philharmoniker",))
    album = Album(artist="Carlos Kleiber", title="Symphony No. 5",
                  credits=(Credit("Wiener Philharmoniker", "orchestra"),))
    cat = FakePlatform([], albums=[candidate],
                       album_track_map={"A1": [_album_track("T1", "Allegro con brio")]})
    items, _, gap_reason = TidalRenderer(cat).resolve_album(album, EditionPolicy.default())
    assert [i.ref for i in items] == ["T1"]
    assert gap_reason is None


def test_resolve_album_gap_reason_edition_scoring_none_candidate():
    """When survivors exist but choose() picks nothing, the gap reason is
    'edition-scoring: no candidate chosen'."""
    from unittest.mock import patch

    anchor = _album(id="A1", title="John Barleycorn Must Die", artists=("Traffic",))
    cat = FakePlatform([], albums=[anchor],
                       album_track_map={"A1": [_album_track("T1", "Glad")]})
    with patch("tidalist.render.tidal.choose", return_value=(None, ())):
        items, comps, gap_reason = TidalRenderer(cat).resolve_album(
            _domain_album(), EditionPolicy.default()
        )
    assert items == [] and comps == ()
    assert gap_reason == "edition-scoring: no candidate chosen"


# --- Unicode/punctuation folding tests ---

def test_fold_strips_diacritics_and_casefolds():
    assert _fold("Brüggen") == "bruggen"


def test_name_in_catalog_folds_u2010_hyphen():
    # Golden credit "Yo‐Yo Ma" uses U+2010 HYPHEN; Tidal catalog uses ASCII hyphen.
    assert _name_in_catalog("Yo‐Yo Ma", ("Yo-Yo Ma",)) is True


def test_title_match_folds_curly_apostrophe():
    # Golden side carries U+2019 RIGHT SINGLE QUOTATION MARK; catalog side ASCII '.
    assert _title_match_album("Time’s Encomium", "Time's Encomium") is True


def test_title_match_folds_curly_double_quotes():
    # Golden side carries U+201C/U+201D curly double quotes; catalog side ASCII ".
    assert _title_match_album('Songs “Live” at Home', 'Songs "Live" at Home') is True


def test_title_match_folds_diacritics_casefold():
    # Diacritics should be stripped and casefolded
    assert _title_match_album('Le Divin Poème', "le divin poeme") is True


# --- Task A2-Py: credit-variant expansion ---

def test_anchor_query_expands_per_credit_variant():
    from tidalist.render.tidal import _anchor_queries
    album = Album(artist="Кирилл Петренко", title="Symphony No. 7",
                  credits=(Credit("Кирилл Петренко", "conductor", ("Kirill Petrenko",)),))
    queries = list(_anchor_queries(album))
    assert "Kirill Petrenko Symphony No. 7" in queries


def test_survivor_matches_via_credit_variant():
    from tidalist.render.tidal import _artist_match_album
    album = Album(artist="Кирилл Петренко", title="Symphony No. 7",
                  credits=(Credit("Кирилл Петренко", "conductor", ("Kirill Petrenko",)),))
    assert _artist_match_album(album, ("Kirill Petrenko",)) is True


# --- Task A3: compound-title segmentation, article strip, combined anchors ---

def test_title_match_accepts_slash_vs_semicolon_segments():
    # Index 3: golden "/" vs Tidal ";".
    assert _title_match_album(
        "Allegri: Miserere / Palestrina: Missa Papae Marcelli",
        "Allegri: Miserere; Palestrina: Missa Papae Marcelli") is True


def test_title_match_accepts_slash_vs_ampersand_segment():
    # Index 142: "/" vs "&", with a composer prefix on the catalog side.
    assert _title_match_album(
        "Symphony no. 5 / Cello Concerto",
        "Shostakovich: Symphony No. 5 & Cello Concerto No. 1") is True


def test_title_match_accepts_leading_article_drop():
    # Index 203: golden "A Polish Requiem" vs Tidal "Penderecki, K.: Polish Requiem".
    assert _title_match_album("A Polish Requiem", "Penderecki, K.: Polish Requiem") is True


def test_anchor_queries_combine_performer_and_composer_for_generic_title():
    # Index 76: a bare title needs conductor + composer combined to disambiguate.
    from tidalist.render.tidal import _anchor_queries
    album = Album(artist="Wiener Philharmoniker", title="Symphonie No. 9",
                  credits=(Credit("Carlo Maria Giulini", "conductor"),
                           Credit("Anton Bruckner", "composer")))
    queries = list(_anchor_queries(album))
    assert "Carlo Maria Giulini Anton Bruckner Symphonie No. 9" in queries


def test_anchor_queries_emit_short_title_for_verbose_title():
    # Index 100: a shorter, less-diluting query leads with the title's head segment.
    from tidalist.render.tidal import _anchor_queries
    album = Album(artist="Mikhail Pletnev",
                  title="Symphony no. 3 / Poème de l'Extase",
                  credits=(Credit("Mikhail Pletnev", "conductor"),))
    queries = list(_anchor_queries(album))
    assert "Mikhail Pletnev Symphony no. 3" in queries


def test_anchor_queries_bounded_and_title_fallback_survives():
    from tidalist.render.tidal import _anchor_queries, _MAX_CREDIT_QUERIES
    creds = tuple(Credit(f"Conductor {i}", "conductor") for i in range(12))
    album = Album(artist="Orchestra X", title="Requiem", credits=creds)
    queries = list(_anchor_queries(album))
    # Credit-derived queries are capped; the title-only fallback still appears.
    assert "Requiem" in queries
    assert sum(1 for q in queries if q.endswith(" Requiem") and q != "Orchestra X Requiem") <= _MAX_CREDIT_QUERIES
