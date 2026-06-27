"""Pure album reconciler: merges MB and Discogs album lists."""

from ..core.album import Album
from ..core.identifiers import ExternalIds


def reconcile_albums(mb: list[Album], discogs: list[Album]) -> list[Album]:
    """Merge MB and Discogs album lists into a single deduplicated list.

    For each MB album, find at most one Discogs album to merge with, preferring
    a foreign-key link (matching discogs_master_id) over a name/year link.
    Unlinked albums from either source pass through unchanged.

    Order: MB-derived albums first, then unlinked Discogs-only albums.
    """
    unlinked_dc: set[int] = set(range(len(discogs)))
    merged: list[Album] = []

    for mb_album in mb:
        dc_match_idx: int | None = _find_discogs_match(mb_album, discogs, unlinked_dc)

        if dc_match_idx is None:
            merged.append(mb_album)
        else:
            unlinked_dc.discard(dc_match_idx)
            merged.append(_merge(mb_album, discogs[dc_match_idx]))

    for idx in sorted(unlinked_dc):
        merged.append(discogs[idx])

    return merged


def _find_discogs_match(
    mb_album: Album,
    discogs: list[Album],
    available: set[int],
) -> int | None:
    """Return the index of the best Discogs album to merge with mb_album, or None."""
    # Pass 1: FK link (both have matching discogs_master_id)
    if mb_album.ids.discogs_master_id is not None:
        for idx in range(len(discogs)):
            if idx not in available:
                continue
            dc = discogs[idx]
            if dc.ids.discogs_master_id == mb_album.ids.discogs_master_id:
                return idx

    # Pass 2: name + year link
    for idx in range(len(discogs)):
        if idx not in available:
            continue
        dc = discogs[idx]
        if _name_year_match(mb_album, dc):
            return idx

    return None


def _name_year_match(mb: Album, dc: Album) -> bool:
    artists_match = mb.artist.casefold() == dc.artist.casefold()
    titles_match = mb.title.casefold() == dc.title.casefold()
    years_agree = (
        mb.first_released is None
        or dc.first_released is None
        or mb.first_released == dc.first_released
    )
    return artists_match and titles_match and years_agree


def _merge(mb: Album, dc: Album) -> Album:
    """Produce one merged Album from an MB album and a linked Discogs album."""
    merged_ids = ExternalIds(
        mbid=mb.ids.mbid,
        discogs_master_id=dc.ids.discogs_master_id or mb.ids.discogs_master_id,
        discogs_release_id=mb.ids.discogs_release_id,
        sources=mb.ids.sources | dc.ids.sources,
    )
    return Album(
        artist=mb.artist,
        title=mb.title,
        ids=merged_ids,
        first_released=mb.first_released if mb.first_released is not None else dc.first_released,
        traits=mb.traits,
        styles=dc.styles | mb.styles,
        tracklist=mb.tracklist if mb.tracklist else dc.tracklist,
    )
