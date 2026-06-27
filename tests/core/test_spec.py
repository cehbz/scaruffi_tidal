import json

import pytest

from tidalist.core.identifiers import ISRC, MBID, ExternalIds
from tidalist.core.recording import Candidate, Credit, Recording, Performance, Kind
from tidalist.core.album import Album, ReleaseTrait
from tidalist.core.criteria import PerformedBy, Studio, NotCompilation, NotLive, Verdict
from tidalist.core.brief import Brief
from tidalist.core.provenance import Provenance
from tidalist.core.golden import GoldenPlaylist, GoldenEntry
from tidalist.core.spec import to_golden, from_golden, to_intent, from_intent


def test_unknown_criterion_type_raises():
    bad = {"name": "x", "brief": {"criteria": [{"type": "nope"}]}, "entries": []}
    with pytest.raises(ValueError):
        from_golden(bad)


# --- golden artifact ---------------------------------------------------------

def _golden():
    brief = Brief("Winwood", (PerformedBy("Steve Winwood"), Studio()))
    admitted = GoldenEntry(
        Recording(artist="Traffic", title="Glad", mbid=MBID("rec-1"),
                  isrc=ISRC("GBABC1234567"), album="John Barleycorn Must Die",
                  first_released=1970, duration_s=386, performance=Performance.STUDIO,
                  credits=(Credit("Steve Winwood", "performer"),)),
        Provenance("nl", "signature track"), Verdict.ok())
    gap = GoldenEntry(Recording(artist="Nobody", title="Nothing"),
                      Provenance("nl"), Verdict.rejected("no recording found"))
    return GoldenPlaylist("Winwood", brief, (admitted, gap))


def test_golden_round_trips():
    golden = _golden()
    assert from_golden(to_golden(golden)) == golden


def test_golden_is_pure_json():
    spec = to_golden(_golden())
    assert json.loads(json.dumps(spec)) == spec


def test_golden_entry_flattens_recording_fields_with_year_key():
    entry = to_golden(_golden())["entries"][0]
    assert entry["mbid"] == "rec-1" and entry["year"] == 1970
    assert entry["artist"] == "Traffic" and entry["title"] == "Glad"
    assert entry["provenance"]["note"] == "signature track"
    assert entry["verdict"]["admitted"] is True


# --- intent artifact (the front-end hand-off: candidates + notes + brief) -----

def _intent():
    brief = Brief("Winwood", (PerformedBy("Steve Winwood"), Studio()))
    candidates = [Candidate("Traffic", "Glad", album="John Barleycorn Must Die", year=1970),
                  Candidate("Blind Faith", "Presence of the Lord")]
    provenances = [Provenance("nl", "signature Traffic track"),
                   Provenance("nl", "supergroup peak")]
    return brief, candidates, provenances


def test_intent_round_trips():
    brief, candidates, provenances = _intent()
    c2, p2, b2 = from_intent(to_intent(brief, candidates, provenances))
    assert c2 == candidates and p2 == provenances and b2 == brief


def test_intent_is_pure_json():
    brief, candidates, provenances = _intent()
    spec = to_intent(brief, candidates, provenances)
    assert json.loads(json.dumps(spec)) == spec


def test_from_intent_attaches_per_candidate_note_as_provenance():
    data = {"name": "x", "brief": {"criteria": []},
            "candidates": [{"artist": "A", "title": "T", "note": "because"}]}
    candidates, provenances, _ = from_intent(data)
    assert candidates[0] == Candidate("A", "T")
    assert provenances[0] == Provenance("nl", "because")


def test_from_intent_source_defaults_to_nl_and_is_overridable():
    data = {"name": "x", "brief": {"criteria": []},
            "candidates": [{"artist": "A", "title": "T"}]}
    assert from_intent(data)[1][0].source == "nl"
    assert from_intent(data, source="scaruffi")[1][0].source == "scaruffi"


def test_golden_round_trips_album_and_track_entries():
    brief = Brief("Winwood", ())
    track = GoldenEntry(Recording(artist="Traffic", title="Glad", mbid=MBID("r1"),
                                  first_released=1970, performance=Performance.STUDIO),
                        Provenance("nl"), Verdict.ok())
    album = GoldenEntry(Album(artist="Traffic", title="John Barleycorn Must Die",
                              ids=ExternalIds(mbid=MBID("rg1")), first_released=1970),
                        Provenance("nl", "whole album"), Verdict.ok())
    g = GoldenPlaylist("Winwood", brief, (track, album))
    assert from_golden(to_golden(g)) == g
    dicts = to_golden(g)["entries"]
    assert dicts[0]["kind"] == "track" and dicts[1]["kind"] == "album"
    assert dicts[1]["mbid"] == "rg1"


def test_intent_round_trips_candidate_kind():
    brief = Brief("x", ())
    candidates = [Candidate("Traffic", "John Barleycorn Must Die", kind=Kind.ALBUM),
                  Candidate("Spencer Davis Group", "Gimme Some Lovin'")]  # default TRACK
    provenances = [Provenance("nl", "album"), Provenance("nl", "track")]
    c2, _, _ = from_intent(to_intent(brief, candidates, provenances))
    assert [c.kind for c in c2] == [Kind.ALBUM, Kind.TRACK]


# --- Phase 4 Task 1: new criteria serialization ---

def test_not_compilation_round_trips():
    brief = Brief("x", (NotCompilation(),))
    d = {"name": "x", "brief": {"criteria": [{"type": "not_compilation"}]}, "entries": []}
    g = from_golden(d)
    assert g.brief.criteria[0] == NotCompilation()


def test_not_live_round_trips():
    brief = Brief("x", (NotLive(),))
    d = {"name": "x", "brief": {"criteria": [{"type": "not_live"}]}, "entries": []}
    g = from_golden(d)
    assert g.brief.criteria[0] == NotLive()


def test_not_compilation_serializes_to_expected_dict():
    from tidalist.core.spec import _criterion_to_dict
    assert _criterion_to_dict(NotCompilation()) == {"type": "not_compilation"}


def test_not_live_serializes_to_expected_dict():
    from tidalist.core.spec import _criterion_to_dict
    assert _criterion_to_dict(NotLive()) == {"type": "not_live"}


# --- Phase 4 Task 1: album golden entry carries type fields ---

def test_golden_album_entry_round_trips_with_traits():
    brief = Brief("Winwood", ())
    album = Album(artist="Traffic", title="John Barleycorn Must Die",
                  ids=ExternalIds(mbid=MBID("rg1")), first_released=1970,
                  traits=frozenset({ReleaseTrait.LIVE}))
    entry = GoldenEntry(album, Provenance("nl"), Verdict.ok())
    g = GoldenPlaylist("Winwood", brief, (entry,))
    result = from_golden(to_golden(g))
    a = result.entries[0].item
    assert isinstance(a, Album)
    assert a.traits == frozenset({ReleaseTrait.LIVE})


def test_golden_album_entry_serializes_traits():
    from tidalist.core.spec import _golden_entry_to_dict
    from tidalist.core.provenance import Provenance
    from tidalist.core.golden import GoldenEntry
    album = Album(artist="Traffic", title="John Barleycorn Must Die",
                  traits=frozenset({ReleaseTrait.COMPILATION}))
    entry = GoldenEntry(album, Provenance("nl"), Verdict.ok())
    d = _golden_entry_to_dict(entry)
    assert d["traits"] == ["compilation"]


# --- Phase 6 Task 1: per-candidate criteria/edition/artist_mbid in intent ---

def test_intent_round_trips_per_candidate_criteria():
    """criteria on a Candidate round-trips through to_intent/from_intent."""
    brief = Brief("x", ())
    candidates = [Candidate("Traffic", "Glad",
                            criteria=(PerformedBy("Steve Winwood"), Studio()))]
    provenances = [Provenance("nl", "note")]
    c2, _, _ = from_intent(to_intent(brief, candidates, provenances))
    assert c2[0].criteria == (PerformedBy("Steve Winwood"), Studio())


def test_intent_round_trips_per_candidate_edition():
    """edition on a Candidate round-trips through to_intent/from_intent."""
    from tidalist.core.edition import EditionPreference
    pref = EditionPreference(markers=("steven wilson",), prefer_original=True)
    brief = Brief("x", ())
    candidates = [Candidate("Traffic", "John Barleycorn Must Die",
                            kind=Kind.ALBUM, edition=pref)]
    provenances = [Provenance("nl")]
    c2, _, _ = from_intent(to_intent(brief, candidates, provenances))
    assert c2[0].edition == pref


def test_intent_round_trips_per_candidate_edition_none():
    """edition=None omits edition from the serialized dict."""
    brief = Brief("x", ())
    candidates = [Candidate("Traffic", "Glad")]
    provenances = [Provenance("nl")]
    spec = to_intent(brief, candidates, provenances)
    assert "edition" not in spec["candidates"][0]
    c2, _, _ = from_intent(spec)
    assert c2[0].edition is None


def test_intent_round_trips_per_candidate_artist_mbid():
    """artist_mbid on a Candidate round-trips through to_intent/from_intent."""
    brief = Brief("x", ())
    candidates = [Candidate("Traffic", "Glad", artist_mbid=MBID("a-traffic"))]
    provenances = [Provenance("nl")]
    c2, _, _ = from_intent(to_intent(brief, candidates, provenances))
    assert c2[0].artist_mbid == MBID("a-traffic")


def test_intent_round_trips_per_candidate_artist_mbid_none():
    """artist_mbid=None omits the key from the serialized dict."""
    brief = Brief("x", ())
    candidates = [Candidate("Traffic", "Glad")]
    provenances = [Provenance("nl")]
    spec = to_intent(brief, candidates, provenances)
    assert "artist_mbid" not in spec["candidates"][0]
    c2, _, _ = from_intent(spec)
    assert c2[0].artist_mbid is None


def test_golden_entry_edition_round_trips():
    """GoldenEntry with an edition preference round-trips through to_golden/from_golden."""
    from tidalist.core.edition import EditionPreference
    pref = EditionPreference(markers=("steven wilson",), prefer_original=True)
    brief = Brief("x", ())
    album = Album(artist="Traffic", title="John Barleycorn Must Die",
                  ids=ExternalIds(mbid=MBID("rg1")), first_released=1970)
    entry = GoldenEntry(album, Provenance("nl"), Verdict.ok(), edition=pref)
    g = GoldenPlaylist("x", brief, (entry,))
    result = from_golden(to_golden(g))
    assert result.entries[0].edition == pref


def test_golden_entry_edition_none_round_trips():
    """GoldenEntry without edition preference round-trips as None."""
    brief = Brief("x", ())
    album = Album(artist="Traffic", title="John Barleycorn Must Die",
                  ids=ExternalIds(mbid=MBID("rg1")), first_released=1970)
    entry = GoldenEntry(album, Provenance("nl"), Verdict.ok())
    g = GoldenPlaylist("x", brief, (entry,))
    result = from_golden(to_golden(g))
    assert result.entries[0].edition is None


# --- Phase 2 (edition-distance): Album.tracklist round-trips ---

def test_golden_album_entry_with_tracklist_round_trips():
    """Album golden entry carrying a 2-track tracklist round-trips exactly."""
    from tidalist.core.album import TrackRef
    tracks = (
        TrackRef(position=1, title="Glad", isrc=ISRC("GBABC1234567"),
                 mbid=MBID("rec-1"), duration_s=386),
        TrackRef(position=2, title="Freedom Rider"),  # None fields
    )
    brief = Brief("x", ())
    album = Album(artist="Traffic", title="John Barleycorn Must Die",
                  ids=ExternalIds(mbid=MBID("rg1")), first_released=1970, tracklist=tracks)
    entry = GoldenEntry(album, Provenance("nl"), Verdict.ok())
    g = GoldenPlaylist("x", brief, (entry,))
    result = from_golden(to_golden(g))
    assert result.entries[0].item.tracklist == tracks  # type: ignore[union-attr]


def test_golden_album_entry_tracklist_is_pure_json():
    """Tracklist serializes to JSON-safe types."""
    from tidalist.core.album import TrackRef
    from tidalist.core.spec import _golden_entry_to_dict
    tracks = (
        TrackRef(position=1, title="Glad", isrc=ISRC("GBABC1234567"),
                 mbid=MBID("rec-1"), duration_s=386),
    )
    album = Album(artist="Traffic", title="John Barleycorn Must Die", tracklist=tracks)
    entry = GoldenEntry(album, Provenance("nl"), Verdict.ok())
    d = _golden_entry_to_dict(entry)
    assert json.loads(json.dumps(d)) == d
    assert d["tracklist"][0] == {
        "position": 1, "title": "Glad", "isrc": "GBABC1234567",
        "mbid": "rec-1", "duration_s": 386,
    }


def test_golden_album_entry_missing_tracklist_key_loads_as_empty():
    """Back-compat: a golden dict with no 'tracklist' key yields tracklist=()."""
    d = {
        "name": "x",
        "brief": {"criteria": []},
        "entries": [{
            "kind": "album", "artist": "Traffic",
            "title": "John Barleycorn Must Die",
            "provenance": {"source": "nl", "note": ""},
            "verdict": {"admitted": True, "violations": []},
        }],
    }
    result = from_golden(d)
    assert result.entries[0].item.tracklist == ()  # type: ignore[union-attr]


def test_album_entry_round_trips_with_external_ids():
    from tidalist.core.identifiers import DiscogsMasterId, DiscogsReleaseId, Source
    album = Album(artist="Traffic", title="Mr. Fantasy",
                  ids=ExternalIds(mbid=MBID("rg-mf"),
                                  discogs_master_id=DiscogsMasterId(639224),
                                  discogs_release_id=DiscogsReleaseId(1234567),
                                  sources=frozenset({Source.MUSICBRAINZ, Source.DISCOGS})),
                  first_released=1967)
    entry = GoldenEntry(album, Provenance("nl"), Verdict.ok())
    golden = GoldenPlaylist("x", Brief("x", ()), (entry,))
    assert from_golden(to_golden(golden)) == golden
    d = to_golden(golden)["entries"][0]
    assert d["mbid"] == "rg-mf"
    assert d["discogs_master_id"] == 639224
    assert d["discogs_release_id"] == 1234567
    assert d["sources"] == ["discogs", "musicbrainz"]


def test_legacy_album_golden_without_discogs_keys_loads():
    legacy = {
        "name": "x", "brief": {"criteria": []},
        "entries": [{
            "kind": "album", "mbid": "rg-old", "artist": "Traffic",
            "title": "Mr. Fantasy", "year": 1967, "traits": [], "tracklist": [],
            "provenance": {"source": "nl", "note": ""},
            "verdict": {"admitted": True, "violations": []},
        }],
    }
    album = from_golden(legacy).entries[0].item
    assert album.ids == ExternalIds(mbid=MBID("rg-old"))
