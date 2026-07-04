import pytest

from tidalist.core.recording import Candidate, Credit, Recording, Performance, Kind
from tidalist.core.identifiers import MBID, ExternalIds
from tidalist.core.album import Album
from tidalist.core.criteria import PerformedBy, Studio, NotCompilation, NotLive
from tidalist.core.brief import Brief
from tidalist.core.provenance import Provenance
from tidalist.core.golden import Curator, GoldenPlaylist, GoldenEntry
from tests.fakes import FakeMetadataProvider


def _rec(title="Glad", artist="Traffic", performer="Steve Winwood",
         performance=Performance.STUDIO, year=1970, mbid="mb-1"):
    return Recording(artist=artist, title=title, mbid=mbid, performance=performance,
                     first_released=year, credits=(Credit(performer, "performer"),))


def _brief(*criteria):
    return Brief("Winwood", tuple(criteria))


def _curate(meta_map, brief, candidates, provenances=None):
    return Curator(FakeMetadataProvider(meta_map)).curate(brief, candidates, provenances)


def test_curate_builds_a_golden_playlist_named_for_the_brief():
    golden = _curate({"Glad": _rec()}, _brief(), [Candidate("Traffic", "Glad")])
    assert isinstance(golden, GoldenPlaylist)
    assert golden.name == "Winwood"
    assert golden.brief == _brief()
    assert len(golden.entries) == 1
    assert isinstance(golden.entries[0], GoldenEntry)


def test_curate_admits_a_recording_that_satisfies_the_criteria():
    golden = _curate({"Glad": _rec()}, _brief(PerformedBy("Steve Winwood")),
                     [Candidate("Traffic", "Glad")])
    entry = golden.entries[0]
    assert entry.item.title == "Glad"
    assert entry.verdict.admitted


def test_curate_rejects_a_cover_but_keeps_the_entry_with_reasons():
    cover = _rec(title="Feelin Alright", artist="Joe Cocker", performer="Joe Cocker")
    golden = _curate({"Feelin Alright": cover}, _brief(PerformedBy("Steve Winwood")),
                     [Candidate("Joe Cocker", "Feelin Alright")])
    entry = golden.entries[0]
    assert not entry.verdict.admitted
    assert any("cover" in v for v in entry.verdict.violations)


def test_curate_picks_the_studio_take_over_a_live_one():
    studio = _rec(performance=Performance.STUDIO, year=1970, mbid="s")
    live = _rec(performance=Performance.LIVE, year=1973, mbid="l")
    golden = _curate({"Glad": [live, studio]}, _brief(), [Candidate("Traffic", "Glad")])
    assert golden.entries[0].item.mbid == "s"
    assert golden.entries[0].item.performance is Performance.STUDIO


def test_curate_chooses_among_admissible_under_a_studio_criterion():
    studio = _rec(performance=Performance.STUDIO, mbid="s")
    live = _rec(performance=Performance.LIVE, mbid="l")
    golden = _curate({"Glad": [live, studio]}, _brief(Studio()), [Candidate("Traffic", "Glad")])
    entry = golden.entries[0]
    assert entry.item.mbid == "s" and entry.verdict.admitted


def test_curate_surfaces_a_best_effort_recording_when_none_admissible():
    live = _rec(performance=Performance.LIVE, mbid="l")
    golden = _curate({"Glad": [live]}, _brief(Studio()), [Candidate("Traffic", "Glad")])
    entry = golden.entries[0]
    assert entry.item.mbid == "l"          # surfaced for review
    assert not entry.verdict.admitted
    assert any("live" in v for v in entry.verdict.violations)


def test_curate_reports_a_gap_when_no_recording_is_found():
    golden = _curate({}, _brief(), [Candidate("Nobody", "Nothing")])
    entry = golden.entries[0]
    assert entry.item.artist == "Nobody" and entry.item.title == "Nothing"
    assert entry.item.mbid is None
    assert not entry.verdict.admitted
    assert any("no recording" in v.lower() for v in entry.verdict.violations)


def test_curate_defaults_provenance_to_nl():
    golden = _curate({"Glad": _rec()}, _brief(), [Candidate("Traffic", "Glad")])
    assert golden.entries[0].provenance == Provenance("nl")


def test_curate_carries_per_candidate_provenance_including_notes():
    golden = _curate(
        {"A": _rec(title="A"), "B": _rec(title="B")}, _brief(),
        [Candidate("x", "A"), Candidate("x", "B")],
        provenances=[Provenance("scaruffi", "first"), Provenance("nl", "second")])
    assert [(e.provenance.source, e.provenance.note) for e in golden.entries] \
        == [("scaruffi", "first"), ("nl", "second")]


def test_curate_rejects_provenances_of_mismatched_length():
    with pytest.raises(ValueError):
        _curate({"A": _rec(title="A")}, _brief(), [Candidate("x", "A")],
                provenances=[Provenance("nl"), Provenance("nl")])


def test_curate_preserves_candidate_order():
    golden = _curate({"A": _rec(title="A"), "B": _rec(title="B")}, _brief(),
                     [Candidate("x", "B"), Candidate("x", "A")])
    assert [e.item.title for e in golden.entries] == ["B", "A"]


# ---------------------------------------------------------------------------
# Album candidate tests (Task 3)
# ---------------------------------------------------------------------------

def _album(title="John Barleycorn Must Die", artist="Traffic",
           mbid="mb-alb-1", year=1970):
    return Album(artist=artist, title=title, ids=ExternalIds(mbid=mbid), first_released=year)


def test_album_candidate_yields_album_golden_entry():
    """ALBUM kind → GoldenEntry whose item is the discovered Album, admitted."""
    alb = _album()
    candidate = Candidate("Traffic", "John Barleycorn Must Die", kind=Kind.ALBUM)
    curator = Curator(FakeMetadataProvider(albums={"John Barleycorn Must Die": alb}))
    golden = curator.curate(_brief(), [candidate])
    entry = golden.entries[0]
    assert isinstance(entry.item, Album)
    assert entry.item.ids.mbid == "mb-alb-1"
    assert entry.verdict.admitted


def test_album_candidate_admitted_even_under_restrictive_brief():
    """Album entries are not gated by recording-level brief criteria."""
    alb = _album()
    candidate = Candidate("Traffic", "John Barleycorn Must Die", kind=Kind.ALBUM)
    # Studio() + PerformedBy() would reject a recording that lacks them, but must not gate albums
    curator = Curator(FakeMetadataProvider(albums={"John Barleycorn Must Die": alb}))
    golden = curator.curate(_brief(Studio(), PerformedBy("Steve Winwood")), [candidate])
    entry = golden.entries[0]
    assert isinstance(entry.item, Album)
    assert entry.verdict.admitted


def test_album_candidate_no_album_found_yields_rejected_album_entry():
    """ALBUM kind with no provider results → rejected Album built from candidate."""
    candidate = Candidate("Traffic", "The Low Spark", kind=Kind.ALBUM)
    curator = Curator(FakeMetadataProvider())
    golden = curator.curate(_brief(), [candidate])
    entry = golden.entries[0]
    assert isinstance(entry.item, Album)
    assert entry.item.artist == "Traffic"
    assert entry.item.title == "The Low Spark"
    assert entry.item.ids.mbid is None
    assert not entry.verdict.admitted
    assert any("no album" in v.lower() for v in entry.verdict.violations)


def test_track_candidate_still_uses_recordings_path():
    """TRACK kind (default) still calls recordings_for — album data is ignored."""
    rec = _rec()
    candidate = Candidate("Traffic", "Glad")  # kind defaults to TRACK
    curator = Curator(FakeMetadataProvider(
        recordings={"Glad": rec},
        albums={"Glad": _album(title="Glad")},  # album present — must not be chosen
    ))
    golden = curator.curate(_brief(), [candidate])
    entry = golden.entries[0]
    assert isinstance(entry.item, Recording)
    assert entry.item.title == "Glad"


# --- Phase 4 Task 1: Curator judges albums via brief ---

def _comp_album(title="Greatest Hits", artist="Traffic"):
    from tidalist.core.album import ReleaseTrait
    return Album(artist=artist, title=title, ids=ExternalIds(mbid="mb-comp"), first_released=1975,
                 traits=frozenset({ReleaseTrait.COMPILATION}))


def _studio_album(title="John Barleycorn Must Die", artist="Traffic"):
    return Album(artist=artist, title=title, ids=ExternalIds(mbid="mb-studio"), first_released=1970,
                 traits=frozenset())


def test_album_candidate_rejected_when_compilation_under_not_compilation_brief():
    """A compilation album is rejected when NotCompilation is in the brief."""
    alb = _comp_album()
    candidate = Candidate("Traffic", "Greatest Hits", kind=Kind.ALBUM)
    curator = Curator(FakeMetadataProvider(albums={"Greatest Hits": alb}))
    golden = curator.curate(_brief(NotCompilation()), [candidate])
    entry = golden.entries[0]
    assert isinstance(entry.item, Album)
    assert not entry.verdict.admitted
    assert any("compilation" in v for v in entry.verdict.violations)


def test_album_candidate_admitted_when_not_compilation_under_not_compilation_brief():
    """A studio album is admitted under a NotCompilation brief."""
    alb = _studio_album()
    candidate = Candidate("Traffic", "John Barleycorn Must Die", kind=Kind.ALBUM)
    curator = Curator(FakeMetadataProvider(albums={"John Barleycorn Must Die": alb}))
    golden = curator.curate(_brief(NotCompilation()), [candidate])
    entry = golden.entries[0]
    assert isinstance(entry.item, Album)
    assert entry.verdict.admitted


def test_album_candidate_still_admitted_under_recording_only_brief():
    """PerformedBy and Studio are no-ops on albums — still admitted."""
    alb = _studio_album()
    candidate = Candidate("Traffic", "John Barleycorn Must Die", kind=Kind.ALBUM)
    curator = Curator(FakeMetadataProvider(albums={"John Barleycorn Must Die": alb}))
    golden = curator.curate(_brief(Studio(), PerformedBy("Steve Winwood")), [candidate])
    entry = golden.entries[0]
    assert isinstance(entry.item, Album)
    assert entry.verdict.admitted


# --- Phase 6 Task 1: per-candidate criteria ---

def test_candidate_criteria_are_applied_in_addition_to_brief_criteria():
    """A Candidate carrying PerformedBy(X) is rejected as cover even with empty brief."""
    cover = _rec(title="Feelin Alright", artist="Joe Cocker", performer="Joe Cocker")
    # PerformedBy("Steve Winwood") on the candidate — not in the brief
    candidate = Candidate("Joe Cocker", "Feelin Alright",
                          criteria=(PerformedBy("Steve Winwood"),))
    curator = Curator(FakeMetadataProvider({"Feelin Alright": cover}))
    golden = curator.curate(_brief(), [candidate])      # empty brief
    entry = golden.entries[0]
    assert not entry.verdict.admitted
    assert any("cover" in v for v in entry.verdict.violations)


def test_candidate_criteria_are_combined_with_brief_criteria():
    """Per-candidate criteria add to brief criteria — both must pass."""
    studio = _rec(performance=Performance.STUDIO, mbid="s",
                  performer="Steve Winwood")
    live = _rec(performance=Performance.LIVE, mbid="l",
                performer="Steve Winwood")
    # Brief says studio; candidate says performed_by — only studio+SW recording passes
    candidate = Candidate("Traffic", "Glad", criteria=(PerformedBy("Steve Winwood"),))
    curator = Curator(FakeMetadataProvider({"Glad": [studio, live]}))
    golden = curator.curate(_brief(Studio()), [candidate])
    entry = golden.entries[0]
    assert entry.item.mbid == "s"
    assert entry.verdict.admitted


def test_golden_entry_carries_edition_from_candidate():
    """GoldenEntry.edition is set from candidate.edition."""
    from tidalist.core.edition import EditionPreference
    pref = EditionPreference(markers=("steven wilson",), prefer_original=True)
    alb = _album()
    candidate = Candidate("Traffic", "John Barleycorn Must Die",
                          kind=Kind.ALBUM, edition=pref)
    curator = Curator(FakeMetadataProvider(albums={"John Barleycorn Must Die": alb}))
    golden = curator.curate(_brief(), [candidate])
    entry = golden.entries[0]
    assert entry.edition == pref


def test_golden_entry_edition_is_none_when_candidate_has_no_edition():
    """GoldenEntry.edition is None when candidate carries no edition."""
    alb = _album()
    candidate = Candidate("Traffic", "John Barleycorn Must Die", kind=Kind.ALBUM)
    curator = Curator(FakeMetadataProvider(albums={"John Barleycorn Must Die": alb}))
    golden = curator.curate(_brief(), [candidate])
    assert golden.entries[0].edition is None


# --- Task A2-Py: credit variants round-trip ---

def test_golden_roundtrips_credit_variants():
    from tidalist.core.spec import to_golden, from_golden
    from tidalist.core.recording import Credit
    from tidalist.core.album import Album
    from tidalist.core.golden import GoldenPlaylist, GoldenEntry
    from tidalist.core.brief import Brief
    from tidalist.core.criteria import Verdict
    from tidalist.core.provenance import Provenance

    album = Album(artist="Валерий Гергиев", title="Le Sacre du printemps",
                  credits=(Credit("Валерий Гергиев", "conductor", ("Valery Gergiev",)),))
    gp = GoldenPlaylist("t", Brief("t", ()),
                        (GoldenEntry(album, Provenance("nl"), Verdict(True, ())),))
    data = to_golden(gp)
    assert data["entries"][0]["credits"][0]["variants"] == ["Valery Gergiev"]
    back = from_golden(data)
    assert back.entries[0].item.credits[0].variants == ("Valery Gergiev",)


def test_golden_without_variants_loads_backcompat():
    from tidalist.core.spec import from_golden
    data = {"name": "t", "brief": {"criteria": []}, "entries": [{
        "kind": "album", "artist": "Traffic", "title": "John Barleycorn Must Die",
        "traits": [], "tracklist": [],
        "credits": [{"artist": "Traffic", "role": "artist"}],
        "provenance": {"source": "nl", "note": ""},
        "verdict": {"admitted": True, "violations": []}}]}
    album = from_golden(data).entries[0].item
    assert album.credits[0].variants == ()
