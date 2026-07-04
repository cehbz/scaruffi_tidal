from dataclasses import dataclass as _dc

import pytest

from tidalist.core.fidelity import (
    Compromise, PlatformCandidate, render_distance, choose,
    IdentityFacet, W_FUZZY_TITLE, W_FUZZY_ARTIST, W_FUZZY_DUR, W_PERFORMANCE,
    EditionFacet, edition_distance,
)
from tidalist.core.edition import EditionPreference
from tidalist.core.identifiers import ISRC
from tidalist.core.recording import Performance, Recording
from tidalist.core.album import Album


@_dc
class _FixedFacet:
    name = "fixed"
    weight = 1.0
    table: dict          # ref -> distance
    note: str | None = None
    def distance(self, golden, cand): return self.table[cand.ref]
    def compromise(self, golden, cand):
        return Compromise("fixed", "x", "y", self.note) if self.note else None


def test_render_distance_sums_weighted_facets():
    cand = PlatformCandidate(ref="A", title="t")
    g = Recording(artist="a", title="t")
    facets = [_FixedFacet(table={"A": 2.0}), _FixedFacet(table={"A": 3.0})]
    assert render_distance(g, cand, facets) == 5.0


def test_choose_returns_min_distance_candidate_and_its_compromises():
    g = Recording(artist="a", title="t")
    c_near = PlatformCandidate(ref="near", title="t")
    c_far = PlatformCandidate(ref="far", title="t")
    facets = [_FixedFacet(table={"near": 1.0, "far": 9.0}, note="used a substitute")]
    chosen, comps = choose(g, [c_far, c_near], facets)
    assert chosen.ref == "near"
    assert len(comps) == 1 and comps[0].note == "used a substitute"


def test_choose_breaks_ties_deterministically_by_ref():
    g = Recording(artist="a", title="t")
    a = PlatformCandidate(ref="a", title="t")
    b = PlatformCandidate(ref="b", title="t")
    facets = [_FixedFacet(table={"a": 5.0, "b": 5.0})]
    # Equal distance → lexicographically smaller ref wins, regardless of input order.
    assert choose(g, [b, a], facets)[0].ref == "a"
    assert choose(g, [a, b], facets)[0].ref == "a"


def test_choose_empty_candidates_returns_none_and_no_compromises():
    g = Recording(artist="a", title="t")
    assert choose(g, [], [_FixedFacet(table={})]) == (None, ())


def test_compromise_carries_facet_desired_used_note():
    c = Compromise(facet="edition", desired="steven wilson",
                   used="(no preferred edition)", note="preferred edition unavailable")
    assert c.facet == "edition"
    assert c.desired == "steven wilson"
    assert c.used == "(no preferred edition)"
    assert "unavailable" in c.note


def test_platform_candidate_defaults_are_observation_unknowns():
    c = PlatformCandidate(ref="A1", title="Mr. Fantasy")
    assert c.artists == () and c.isrc is None and c.year is None and c.tracks == ()
    assert c.release_class is None
    assert c.performance is Performance.UNKNOWN
    assert c.source_kind is None and c.audio_quality is None and c.popularity is None
    assert c.duration_s is None


def test_platform_candidate_carries_duration():
    c = PlatformCandidate(ref="t1", title="Glad", duration_s=386)
    assert c.duration_s == 386


def test_identity_recording_isrc_match_is_zero():
    g = Recording(artist="Traffic", title="Glad", isrc=ISRC("GB1"))
    cand = PlatformCandidate(ref="t1", title="Glad", isrc=ISRC("GB1"))
    assert IdentityFacet().distance(g, cand) == 0.0


def test_identity_recording_isrc_mismatch_falls_back_to_fuzzy():
    # Different ISRC but identical title/artist/duration -> fuzzy match -> 0 (no cliff).
    g = Recording(artist="Traffic", title="Glad", isrc=ISRC("GB1"), duration_s=200)
    cand = PlatformCandidate(ref="t2", title="Glad", isrc=ISRC("GB2"),
                             artists=("Traffic",), duration_s=200)
    assert IdentityFacet().distance(g, cand) == 0.0


def test_identity_recording_fuzzy_full_match_is_zero():
    g = Recording(artist="Traffic", title="Glad", duration_s=200)   # no ISRC
    cand = PlatformCandidate(ref="t3", title="Glad", artists=("Traffic",), duration_s=200)
    assert IdentityFacet().distance(g, cand) == 0.0


def test_identity_recording_title_partial_overlap_grades_title():
    # {glad} vs {glad,rag,doll}: Jaccard distance = 1 - 1/3 = 2/3 (graded, not the full penalty).
    g = Recording(artist="Traffic", title="Glad", duration_s=200)
    cand = PlatformCandidate(ref="x", title="Glad Rag Doll", artists=("Traffic",), duration_s=200)
    assert IdentityFacet().distance(g, cand) == pytest.approx(W_FUZZY_TITLE * 2 / 3)
    assert IdentityFacet().distance(g, cand) < W_FUZZY_TITLE   # graded, below the full penalty


def test_identity_recording_artist_mismatch_adds_fuzzy_artist():
    g = Recording(artist="Traffic", title="Glad", duration_s=200)
    cand = PlatformCandidate(ref="x", title="Glad", artists=("Other Band",), duration_s=200)
    assert IdentityFacet().distance(g, cand) == W_FUZZY_ARTIST


def test_identity_recording_duration_ratio_grades_duration():
    # exact title+artist; only the relative duration term: W_FUZZY_DUR * 6/386.
    g = Recording(artist="Traffic", title="Glad", duration_s=386)
    cand = PlatformCandidate(ref="x", title="Glad", artists=("Traffic",), duration_s=380)
    assert IdentityFacet().distance(g, cand) == pytest.approx(W_FUZZY_DUR * 6 / 386)


def test_identity_missing_candidate_duration_no_penalty():
    g = Recording(artist="Traffic", title="Glad", duration_s=200)
    cand = PlatformCandidate(ref="x", title="Glad", artists=("Traffic",))  # no duration observed
    assert IdentityFacet().distance(g, cand) == 0.0   # title 0, artist 0, duration skipped


def test_identity_duration_term_stays_below_performance_tier():
    # Even a huge duration gap is a bounded sub-performance tiebreak (ratio<1 * W_FUZZY_DUR < W_PERFORMANCE).
    g = Recording(artist="Traffic", title="Glad", duration_s=10)
    cand = PlatformCandidate(ref="x", title="Glad", artists=("Traffic",), duration_s=10000)
    assert IdentityFacet().distance(g, cand) < W_PERFORMANCE


def test_identity_album_is_zero():
    g = Album(artist="Traffic", title="Mr. Fantasy")
    cand = PlatformCandidate(ref="A1", title="Mr. Fantasy")
    assert IdentityFacet().distance(g, cand) == 0.0
    assert IdentityFacet().compromise(g, cand) is None


# ---------------------------------------------------------------------------
# EditionFacet tests
# ---------------------------------------------------------------------------

def test_edition_facet_distance_matches_edition_distance_for_albums():
    g = Album(artist="Yes", title="Close to the Edge", first_released=1972)
    pref = EditionPreference(markers=(), prefer_original=True)
    cand = PlatformCandidate(ref="orig", title="Close to the Edge", year=1972)
    assert EditionFacet(pref).distance(g, cand) == edition_distance(g, cand, pref)


def test_edition_facet_no_op_on_recordings():
    g = Recording(artist="Yes", title="Roundabout")
    pref = EditionPreference(markers=("steven wilson",))
    cand = PlatformCandidate(ref="x", title="Roundabout")
    assert EditionFacet(pref).distance(g, cand) == 0.0
    assert EditionFacet(pref).compromise(g, cand) is None


def test_edition_facet_compromise_when_no_marker_present():
    g = Album(artist="Yes", title="Close to the Edge", first_released=1972)
    pref = EditionPreference(markers=("steven wilson",), prefer_original=True)
    cand = PlatformCandidate(ref="orig", title="Close to the Edge", year=1972)
    comp = EditionFacet(pref).compromise(g, cand)
    assert comp is not None
    assert comp.facet == "edition" and comp.desired == "steven wilson"
    assert comp.note == "preferred edition (steven wilson) unavailable"


def test_edition_facet_no_compromise_when_marker_present():
    g = Album(artist="Yes", title="Close to the Edge", first_released=1972)
    pref = EditionPreference(markers=("steven wilson",))
    cand = PlatformCandidate(ref="sw", title="Close to the Edge (Steven Wilson Mix)", year=2013)
    assert EditionFacet(pref).compromise(g, cand) is None


# ---------------------------------------------------------------------------
# PerformanceFacet tests
# ---------------------------------------------------------------------------

def test_performance_facet_no_op_on_albums():
    from tidalist.core.fidelity import PerformanceFacet
    g = Album(artist="Traffic", title="Mr. Fantasy")
    cand = PlatformCandidate(ref="a", title="Mr. Fantasy")
    assert PerformanceFacet().distance(g, cand) == 0.0
    assert PerformanceFacet().compromise(g, cand) is None


def test_performance_facet_no_penalty_when_observation_unknown():
    from tidalist.core.fidelity import PerformanceFacet
    g = Recording(artist="t", title="Glad", performance=Performance.STUDIO)
    cand = PlatformCandidate(ref="x", title="Glad")  # performance UNKNOWN
    assert PerformanceFacet().distance(g, cand) == 0.0
    assert PerformanceFacet().compromise(g, cand) is None


def test_performance_facet_match_is_zero_no_compromise():
    from tidalist.core.fidelity import PerformanceFacet
    g = Recording(artist="t", title="Glad", performance=Performance.STUDIO)
    cand = PlatformCandidate(ref="x", title="Glad", performance=Performance.STUDIO)
    assert PerformanceFacet().distance(g, cand) == 0.0
    assert PerformanceFacet().compromise(g, cand) is None


def test_performance_facet_mismatch_penalizes_and_reports():
    from tidalist.core.fidelity import PerformanceFacet, W_PERFORMANCE
    g = Recording(artist="t", title="Glad", performance=Performance.STUDIO)
    cand = PlatformCandidate(ref="x", title="Glad (Live)", performance=Performance.LIVE)
    assert PerformanceFacet().distance(g, cand) == W_PERFORMANCE
    comp = PerformanceFacet().compromise(g, cand)
    assert comp is not None
    assert comp.facet == "performance"
    assert comp.desired == "studio" and comp.used == "live"
    assert comp.note == "studio take unavailable; used a live version"


# ---------------------------------------------------------------------------
# QualityPreference + audio-quality tiebreak tests
# ---------------------------------------------------------------------------

def test_quality_preference_ranks_hires_then_popularity():
    from tidalist.core.fidelity import QualityPreference, PlatformCandidate
    pref = QualityPreference()
    hi = PlatformCandidate(ref="hi", title="t", audio_quality="HI_RES_LOSSLESS", popularity=10)
    lo = PlatformCandidate(ref="lo", title="t", audio_quality="LOW", popularity=99)
    # hi-res dominates popularity: hi sorts before lo (smaller tiebreak tuple).
    assert pref.tiebreak(hi) < pref.tiebreak(lo)


def test_quality_preference_popularity_breaks_equal_quality():
    from tidalist.core.fidelity import QualityPreference, PlatformCandidate
    pref = QualityPreference()
    a = PlatformCandidate(ref="a", title="t", audio_quality="LOSSLESS", popularity=80)
    b = PlatformCandidate(ref="b", title="t", audio_quality="LOSSLESS", popularity=20)
    assert pref.tiebreak(a) < pref.tiebreak(b)   # higher popularity first


def test_choose_quality_tiebreak_picks_hires_on_a_tie():
    from tidalist.core.fidelity import QualityPreference, PlatformCandidate, choose
    from tidalist.core.recording import Recording
    g = Recording(artist="a", title="t")
    pref = QualityPreference()
    lossy = PlatformCandidate(ref="aaa", title="t", audio_quality="LOW")
    hires = PlatformCandidate(ref="hires", title="t", audio_quality="HI_RES_LOSSLESS")
    # No facets => render_distance 0 for both => the quality tiebreak decides.
    chosen, _ = choose(g, [lossy, hires], [], tiebreak=lambda c: (pref.tiebreak(c), c.ref))
    assert chosen.ref == "hires"


def test_choose_default_tiebreak_unchanged_uses_ref():
    from tidalist.core.fidelity import PlatformCandidate, choose
    from tidalist.core.recording import Recording
    g = Recording(artist="a", title="t")
    a = PlatformCandidate(ref="a", title="t")
    b = PlatformCandidate(ref="b", title="t")
    assert choose(g, [b, a], [])[0].ref == "a"   # default ref tiebreak still wins
