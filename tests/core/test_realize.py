import pytest

from tidalist.core.recording import Recording
from tidalist.core.album import Album, TrackRef
from tidalist.core.catalog import Track
from tidalist.core.criteria import Verdict
from tidalist.core.provenance import Provenance
from tidalist.core.brief import Brief
from tidalist.core.golden import GoldenEntry, GoldenPlaylist
from tidalist.core.realize import realize, publish, Realization, PlatformItem, MatchQuality, EditionOption, choose_edition, edition_distance
from tidalist.core.edition import EditionPreference, EditionPolicy
from tidalist.core.errors import PlatformError


class _FakeRealizer:
    """Resolves by recording title; a missing title is a gap. Records emit calls.

    Also supports resolve_album: keyed by album title, seeded as
    (items_list, tuple[Compromise, ...]); returns the AlbumResolution 3-tuple
    (items, compromises, gap_reason=None).
    """

    def __init__(self, items: dict, albums: dict | None = None):
        self._by_title = {k.casefold(): v for k, v in items.items()}
        # albums: {title: ([PlatformItem, ...], tuple[Compromise, ...])}
        self._albums = {k.casefold(): v for k, v in (albums or {}).items()}
        self.emitted = []

    def resolve(self, recording):
        return self._by_title.get(recording.title.casefold()), ()

    def resolve_album(self, album, preference):
        key = album.title.casefold()
        if key in self._albums:
            items, comps = self._albums[key]
            return items, comps, None
        return [], (), None

    def emit(self, name, items):
        ref = f"playlist-{len(self.emitted) + 1}"
        self.emitted.append((name, [i.ref for i in items], ref))
        return ref


def _entry(title, admitted=True):
    rec = Recording(artist="Traffic", title=title)
    verdict = Verdict.ok() if admitted else Verdict.rejected("cover")
    return GoldenEntry(rec, Provenance("nl"), verdict)


def _golden(*entries, name="Winwood"):
    return GoldenPlaylist(name, Brief(name, ()), tuple(entries))


def _item(ref="t1", title="Glad"):
    return PlatformItem(ref=ref, title=title, artists=("Traffic",), quality=MatchQuality.ISRC)


def test_realize_resolves_each_admitted_entry():
    r = realize(_golden(_entry("Glad")), _FakeRealizer({"Glad": _item("T-glad")}))
    assert isinstance(r, Realization)
    assert len(r.entries) == 1
    assert r.entries[0].items[0].ref == "T-glad"


def test_realize_skips_rejected_golden_entries():
    golden = _golden(_entry("Glad"), _entry("Cover", admitted=False))
    r = realize(golden, _FakeRealizer({"Glad": _item("T-glad")}))
    assert [e.golden.item.title for e in r.entries] == ["Glad"]


def test_realize_records_a_gap_when_unresolved():
    golden = _golden(_entry("Glad"), _entry("Obscure"))
    r = realize(golden, _FakeRealizer({"Glad": _item("T-glad")}))   # Obscure unresolved
    assert [g.item.title for g in r.gaps()] == ["Obscure"]
    assert [e.golden.item.title for e in r.resolved()] == ["Glad"]


def test_publish_emits_only_resolved_items_and_returns_the_reference():
    golden = _golden(_entry("Glad"), _entry("Obscure"))
    realizer = _FakeRealizer({"Glad": _item("T-glad")})
    ref = publish(realize(golden, realizer), realizer)
    name, refs, returned = realizer.emitted[-1]
    assert name == "Winwood" and refs == ["T-glad"]
    assert ref == returned


def test_publish_raises_when_nothing_resolved():
    realizer = _FakeRealizer({})
    r = realize(_golden(_entry("Obscure")), realizer)
    with pytest.raises(PlatformError):
        publish(r, realizer)


def test_album_entry_with_no_tracks_is_a_gap():
    g = _golden(GoldenEntry(Album(artist="Traffic", title="John Barleycorn Must Die"),
                            Provenance("nl"), Verdict.ok()))
    r = realize(g, _FakeRealizer({}))
    # resolve_album returns ([], ()) → gap
    assert [e.golden.item.title for e in r.entries if e.is_gap] == ["John Barleycorn Must Die"]


def test_album_entry_with_tracks_is_resolved():
    track1 = PlatformItem(ref="t1", title="Glad", artists=("Traffic",))
    track2 = PlatformItem(ref="t2", title="Freedom Rider", artists=("Traffic",))
    album = Album(artist="Traffic", title="John Barleycorn Must Die")
    entry = GoldenEntry(album, Provenance("nl"), Verdict.ok())
    g = _golden(entry)
    realizer = _FakeRealizer({}, albums={"John Barleycorn Must Die": ([track1, track2], ())})
    r = realize(g, realizer)
    assert len(r.resolved()) == 1
    assert r.resolved()[0].items == (track1, track2)
    assert not r.resolved()[0].is_gap


def test_album_entry_compromise_surfaces_in_compromises():
    from tidalist.core.fidelity import Compromise
    track1 = PlatformItem(ref="t1", title="Glad", artists=("Traffic",))
    album = Album(artist="Traffic", title="John Barleycorn Must Die")
    entry = GoldenEntry(album, Provenance("nl"), Verdict.ok())
    g = _golden(entry)
    comp = Compromise("edition", "steven wilson", "(no preferred edition)",
                      "preferred edition (steven wilson) unavailable")
    realizer = _FakeRealizer({}, albums={"John Barleycorn Must Die": ([track1], (comp,))})
    r = realize(g, realizer)
    comps = r.compromises()
    assert len(comps) == 1
    golden_e, got = comps[0]
    assert golden_e.item.title == "John Barleycorn Must Die"
    assert got.facet == "edition" and "steven wilson" in got.note


def test_compromises_empty_when_no_compromise():
    track1 = PlatformItem(ref="t1", title="Glad", artists=("Traffic",))
    album = Album(artist="Traffic", title="John Barleycorn Must Die")
    entry = GoldenEntry(album, Provenance("nl"), Verdict.ok())
    g = _golden(entry)
    realizer = _FakeRealizer({}, albums={"John Barleycorn Must Die": ([track1], ())})
    r = realize(g, realizer)
    assert r.compromises() == ()


def test_publish_flattens_album_tracks():
    track1 = PlatformItem(ref="t1", title="Glad", artists=("Traffic",))
    track2 = PlatformItem(ref="t2", title="Freedom Rider", artists=("Traffic",))
    album = Album(artist="Traffic", title="John Barleycorn Must Die")
    entry = GoldenEntry(album, Provenance("nl"), Verdict.ok())
    g = _golden(entry)
    realizer = _FakeRealizer({}, albums={"John Barleycorn Must Die": ([track1, track2], ())})
    r = realize(g, realizer)
    publish(r, realizer)
    name, refs, _ = realizer.emitted[-1]
    assert refs == ["t1", "t2"]


def test_publish_flattens_mixed_recording_and_album():
    track1 = PlatformItem(ref="t1", title="Glad", artists=("Traffic",))
    track2 = PlatformItem(ref="t2", title="Freedom Rider", artists=("Traffic",))
    album = Album(artist="Traffic", title="John Barleycorn Must Die")
    album_entry = GoldenEntry(album, Provenance("nl"), Verdict.ok())
    rec_entry = _entry("Glad")
    g = _golden(rec_entry, album_entry)
    realizer = _FakeRealizer(
        {"Glad": _item("T-glad")},
        albums={"John Barleycorn Must Die": ([track1, track2], ())},
    )
    r = realize(g, realizer)
    publish(r, realizer)
    name, refs, _ = realizer.emitted[-1]
    assert refs == ["T-glad", "t1", "t2"]


# --- choose_edition tests ---

def test_choose_edition_empty_options_returns_none_none():
    chosen, compromise = choose_edition([], EditionPreference(markers=("steven wilson",)))
    assert chosen is None
    assert compromise is None


def test_choose_edition_marker_match_wins_no_compromise():
    options = [
        EditionOption(ref="orig", title="Close to the Edge", year=1972),
        EditionOption(ref="sw", title="Close to the Edge (Steven Wilson Mix)", year=2013),
    ]
    pref = EditionPreference(markers=("steven wilson",))
    chosen, compromise = choose_edition(options, pref)
    assert chosen is not None
    assert chosen.ref == "sw"
    assert compromise is None


def test_choose_edition_no_marker_falls_back_to_original_with_compromise():
    options = [
        EditionOption(ref="orig", title="Close to the Edge", year=1972),
        EditionOption(ref="remaster", title="Close to the Edge (Remastered)", year=2003),
    ]
    pref = EditionPreference(markers=("steven wilson",), prefer_original=True)
    golden = Album(artist="Yes", title="Close to the Edge", first_released=1972)
    chosen, compromise = choose_edition(options, pref, golden=golden)
    assert chosen is not None
    assert chosen.ref == "orig"
    assert compromise == "preferred edition (steven wilson) unavailable"


def test_choose_edition_no_marker_no_compromise_when_markers_empty():
    options = [
        EditionOption(ref="orig", title="Close to the Edge", year=1972),
        EditionOption(ref="remaster", title="Close to the Edge (Remastered)", year=2003),
    ]
    pref = EditionPreference(markers=(), prefer_original=True)
    chosen, compromise = choose_edition(options, pref)
    assert chosen is not None
    assert chosen.ref == "orig"
    assert compromise is None


def test_choose_edition_marker_order_honored_first_wins():
    options = [
        EditionOption(ref="mojo", title="Quadrophenia (MoFi Edition)", year=2011),
        EditionOption(ref="sw", title="Quadrophenia (Steven Wilson Mix)", year=2021),
    ]
    # "mobile fidelity" appears in title "MoFi Edition"? No, let's use exact markers.
    # First marker is "mobile fidelity" — won't match "MoFi". Use "mofi" to match.
    pref = EditionPreference(markers=("mofi", "steven wilson"))
    chosen, compromise = choose_edition(options, pref)
    assert chosen is not None
    assert chosen.ref == "mojo"
    assert compromise is None


def test_choose_edition_original_fallback_prefers_non_reissue_on_year_tie():
    options = [
        EditionOption(ref="deluxe", title="Thick as a Brick (Deluxe Edition)", year=1972),
        EditionOption(ref="orig", title="Thick as a Brick", year=1972),
    ]
    pref = EditionPreference(markers=(), prefer_original=True)
    chosen, compromise = choose_edition(options, pref)
    assert chosen is not None
    assert chosen.ref == "orig"


def test_choose_edition_none_year_sorts_last():
    options = [
        EditionOption(ref="no_year", title="Aqualung", year=None),
        EditionOption(ref="orig", title="Aqualung", year=1971),
    ]
    pref = EditionPreference(markers=(), prefer_original=True)
    golden = Album(artist="Jethro Tull", title="Aqualung", first_released=1971)
    chosen, compromise = choose_edition(options, pref, golden=golden)
    assert chosen is not None
    assert chosen.ref == "orig"


def test_choose_edition_reports_compromise_when_markers_unmatched_even_without_prefer_original():
    # Markers are present but none match, and prefer_original is off. We still fall back
    # to an available edition, but the requested edition was unavailable — that is a
    # compromise and must be reported, never silently swallowed.
    options = [
        EditionOption(ref="a", title="Selling England by the Pound", year=1973),
        EditionOption(ref="b", title="Selling England by the Pound (Remaster)", year=2008),
    ]
    pref = EditionPreference(markers=("steven wilson",), prefer_original=False)
    chosen, compromise = choose_edition(options, pref)
    assert chosen is not None
    assert compromise == "preferred edition (steven wilson) unavailable"


# --- Phase 6 Task 1: realize() uses entry.edition over the global preference ---

class _PreferenceCapturingRealizer(_FakeRealizer):
    """Records the preference passed to resolve_album."""
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.resolve_album_preferences: list[EditionPreference] = []

    def resolve_album(self, album, preference):
        self.resolve_album_preferences.append(preference)
        return super().resolve_album(album, preference)


def test_realize_uses_entry_edition_over_global_preference():
    """realize() passes entry.edition (not the global preference) when set."""
    from tidalist.core.golden import GoldenEntry
    per_entry_pref = EditionPreference(markers=("steven wilson",), prefer_original=True)
    global_pref = EditionPreference(markers=("mobile fidelity",), prefer_original=False)
    album = Album(artist="Traffic", title="John Barleycorn Must Die")
    entry = GoldenEntry(album, Provenance("nl"), Verdict.ok(), edition=per_entry_pref)
    g = _golden(entry)
    realizer = _PreferenceCapturingRealizer(
        {}, albums={"John Barleycorn Must Die": ([PlatformItem("t1", "Glad", ())], ())}
    )
    realize(g, realizer, preference=global_pref)
    assert realizer.resolve_album_preferences == [per_entry_pref]


def test_realize_uses_global_preference_when_entry_edition_is_none():
    """realize() falls back to global preference when entry.edition is None."""
    from tidalist.core.golden import GoldenEntry
    global_pref = EditionPreference(markers=("mobile fidelity",), prefer_original=False)
    album = Album(artist="Traffic", title="John Barleycorn Must Die")
    entry = GoldenEntry(album, Provenance("nl"), Verdict.ok())   # edition=None
    g = _golden(entry)
    realizer = _PreferenceCapturingRealizer(
        {}, albums={"John Barleycorn Must Die": ([PlatformItem("t1", "Glad", ())], ())}
    )
    realize(g, realizer, preference=global_pref)
    assert realizer.resolve_album_preferences == [global_pref]


# ---------------------------------------------------------------------------
# edition_distance tests
# ---------------------------------------------------------------------------

def _make_track(title: str, isrc: str | None = None, track_id: str = "t1") -> Track:
    return Track(id=track_id, title=title, artists=("Artist",), isrc=isrc)


def _make_ref(title: str, isrc: str | None = None) -> TrackRef:
    return TrackRef(position=1, title=title, isrc=isrc)


def test_edition_distance_tracklist_count_diff_dominates():
    """A 22-track edition is farther than a 10-track edition when golden has 10 tracks."""
    golden_tracks = tuple(
        TrackRef(position=i + 1, title=f"Track {i + 1}") for i in range(10)
    )
    golden = Album(artist="Yes", title="Close to the Edge", first_released=1972, tracklist=golden_tracks)

    opt_exact = EditionOption(
        ref="exact",
        title="Close to the Edge",
        year=1972,
        tracks=tuple(_make_track(f"Track {i + 1}", track_id=f"t{i}") for i in range(10)),
    )
    opt_big = EditionOption(
        ref="big",
        title="Close to the Edge",
        year=1972,
        tracks=tuple(_make_track(f"Track {i + 1}", track_id=f"t{i}") for i in range(22)),
    )

    pref = EditionPreference(markers=(), prefer_original=False)
    d_exact = edition_distance(golden, opt_exact, pref)
    d_big = edition_distance(golden, opt_big, pref)
    assert d_exact < d_big


def test_edition_distance_isrc_match_counts_even_when_titles_differ():
    """An ISRC match counts a track as matched even if titles differ."""
    golden_tracks = (
        TrackRef(position=1, title="Original Title", isrc="GBУМ71234567"),
    )
    golden = Album(artist="Yes", title="Roundabout", first_released=1972, tracklist=golden_tracks)

    opt_with_isrc = EditionOption(
        ref="isrc_match",
        title="Roundabout",
        year=1972,
        tracks=(_make_track("Different Title", isrc="GBУМ71234567"),),
    )
    opt_no_isrc = EditionOption(
        ref="no_isrc",
        title="Roundabout",
        year=1972,
        tracks=(_make_track("Different Title"),),
    )

    pref = EditionPreference(markers=(), prefer_original=False)
    d_isrc = edition_distance(golden, opt_with_isrc, pref)
    d_no_isrc = edition_distance(golden, opt_no_isrc, pref)
    assert d_isrc < d_no_isrc


def test_edition_distance_title_fallback_matches():
    """Title fallback matches a track when ISRC is absent."""
    golden_tracks = (TrackRef(position=1, title="Roundabout"),)
    golden = Album(artist="Yes", title="Fragile", first_released=1971, tracklist=golden_tracks)

    opt_matched = EditionOption(
        ref="matched",
        title="Fragile",
        year=1971,
        tracks=(_make_track("Roundabout"),),
    )
    opt_unmatched = EditionOption(
        ref="unmatched",
        title="Fragile",
        year=1971,
        tracks=(_make_track("Heart of the Sunrise"),),
    )

    pref = EditionPreference(markers=(), prefer_original=False)
    d_matched = edition_distance(golden, opt_matched, pref)
    d_unmatched = edition_distance(golden, opt_unmatched, pref)
    assert d_matched < d_unmatched


def test_edition_distance_title_dim_exact_beats_deluxe():
    """An exact-title edition beats a 'Deluxe Edition' variant (same year, prefer_original)."""
    golden = Album(artist="Yes", title="Close to the Edge", first_released=1972)
    pref = EditionPreference(markers=(), prefer_original=True)

    opt_plain = EditionOption(ref="plain", title="Close to the Edge", year=1972)
    opt_deluxe = EditionOption(ref="deluxe", title="Close to the Edge (Deluxe Edition)", year=1972)

    d_plain = edition_distance(golden, opt_plain, pref)
    d_deluxe = edition_distance(golden, opt_deluxe, pref)
    assert d_plain < d_deluxe


def test_edition_distance_year_dim_nearest_wins():
    """With a golden first_released, the edition nearest that year wins."""
    golden = Album(artist="Yes", title="Close to the Edge", first_released=1972)
    pref = EditionPreference(markers=(), prefer_original=True)

    opt_near = EditionOption(ref="near", title="Close to the Edge", year=1973)
    opt_far = EditionOption(ref="far", title="Close to the Edge", year=2003)

    d_near = edition_distance(golden, opt_near, pref)
    d_far = edition_distance(golden, opt_far, pref)
    assert d_near < d_far


def test_edition_distance_reissue_dim_plain_beats_remastered():
    """A plain edition beats a Remastered one on a year tie (prefer_original)."""
    golden = Album(artist="Jethro Tull", title="Aqualung", first_released=1971)
    pref = EditionPreference(markers=(), prefer_original=True)

    opt_plain = EditionOption(ref="plain", title="Aqualung", year=1971)
    opt_remastered = EditionOption(ref="remastered", title="Aqualung (Remastered)", year=1971)

    d_plain = edition_distance(golden, opt_plain, pref)
    d_remastered = edition_distance(golden, opt_remastered, pref)
    assert d_plain < d_remastered


def test_edition_distance_marker_dominates_tracklist():
    """A marker-bearing edition wins even when it is far on tracklist."""
    golden_tracks = tuple(TrackRef(position=i + 1, title=f"T{i}") for i in range(10))
    golden = Album(artist="Yes", title="Fragile", first_released=1971, tracklist=golden_tracks)
    pref = EditionPreference(markers=("steven wilson",), prefer_original=False)

    # marker edition: has the marker but 22 tracks (far on tracklist)
    opt_marker = EditionOption(
        ref="marker",
        title="Fragile (Steven Wilson Mix)",
        year=2015,
        tracks=tuple(_make_track(f"T{i}", track_id=f"m{i}") for i in range(22)),
    )
    # no-marker edition: no marker but exact 10 tracks
    opt_exact = EditionOption(
        ref="exact",
        title="Fragile",
        year=1971,
        tracks=tuple(_make_track(f"T{i}", track_id=f"e{i}") for i in range(10)),
    )

    d_marker = edition_distance(golden, opt_marker, pref)
    d_exact = edition_distance(golden, opt_exact, pref)
    assert d_marker < d_exact


def test_edition_distance_marker_order_first_beats_second():
    """An edition matching the first preference marker beats one matching the second."""
    golden = Album(artist="Yes", title="Close to the Edge", first_released=1972)
    pref = EditionPreference(markers=("mofi", "steven wilson"), prefer_original=False)

    opt_first = EditionOption(ref="mofi", title="Close to the Edge (MoFi Edition)", year=2011)
    opt_second = EditionOption(ref="sw", title="Close to the Edge (Steven Wilson Mix)", year=2013)

    d_first = edition_distance(golden, opt_first, pref)
    d_second = edition_distance(golden, opt_second, pref)
    assert d_first < d_second


def test_edition_distance_same_marker_content_tiebreak():
    """Among two editions both carrying the top marker, the nearer on content wins."""
    golden_tracks = tuple(TrackRef(position=i + 1, title=f"T{i}") for i in range(10))
    golden = Album(artist="Yes", title="Fragile", first_released=1971, tracklist=golden_tracks)
    pref = EditionPreference(markers=("steven wilson",), prefer_original=False)

    opt_near = EditionOption(
        ref="near",
        title="Fragile (Steven Wilson Mix)",
        year=2015,
        tracks=tuple(_make_track(f"T{i}", track_id=f"n{i}") for i in range(10)),
    )
    opt_far = EditionOption(
        ref="far",
        title="Fragile (Steven Wilson Mix)",
        year=2015,
        tracks=tuple(_make_track(f"T{i}", track_id=f"f{i}") for i in range(22)),
    )

    d_near = edition_distance(golden, opt_near, pref)
    d_far = edition_distance(golden, opt_far, pref)
    assert d_near < d_far


def test_choose_edition_returns_min_distance():
    """choose_edition returns the option with the lowest edition_distance."""
    golden_tracks = tuple(TrackRef(position=i + 1, title=f"T{i}") for i in range(10))
    golden = Album(artist="Yes", title="Fragile", first_released=1971, tracklist=golden_tracks)
    pref = EditionPreference(markers=(), prefer_original=False)

    opt_exact = EditionOption(
        ref="exact",
        title="Fragile",
        year=1971,
        tracks=tuple(_make_track(f"T{i}", track_id=f"e{i}") for i in range(10)),
    )
    opt_far = EditionOption(
        ref="far",
        title="Fragile",
        year=1971,
        tracks=tuple(_make_track(f"T{i}", track_id=f"f{i}") for i in range(22)),
    )

    chosen, compromise = choose_edition([opt_far, opt_exact], pref, golden=golden)
    assert chosen is not None
    assert chosen.ref == "exact"
    assert compromise is None


def test_choose_edition_compromise_when_no_marker_matched():
    """Reports compromise when no edition carries any requested marker."""
    golden = Album(artist="Yes", title="Close to the Edge", first_released=1972)
    options = [
        EditionOption(ref="orig", title="Close to the Edge", year=1972),
        EditionOption(ref="remaster", title="Close to the Edge (Remastered)", year=2003),
    ]
    pref = EditionPreference(markers=("steven wilson",), prefer_original=True)
    chosen, compromise = choose_edition(options, pref, golden=golden)
    assert chosen is not None
    assert compromise == "preferred edition (steven wilson) unavailable"


def test_choose_edition_prefer_original_false_ignores_title_year_reissue():
    """With prefer_original=False, title/year/reissue dimensions contribute 0; only marker+tracklist matter."""
    # Two editions: "plain 1972" and "Deluxe 1999".
    # With prefer_original=True, the 1972 plain would win on year+reissue.
    # With prefer_original=False, they should tie on those dims → first in list (or tracklist) wins.
    golden = Album(artist="Yes", title="Fragile", first_released=1971)
    pref = EditionPreference(markers=(), prefer_original=False)

    opt_plain_old = EditionOption(ref="plain", title="Fragile", year=1971)
    opt_deluxe_new = EditionOption(ref="deluxe", title="Fragile (Deluxe Edition)", year=2004)

    d_plain = edition_distance(golden, opt_plain_old, pref)
    d_deluxe = edition_distance(golden, opt_deluxe_new, pref)
    # With prefer_original=False, both year and reissue dims are 0, so distances are equal
    assert d_plain == d_deluxe
