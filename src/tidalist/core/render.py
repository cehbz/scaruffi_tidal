"""The render stage: map a golden playlist onto one platform, best-effort.

A `Renderer` resolves a recording to a platform item (ISRC-first, then closeness) and
emits a playlist. `render` resolves every admitted golden entry into a `Rendering`
(resolved items + gaps) without writing; `publish` then emits the resolved items. The
two are separate so gaps can be reviewed before anything is created on the platform.
"""

from dataclasses import dataclass
from enum import StrEnum
from typing import NamedTuple, Protocol, runtime_checkable

from .album import Album
from .edition import EditionPreference, EditionPolicy
from .identifiers import ISRC
from .recording import Recording
from .golden import GoldenEntry, GoldenPlaylist
from .errors import PlatformError
from .fidelity import (
    W_MARKER, W_TRACKLIST, W_TITLE, W_YEAR, W_REISSUE,
    Compromise, EditionOption, edition_distance, choose_edition,
)


class MatchQuality(StrEnum):
    """How confidently a recording was matched to a platform item."""
    ISRC = "isrc"        # exact: same recording by ISRC
    STRONG = "strong"    # title and artist agree
    WEAK = "weak"        # found something, low confidence


@dataclass(frozen=True, slots=True)
class PlatformItem:
    """A resolved playable item: `ref` is the token `emit` needs (a track id, a path)."""
    ref: str
    title: str
    artists: tuple[str, ...]
    isrc: ISRC | None = None
    quality: MatchQuality = MatchQuality.WEAK


@dataclass(frozen=True, slots=True)
class RenderedEntry:
    golden: GoldenEntry
    items: tuple[PlatformItem, ...] = ()
    compromises: tuple[Compromise, ...] = ()
    gap_reason: str | None = None  # machine-readable reason when items is empty (album gaps only)

    @property
    def is_gap(self) -> bool:
        return not self.items


@dataclass(frozen=True, slots=True)
class Rendering:
    name: str
    entries: tuple[RenderedEntry, ...]

    def resolved(self) -> tuple[RenderedEntry, ...]:
        return tuple(e for e in self.entries if not e.is_gap)

    def gaps(self) -> tuple[GoldenEntry, ...]:
        return tuple(e.golden for e in self.entries if e.is_gap)

    def compromises(self) -> tuple[tuple[GoldenEntry, Compromise], ...]:
        return tuple((e.golden, c) for e in self.entries for c in e.compromises)


class AlbumResolution(NamedTuple):
    """Result of resolving a golden Album: items (empty on gap), fidelity compromises,
    and — when items is empty — a machine-readable reason for the gap (None otherwise)."""
    items: list[PlatformItem]
    compromises: tuple[Compromise, ...]
    gap_reason: str | None


@runtime_checkable
class Renderer(Protocol):
    def resolve(self, recording: Recording) -> tuple[PlatformItem | None, tuple[Compromise, ...]]: ...
    def resolve_album(self, album: Album, preference: EditionPreference) -> AlbumResolution: ...
    def emit(self, name: str, items: list[PlatformItem]) -> str: ...


def render(
    golden: GoldenPlaylist,
    renderer: Renderer,
    preference: EditionPreference = EditionPolicy.default(),
) -> Rendering:
    """Resolve every admitted golden entry to platform items (or a gap). No writes."""
    rendered = []
    for e in golden.entries:
        if not e.verdict.admitted:
            continue
        if isinstance(e.item, Recording):
            pi, comps = renderer.resolve(e.item)
            items = (pi,) if pi is not None else ()
            rendered.append(RenderedEntry(e, items=items, compromises=comps))
        elif isinstance(e.item, Album):
            effective_preference = e.edition if e.edition is not None else preference
            items_list, comps, gap_reason = renderer.resolve_album(e.item, effective_preference)
            rendered.append(RenderedEntry(e, items=tuple(items_list), compromises=comps,
                                          gap_reason=gap_reason))
        else:
            rendered.append(RenderedEntry(e, items=(), compromises=()))
    return Rendering(golden.name, tuple(rendered))


def publish(rendering: Rendering, renderer: Renderer) -> str:
    """Emit the resolved items to the platform; return the platform playlist reference."""
    items = [item for e in rendering.resolved() for item in e.items]
    if not items:
        raise PlatformError(f"nothing resolved to publish for '{rendering.name}'")
    return renderer.emit(rendering.name, items)
