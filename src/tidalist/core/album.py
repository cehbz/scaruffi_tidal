"""Album: release-group identity value object."""

from dataclasses import dataclass
from enum import StrEnum

from .identifiers import ISRC, MBID, ExternalIds


class ReleaseTrait(StrEnum):
    """A release-group classification a curation criterion can filter on (extend when a new filter is added)."""
    COMPILATION = "compilation"
    LIVE = "live"


@dataclass(frozen=True, slots=True)
class TrackRef:
    """One track of an album's canonical tracklist: ordered identity for edition distance."""
    position: int
    title: str
    isrc: ISRC | None = None
    mbid: MBID | None = None
    duration_s: int | None = None


@dataclass(frozen=True, slots=True)
class Album:
    """Release-group identity + release traits + canonical tracklist."""
    artist: str
    title: str
    ids: ExternalIds = ExternalIds()
    first_released: int | None = None
    traits: frozenset[ReleaseTrait] = frozenset()
    styles: frozenset[str] = frozenset()
    tracklist: tuple[TrackRef, ...] = ()
