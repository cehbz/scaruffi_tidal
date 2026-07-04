"""Fidelity facets: per-dimension distance + compromise between a golden item and a
platform candidate. render_distance sums the applicable facets; choose is the argmin.
Edition is the first facet ported here; identity/release-class/performance/quality follow.
"""

import re
from dataclasses import dataclass
from typing import Protocol, runtime_checkable

from .album import Album, ReleaseTrait, TrackRef
from .catalog import Track
from .edition import EditionPreference
from .identifiers import ISRC
from .recording import Performance, Recording

# ---------------------------------------------------------------------------
# Edition-distance metric
# ---------------------------------------------------------------------------

# Weight constants for the multi-dimensional edition distance.
# Dominance invariant: W_MARKER must strictly exceed the maximum possible sum of
# all other dimensions so a single-rank difference in marker always outranks any
# combination of content differences.
W_MARKER    = 1_000_000   # requested-marker rank (dominating dimension)
W_TRACKLIST = 100         # per-track count/missing penalty
W_TITLE     = 10          # title mismatch penalty
W_YEAR      = 1           # per-year distance from golden's first_released
W_REISSUE   = 5           # per kind-marker found in option title

# Fuzzy-closeness weights for recording identity when ISRC doesn't decide.
# title/artist dominate W_PERFORMANCE (100); duration is the finest tiebreak.
W_FUZZY_TITLE  = 1000
W_FUZZY_ARTIST = 1000
W_FUZZY_DUR    = 50   # weight at full (ratio=1) duration mismatch; bounded below W_PERFORMANCE

# Weight constant for performance facet (studio/live mismatch).
W_PERFORMANCE = 100   # studio/live mismatch penalty for recordings

# Markers that indicate non-original kind (reissues, expansions, live, etc.)
_KIND_MARKERS = (
    "reissue", "remaster", "deluxe", "live", "compilation", "expanded", "anniversary"
)


def _norm(s: str | None) -> str:
    """Normalise a string for comparison: casefold + strip."""
    return (s or "").casefold().strip()


def recording_artist_match(recording: "Recording", artists: tuple[str, ...]) -> bool:
    """True if a performer of the recording overlaps the candidate's artists."""
    performers = {_norm(c.artist) for c in recording.credits if c.role == "performer"}
    performers.add(_norm(recording.artist))
    return any(p and any(p in _norm(a) or _norm(a) in p for a in artists)
               for p in performers)


def _title_tokens(s: str) -> frozenset[str]:
    """Word tokens of a title, lowercased, punctuation dropped."""
    return frozenset(re.findall(r"\w+", s.casefold()))


def _jaccard_distance(a: frozenset[str], b: frozenset[str]) -> float:
    """Token-set distance 1 - |a∩b|/|a∪b|, in [0,1]; 0 when both empty."""
    union = a | b
    if not union:
        return 0.0
    return 1.0 - len(a & b) / len(union)


@dataclass(frozen=True, slots=True)
class Compromise:
    """A reported deviation along one fidelity facet between desired and used."""
    facet: str
    desired: str
    used: str
    note: str


@dataclass(frozen=True, slots=True)
class EditionOption:
    """One available edition on a platform (`ref` is the platform handle)."""
    ref: str
    title: str
    year: int | None = None
    tracks: tuple[Track, ...] = ()


@dataclass(frozen=True, slots=True)
class PlatformCandidate:
    """A platform item carrying best-effort observed fidelity values. Unknowns stay
    None / UNKNOWN so the corresponding facet no-ops. `title`/`year`/`tracks` mirror
    EditionOption so edition_distance reads a candidate structurally."""
    ref: str
    title: str
    artists: tuple[str, ...] = ()
    isrc: ISRC | None = None
    year: int | None = None
    duration_s: int | None = None
    tracks: tuple[Track, ...] = ()
    release_class: frozenset[ReleaseTrait] | None = None
    performance: Performance = Performance.UNKNOWN
    source_kind: str | None = None
    audio_quality: str | None = None
    popularity: int | None = None


def _track_matches(ref: TrackRef, t: Track) -> bool:
    """True if the golden TrackRef matches a platform Track by ISRC or normalised title."""
    if ref.isrc is not None and t.isrc is not None:
        return ref.isrc == t.isrc
    return _norm(ref.title) == _norm(t.title)


def edition_distance(
    golden: Album | None,
    option: EditionOption,
    preference: EditionPreference,
) -> float:
    """Compute a weighted distance between a golden Album and an EditionOption.

    Lower is better. Dimensions:
    - requested-marker (dominating): rank of the first preference marker found in option title
    - tracklist: count_diff + missing tracks (when golden has a tracklist)
    - title: mismatch between golden and option title (gated by prefer_original)
    - year: |option.year - golden.first_released| (gated by prefer_original)
    - reissue/kind: number of kind markers in option title (gated by prefer_original)
    """
    total = 0.0

    # --- Requested-marker dimension (always applies) ---
    title_lower = option.title.casefold()
    rank = min(
        (i for i, m in enumerate(preference.markers) if m in title_lower),
        default=len(preference.markers),
    )
    total += W_MARKER * rank

    # --- Tracklist dimension (always applies when golden has a tracklist) ---
    if golden is not None and golden.tracklist:
        count_diff = abs(len(golden.tracklist) - len(option.tracks))
        missing = sum(
            1 for g in golden.tracklist
            if not any(_track_matches(g, t) for t in option.tracks)
        )
        total += W_TRACKLIST * (count_diff + missing)

    # --- prefer_original-gated dimensions ---
    if preference.prefer_original:
        # Title dimension (requires golden)
        if golden is not None and _norm(golden.title) != _norm(option.title):
            total += W_TITLE

        # Year dimension (requires golden with first_released)
        if golden is not None and golden.first_released is not None:
            if option.year is not None:
                total += W_YEAR * abs(option.year - golden.first_released)
            else:
                # Unknown year is penalised as if arbitrarily far; use W_TITLE as a
                # reasonable upper bound (less than W_MARKER but more than a close year).
                total += W_TITLE

        # Reissue/kind dimension (applies whenever prefer_original is set)
        total += W_REISSUE * sum(1 for m in _KIND_MARKERS if m in title_lower)

    return total


@dataclass(frozen=True, slots=True)
class EditionFacet:
    """Edition-fidelity facet: wraps edition_distance; no-ops only for Recordings."""
    preference: EditionPreference
    name: str = "edition"
    weight: float = 1.0

    def distance(self, golden, cand) -> float:
        # No-op only for recordings. golden may be None (legacy choose_edition):
        # edition_distance handles None and still scores marker + reissue dimensions.
        if isinstance(golden, Recording):
            return 0.0
        return edition_distance(golden, cand, self.preference)

    def compromise(self, golden, cand):
        if isinstance(golden, Recording) or not self.preference.markers:
            return None
        title_lower = cand.title.casefold()
        if any(m in title_lower for m in self.preference.markers):
            return None
        marker = self.preference.markers[0]
        return Compromise("edition", marker, "(no preferred edition)",
                          f"preferred edition ({marker}) unavailable")


@dataclass(frozen=True, slots=True)
class IdentityFacet:
    """Recording identity facet: exact ISRC match scores 0; albums always score 0.

    When ISRCs are absent or differ, falls back to fuzzy closeness over title,
    artist, and duration so a different-ISRC live take is ranked by content
    similarity rather than being nuked below a no-ISRC wrong song.
    """
    name: str = "identity"
    weight: float = 1.0

    def distance(self, golden, cand) -> float:
        if not isinstance(golden, Recording):
            return 0.0
        # Exact ISRC is a positive identity signal.
        if golden.isrc is not None and cand.isrc is not None and golden.isrc == cand.isrc:
            return 0.0
        # Otherwise fuzzy closeness: graded title (token-set Jaccard) + artist + relative duration.
        d = 0.0
        d += W_FUZZY_TITLE * _jaccard_distance(_title_tokens(golden.title),
                                               _title_tokens(cand.title))
        if not recording_artist_match(golden, cand.artists):
            d += W_FUZZY_ARTIST
        if golden.duration_s and cand.duration_s:
            d += W_FUZZY_DUR * abs(golden.duration_s - cand.duration_s) / max(golden.duration_s, cand.duration_s)
        return d

    def compromise(self, golden, cand):
        """Never emits a compromise."""
        return None


@dataclass(frozen=True, slots=True)
class PerformanceFacet:
    """Penalizes a recording whose observed performance differs from the golden's
    desired performance, and reports it. No-ops on albums and when either performance
    is UNKNOWN (unobserved ⇒ best-effort, no penalty)."""
    name: str = "performance"
    weight: float = 1.0

    def distance(self, golden, cand) -> float:
        if not isinstance(golden, Recording):
            return 0.0
        if golden.performance is Performance.UNKNOWN or cand.performance is Performance.UNKNOWN:
            return 0.0
        return 0.0 if cand.performance == golden.performance else W_PERFORMANCE

    def compromise(self, golden, cand):
        if not isinstance(golden, Recording):
            return None
        if golden.performance is Performance.UNKNOWN or cand.performance is Performance.UNKNOWN:
            return None
        if cand.performance == golden.performance:
            return None
        return Compromise("performance", golden.performance.value, cand.performance.value,
                          f"{golden.performance.value} take unavailable; "
                          f"used a {cand.performance.value} version")


@runtime_checkable
class Facet(Protocol):
    name: str
    weight: float
    def distance(self, golden: "Album | Recording", cand: "PlatformCandidate") -> float: ...
    def compromise(self, golden: "Album | Recording",
                   cand: "PlatformCandidate") -> "Compromise | None": ...


def render_distance(golden, cand, facets) -> float:
    return sum(f.weight * f.distance(golden, cand) for f in facets)


_QUALITY_RANK: dict[str, int] = {
    "low": 0, "high": 1, "lossless": 2,
    "hi_res": 3, "hi_res_lossless": 3, "master": 3, "max": 3,
}


def _audio_rank(quality: str | None) -> int:
    return _QUALITY_RANK.get((quality or "").casefold(), 0)


@dataclass(frozen=True, slots=True)
class QualityPreference:
    """A specifiable quality tiebreak: prefer hi-res, then more popular. The distance-0
    quality layer — it breaks ties below every fidelity facet, never overriding one."""
    prefer_hires: bool = True
    prefer_popular: bool = True

    def tiebreak(self, cand) -> tuple:
        """Sort key, lower is better: higher audio rank and higher popularity first."""
        hires = -_audio_rank(cand.audio_quality) if self.prefer_hires else 0
        popular = -(cand.popularity or 0) if self.prefer_popular else 0
        return (hires, popular)


def choose(golden, candidates, facets, tiebreak=None):
    """Pick the candidate of minimum render_distance; break ties by `tiebreak`
    (default: the candidate ref, for determinism); return it plus the winner's
    compromises."""
    if not candidates:
        return None, ()
    second = tiebreak if tiebreak is not None else (lambda c: c.ref)
    chosen = min(candidates, key=lambda c: (render_distance(golden, c, facets), second(c)))
    comps = tuple(
        c for c in (f.compromise(golden, chosen) for f in facets) if c is not None
    )
    return chosen, comps


def choose_edition(
    options: list[EditionOption],
    preference: EditionPreference,
    golden: Album | None = None,
) -> tuple[EditionOption | None, str | None]:
    """Back-compat edition selector: delegates to choose over a single EditionFacet,
    returning the chosen option and a string compromise (or None)."""
    chosen, comps = choose(golden, options, [EditionFacet(preference)])
    return chosen, (comps[0].note if comps else None)
