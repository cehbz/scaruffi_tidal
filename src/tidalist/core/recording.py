"""Recording: a performance's identity. Candidate: a request to find one."""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum
from typing import TYPE_CHECKING

from .identifiers import ISRC, MBID

if TYPE_CHECKING:
    from .criteria import Criterion
    from .edition import EditionPreference


class Performance(StrEnum):
    """How a recording was made (a live take is a distinct ISRC)."""
    STUDIO = "studio"
    LIVE = "live"
    SESSION = "session"
    DEMO = "demo"
    UNKNOWN = "unknown"


class Kind(StrEnum):
    """Whether a candidate represents a whole album or a single track."""
    ALBUM = "album"
    TRACK = "track"


@dataclass(frozen=True, slots=True)
class Credit:
    artist: str
    role: str  # "performer", "vocals", "guitar", "composer", ...


@dataclass(frozen=True, slots=True)
class Recording:
    """A specific performance. Identity: mbid (primary) + isrc (rendering bridge);
    artist/title/album/first_released/duration_s for human display and fuzzy fallback."""
    artist: str
    title: str
    mbid: MBID | None = None
    isrc: ISRC | None = None
    album: str | None = None
    first_released: int | None = None
    duration_s: int | None = None
    performance: Performance = Performance.UNKNOWN
    credits: tuple[Credit, ...] = ()

    def is_live(self) -> bool:
        return self.performance is Performance.LIVE

    def performs(self, artist: str) -> bool:
        """Whether `artist` is among the performer credits (so: not a cover)."""
        a = artist.casefold()
        return any(a in c.artist.casefold() or c.artist.casefold() in a
                   for c in self.credits if c.role == "performer")


@dataclass(frozen=True, slots=True)
class Candidate:
    """A described item to find in the catalog: one track, or a whole album."""
    artist: str
    title: str
    album: str | None = None
    year: int | None = None
    isrc: ISRC | None = None
    kind: Kind = Kind.TRACK
    criteria: tuple[Criterion, ...] = ()
    edition: EditionPreference | None = None
    artist_mbid: MBID | None = None

    def __post_init__(self):
        if not self.artist.strip() or not self.title.strip():
            raise ValueError("Candidate requires a non-empty artist and title")

    def search_query(self) -> str:
        # Broad: artist + title. album is a resolver pin, not a search term.
        return f"{self.artist} {self.title}".strip()
