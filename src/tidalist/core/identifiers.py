"""Strong identifier aliases and the album identity value object for the domain."""

from dataclasses import dataclass
from enum import StrEnum
from typing import NewType

ISRC = NewType("ISRC", str)        # International Standard Recording Code
MBID = NewType("MBID", str)        # MusicBrainz id (recording or release-group)
TrackId = NewType("TrackId", str)  # catalog (Tidal) track id
PlaylistId = NewType("PlaylistId", str)
DiscogsMasterId = NewType("DiscogsMasterId", int)
DiscogsReleaseId = NewType("DiscogsReleaseId", int)


class Source(StrEnum):
    """A metadata source that can assert an album identity."""
    MUSICBRAINZ = "musicbrainz"
    DISCOGS = "discogs"


@dataclass(frozen=True, slots=True)
class ExternalIds:
    """An album's cross-catalog identifiers: MB release-group + Discogs master/release.

    MB and Discogs are complementary peers at the album grain; a Discogs-only id is a full
    identity, not a gap. `sources` records which providers asserted this album.
    """
    mbid: MBID | None = None
    discogs_master_id: DiscogsMasterId | None = None
    discogs_release_id: DiscogsReleaseId | None = None
    sources: frozenset[Source] = frozenset()
