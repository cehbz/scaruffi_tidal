"""Steve Winwood north-star end-to-end acceptance test.

Exercises the full offline pipeline:
    parse_intent → Curator.curate → render → publish

Proves that a mixed album+track intent:
  - admits real album and track entries in order
  - excludes covers (PerformedBy per-candidate criterion)
  - excludes compilations (NotCompilation brief criterion)
  - renders: albums expand to multiple tracks, tracks resolve to single items
  - publishes to a playlist reference
"""
import json
import pathlib

from tidalist.nl.intent import parse_intent
from tidalist.core.golden import Curator
from tidalist.core.render import render, publish, PlatformItem, MatchQuality
from tidalist.core.recording import Recording, Credit, Performance, Kind
from tidalist.core.album import Album, ReleaseTrait

from tests.fakes import FakeMetadataProvider, FakeRenderer


FIXTURE = pathlib.Path(__file__).parent / "fixtures" / "winwood_intent.json"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _winwood_credit():
    return (Credit("Steve Winwood", "performer"),)


def _studio_recording(title: str, artist: str = "Steve Winwood") -> Recording:
    return Recording(
        artist=artist,
        title=title,
        performance=Performance.STUDIO,
        credits=_winwood_credit(),
    )


def _platform_item(ref: str, title: str) -> PlatformItem:
    return PlatformItem(ref=ref, title=title, artists=("Steve Winwood",),
                        quality=MatchQuality.STRONG)


def _album(artist: str, title: str, traits: frozenset = frozenset()) -> Album:
    return Album(artist=artist, title=title, traits=traits)


# ---------------------------------------------------------------------------
# Fixture: five album candidates + three track candidates
# Titles must exactly match the fixture JSON (case-fold comparison in fakes).
# ---------------------------------------------------------------------------

ALBUM_TITLES = [
    ("Traffic", "Mr. Fantasy"),
    ("Traffic", "John Barleycorn Must Die"),
    ("Traffic", "The Low Spark of High Heeled Boys"),
    ("Blind Faith", "Blind Faith"),
    ("The Spencer Davis Group", "Their First LP"),
    # "The Finer Things" is a real Winwood box-set compilation → rejected by NotCompilation
    ("Steve Winwood", "The Finer Things"),
]

TRACK_TITLES = [
    "Gimme Some Lovin'",
    "Higher Love",
    # "Valerie" will be seeded as a COVER (no Winwood performer credit) → rejected
    "Valerie",
]


def _build_metadata_provider() -> FakeMetadataProvider:
    """Seed:
    - 5 admitted albums (studio)
    - 1 compilation album ("The Finer Things") → rejected by brief NotCompilation
    - 2 admitted track recordings (Winwood as performer)
    - 1 cover recording for 'Valerie' → rejected by per-candidate PerformedBy
    """
    albums = {
        "Mr. Fantasy": _album("Traffic", "Mr. Fantasy"),
        "John Barleycorn Must Die": _album("Traffic", "John Barleycorn Must Die"),
        "The Low Spark of High Heeled Boys": _album("Traffic", "The Low Spark of High Heeled Boys"),
        "Blind Faith": _album("Blind Faith", "Blind Faith"),
        "Their First LP": _album("The Spencer Davis Group", "Their First LP"),
        # A genuine compilation — brief NotCompilation must reject it
        "The Finer Things": _album("Steve Winwood", "The Finer Things",
                                   traits=frozenset({ReleaseTrait.COMPILATION})),
    }
    recordings = {
        "Gimme Some Lovin'": _studio_recording("Gimme Some Lovin'"),
        "Higher Love": _studio_recording("Higher Love"),
        # Cover: performed by Kygo, not Steve Winwood — PerformedBy must reject it
        "Valerie": Recording(
            artist="Kygo",
            title="Valerie",
            performance=Performance.STUDIO,
            credits=(Credit("Kygo", "performer"),),  # NO Steve Winwood credit
        ),
    }
    return FakeMetadataProvider(recordings=recordings, albums=albums)


def _build_renderer() -> FakeRenderer:
    """Seed platform items for admitted entries.

    Albums: each expands to 3 tracks (enough to verify expansion).
    Tracks: one item per admitted recording.
    """
    def _album_tracks(album_title: str, n: int = 3):
        return [
            _platform_item(f"{album_title}-track-{i}", f"{album_title} Track {i}")
            for i in range(1, n + 1)
        ]

    albums = {
        "Mr. Fantasy": (_album_tracks("Mr. Fantasy"), None),
        "John Barleycorn Must Die": (_album_tracks("John Barleycorn Must Die"), None),
        "The Low Spark of High Heeled Boys": (_album_tracks("The Low Spark of High Heeled Boys"), None),
        "Blind Faith": (_album_tracks("Blind Faith"), None),
        "Their First LP": (_album_tracks("Their First LP"), None),
        # The Finer Things is NOT seeded — the compilation is rejected before render sees it
    }
    items = {
        "Gimme Some Lovin'": _platform_item("track-gsl", "Gimme Some Lovin'"),
        "Higher Love": _platform_item("track-hl", "Higher Love"),
        # Valerie is NOT seeded — it will be rejected before render sees it
    }
    return FakeRenderer(items=items, albums=albums)


# ---------------------------------------------------------------------------
# Acceptance test
# ---------------------------------------------------------------------------

def test_winwood_end_to_end():
    """Full offline pipeline: intent → golden → rendering → publish."""

    # --- parse intent -------------------------------------------------------
    with open(FIXTURE) as f:
        data = json.load(f)

    candidates, provenances, brief = parse_intent(data)

    # Sanity: 6 albums + 3 tracks
    assert len(candidates) == 9
    album_candidates = [c for c in candidates if c.kind is Kind.ALBUM]
    track_candidates = [c for c in candidates if c.kind is Kind.TRACK]
    assert len(album_candidates) == 6
    assert len(track_candidates) == 3

    # Brief carries NotCompilation
    assert any(type(cr).__name__ == "NotCompilation" for cr in brief.criteria)

    # Each track candidate carries PerformedBy("Steve Winwood")
    for tc in track_candidates:
        assert any(
            type(cr).__name__ == "PerformedBy" and cr.artist == "Steve Winwood"
            for cr in tc.criteria
        ), f"Track candidate '{tc.title}' is missing PerformedBy(Steve Winwood)"

    # --- golden curation ----------------------------------------------------
    provider = _build_metadata_provider()
    golden = Curator(provider).curate(brief, candidates, provenances)

    assert golden.name == "Steve Winwood — Albums & Classics"
    assert len(golden.entries) == 9  # all candidates have an entry (admitted or not)

    # Check order preserved: first 6 entries are albums, last 3 are tracks
    from tidalist.core.recording import Recording as Rec
    for i, entry in enumerate(golden.entries[:6]):
        assert isinstance(entry.item, Album), f"Entry {i} should be Album"
    for i, entry in enumerate(golden.entries[6:], start=6):
        assert isinstance(entry.item, Rec), f"Entry {i} should be Recording"

    # --- admitted albums: 5 out of 6 ----------------------------------------
    admitted_albums = [e for e in golden.entries
                       if isinstance(e.item, Album) and e.verdict.admitted]
    rejected_albums = [e for e in golden.entries
                       if isinstance(e.item, Album) and not e.verdict.admitted]

    assert len(admitted_albums) == 5, (
        f"Expected 5 admitted albums, got {len(admitted_albums)}: "
        f"{[e.item.title for e in admitted_albums]}"
    )
    assert len(rejected_albums) == 1

    # The Finer Things must be the rejected album (compilation)
    rejected_album = rejected_albums[0]
    assert rejected_album.item.title == "The Finer Things"
    assert any("compilation" in v.lower() for v in rejected_album.verdict.violations), (
        f"Expected 'compilation' in violations, got: {rejected_album.verdict.violations}"
    )

    # --- admitted tracks: 2 out of 3 ----------------------------------------
    admitted_tracks = [e for e in golden.entries
                       if isinstance(e.item, Rec) and e.verdict.admitted]
    rejected_tracks = [e for e in golden.entries
                       if isinstance(e.item, Rec) and not e.verdict.admitted]

    assert len(admitted_tracks) == 2, (
        f"Expected 2 admitted tracks, got {len(admitted_tracks)}: "
        f"{[e.item.title for e in admitted_tracks]}"
    )
    assert len(rejected_tracks) == 1

    # Valerie must be the rejected track (cover — no Winwood performer credit)
    rejected_track = rejected_tracks[0]
    assert rejected_track.item.title == "Valerie"
    assert any("cover" in v.lower() for v in rejected_track.verdict.violations), (
        f"Expected 'cover' in violations, got: {rejected_track.verdict.violations}"
    )

    # Admitted tracks: Gimme Some Lovin' and Higher Love
    admitted_track_titles = {e.item.title for e in admitted_tracks}
    assert "Gimme Some Lovin'" in admitted_track_titles
    assert "Higher Love" in admitted_track_titles

    # --- render ------------------------------------------------------------
    renderer = _build_renderer()
    rendering = render(golden, renderer)

    # render only processes admitted entries (5 albums + 2 tracks = 7)
    assert len(rendering.entries) == 7

    resolved = rendering.resolved()
    gaps = rendering.gaps()

    # All 7 should resolve (no gaps expected — everything admitted was seeded)
    assert len(gaps) == 0, f"Unexpected gaps: {[g.item.title for g in gaps]}"
    assert len(resolved) == 7

    # Albums should expand to multiple tracks (3 each); tracks to 1 each
    album_entries = [e for e in resolved if isinstance(e.golden.item, Album)]
    track_entries = [e for e in resolved if isinstance(e.golden.item, Rec)]

    assert len(album_entries) == 5
    assert len(track_entries) == 2

    # Each admitted album expands to 3 platform items
    for ae in album_entries:
        assert len(ae.items) == 3, (
            f"Album '{ae.golden.item.title}' expanded to {len(ae.items)} items, expected 3"
        )

    # Each admitted track resolves to exactly 1 platform item
    for te in track_entries:
        assert len(te.items) == 1, (
            f"Track '{te.golden.item.title}' resolved to {len(te.items)} items, expected 1"
        )

    # --- publish ------------------------------------------------------------
    ref = publish(rendering, renderer)

    assert ref is not None
    assert ref.startswith("playlist-")

    # Total items published: 5 albums × 3 tracks + 2 single tracks = 17
    assert len(renderer.emitted) == 1
    _, emitted_refs, emitted_ref = renderer.emitted[0]
    assert emitted_ref == ref
    assert len(emitted_refs) == 17, (
        f"Expected 17 published items (5×3 album tracks + 2 single tracks), "
        f"got {len(emitted_refs)}"
    )
