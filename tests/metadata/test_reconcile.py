"""Tests for reconcile_albums: pure album reconciler."""
import pytest

from tidalist.core.album import Album, ReleaseTrait, TrackRef
from tidalist.core.identifiers import (
    DiscogsMasterId,
    DiscogsReleaseId,
    ExternalIds,
    MBID,
    Source,
)
from tidalist.metadata.reconcile import reconcile_albums


def _mb_album(**kwargs) -> Album:
    defaults = dict(
        artist="Steve Winwood",
        title="Arc of a Diver",
        ids=ExternalIds(
            mbid=MBID("mb-uuid-001"),
            sources=frozenset({Source.MUSICBRAINZ}),
        ),
        first_released=1980,
    )
    defaults.update(kwargs)
    return Album(**defaults)


def _dc_album(**kwargs) -> Album:
    defaults = dict(
        artist="Steve Winwood",
        title="Arc of a Diver",
        ids=ExternalIds(
            discogs_master_id=DiscogsMasterId(69017),
            sources=frozenset({Source.DISCOGS}),
        ),
        first_released=1980,
        styles=frozenset({"Rock", "Pop"}),
    )
    defaults.update(kwargs)
    return Album(**defaults)


# (a) FK link: discogs_master_id on both sides matches

def test_fk_link_merges_ids():
    mb = _mb_album(ids=ExternalIds(
        mbid=MBID("mb-uuid-001"),
        discogs_master_id=DiscogsMasterId(69017),
        sources=frozenset({Source.MUSICBRAINZ}),
    ))
    dc = _dc_album()
    result = reconcile_albums([mb], [dc])

    assert len(result) == 1
    merged = result[0]
    assert merged.ids.mbid == MBID("mb-uuid-001")
    assert merged.ids.discogs_master_id == DiscogsMasterId(69017)
    assert Source.MUSICBRAINZ in merged.ids.sources
    assert Source.DISCOGS in merged.ids.sources


def test_fk_link_styles_from_discogs():
    mb = _mb_album(
        ids=ExternalIds(
            mbid=MBID("mb-uuid-001"),
            discogs_master_id=DiscogsMasterId(69017),
            sources=frozenset({Source.MUSICBRAINZ}),
        ),
        styles=frozenset(),
    )
    dc = _dc_album(styles=frozenset({"Rock", "Pop"}))
    result = reconcile_albums([mb], [dc])

    assert result[0].styles == frozenset({"Rock", "Pop"})


def test_fk_link_mb_tracklist_kept():
    track = TrackRef(position=1, title="While You See a Chance")
    mb = _mb_album(
        ids=ExternalIds(
            mbid=MBID("mb-uuid-001"),
            discogs_master_id=DiscogsMasterId(69017),
            sources=frozenset({Source.MUSICBRAINZ}),
        ),
        tracklist=(track,),
    )
    dc = _dc_album(tracklist=(TrackRef(position=1, title="Different Track"),))
    result = reconcile_albums([mb], [dc])

    assert result[0].tracklist == (track,)


def test_fk_link_discogs_tracklist_used_when_mb_empty():
    dc_track = TrackRef(position=1, title="While You See a Chance")
    mb = _mb_album(
        ids=ExternalIds(
            mbid=MBID("mb-uuid-001"),
            discogs_master_id=DiscogsMasterId(69017),
            sources=frozenset({Source.MUSICBRAINZ}),
        ),
        tracklist=(),
    )
    dc = _dc_album(tracklist=(dc_track,))
    result = reconcile_albums([mb], [dc])

    assert result[0].tracklist == (dc_track,)


# (b) artist+title link when no FK

def test_name_link_matches_case_insensitive():
    mb = _mb_album(
        artist="steve winwood",
        title="arc of a diver",
        ids=ExternalIds(mbid=MBID("mb-uuid-001"), sources=frozenset({Source.MUSICBRAINZ})),
    )
    dc = _dc_album(
        artist="Steve Winwood",
        title="Arc of a Diver",
    )
    result = reconcile_albums([mb], [dc])

    assert len(result) == 1
    assert result[0].ids.mbid == MBID("mb-uuid-001")
    assert Source.DISCOGS in result[0].ids.sources


def test_name_link_year_mismatch_no_link():
    mb = _mb_album(
        ids=ExternalIds(mbid=MBID("mb-uuid-001"), sources=frozenset({Source.MUSICBRAINZ})),
        first_released=1980,
    )
    dc = _dc_album(first_released=1985)
    result = reconcile_albums([mb], [dc])

    assert len(result) == 2  # no link, both pass through


def test_name_link_year_none_on_mb_still_links():
    mb = _mb_album(
        ids=ExternalIds(mbid=MBID("mb-uuid-001"), sources=frozenset({Source.MUSICBRAINZ})),
        first_released=None,
    )
    dc = _dc_album(first_released=1980)
    result = reconcile_albums([mb], [dc])

    assert len(result) == 1


def test_name_link_year_none_on_dc_still_links():
    mb = _mb_album(
        ids=ExternalIds(mbid=MBID("mb-uuid-001"), sources=frozenset({Source.MUSICBRAINZ})),
        first_released=1980,
    )
    dc = _dc_album(first_released=None)
    result = reconcile_albums([mb], [dc])

    assert len(result) == 1


def test_name_link_title_mismatch_no_link():
    mb = _mb_album(
        title="Arc of a Diver",
        ids=ExternalIds(mbid=MBID("mb-uuid-001"), sources=frozenset({Source.MUSICBRAINZ})),
    )
    dc = _dc_album(title="Talking Back to the Night")
    result = reconcile_albums([mb], [dc])

    assert len(result) == 2


# (c) MB-only and Discogs-only pass through unchanged

def test_mb_only_passes_through():
    mb = _mb_album()
    result = reconcile_albums([mb], [])

    assert len(result) == 1
    assert result[0] == mb


def test_dc_only_passes_through():
    dc = _dc_album()
    result = reconcile_albums([], [dc])

    assert len(result) == 1
    assert result[0] == dc


def test_mb_first_then_dc_order():
    mb = _mb_album(title="Arc of a Diver")
    dc = _dc_album(title="Talking Back to the Night")  # different title, won't link
    result = reconcile_albums([mb], [dc])

    assert len(result) == 2
    assert result[0] == mb
    assert result[1] == dc


# (d) first_released: MB preferred, Discogs fills when MB None

def test_first_released_mb_preferred():
    mb = _mb_album(first_released=1980)
    dc = _dc_album(
        ids=ExternalIds(
            discogs_master_id=DiscogsMasterId(69017),
            sources=frozenset({Source.DISCOGS}),
        ),
        first_released=1979,
    )
    mb = _mb_album(
        ids=ExternalIds(
            mbid=MBID("mb-uuid-001"),
            discogs_master_id=DiscogsMasterId(69017),
            sources=frozenset({Source.MUSICBRAINZ}),
        ),
        first_released=1980,
    )
    result = reconcile_albums([mb], [dc])

    assert result[0].first_released == 1980


def test_first_released_discogs_fills_when_mb_none():
    mb = _mb_album(
        ids=ExternalIds(
            mbid=MBID("mb-uuid-001"),
            discogs_master_id=DiscogsMasterId(69017),
            sources=frozenset({Source.MUSICBRAINZ}),
        ),
        first_released=None,
    )
    dc = _dc_album(first_released=1980)
    result = reconcile_albums([mb], [dc])

    assert result[0].first_released == 1980


# Each Discogs album links to at most one MB album

def test_discogs_album_links_to_at_most_one_mb():
    mb1 = _mb_album(
        ids=ExternalIds(mbid=MBID("mb-uuid-001"), sources=frozenset({Source.MUSICBRAINZ})),
    )
    mb2 = _mb_album(
        ids=ExternalIds(mbid=MBID("mb-uuid-002"), sources=frozenset({Source.MUSICBRAINZ})),
    )
    dc = _dc_album()
    result = reconcile_albums([mb1, mb2], [dc])

    # dc links to mb1 (first match); mb2 passes through; dc is consumed
    assert len(result) == 2
    merged_ids = {r.ids.mbid for r in result}
    assert MBID("mb-uuid-001") in merged_ids
    assert MBID("mb-uuid-002") in merged_ids


def test_mb_artist_kept_in_merged():
    mb = _mb_album(
        artist="Steve Winwood",
        ids=ExternalIds(
            mbid=MBID("mb-uuid-001"),
            discogs_master_id=DiscogsMasterId(69017),
            sources=frozenset({Source.MUSICBRAINZ}),
        ),
    )
    dc = _dc_album(artist="steve winwood")
    result = reconcile_albums([mb], [dc])

    assert result[0].artist == "Steve Winwood"


def test_mb_traits_kept_in_merged():
    mb = _mb_album(
        ids=ExternalIds(
            mbid=MBID("mb-uuid-001"),
            discogs_master_id=DiscogsMasterId(69017),
            sources=frozenset({Source.MUSICBRAINZ}),
        ),
        traits=frozenset({ReleaseTrait.LIVE}),
    )
    dc = _dc_album()
    result = reconcile_albums([mb], [dc])

    assert result[0].traits == frozenset({ReleaseTrait.LIVE})


def test_styles_union_of_both():
    mb = _mb_album(
        ids=ExternalIds(
            mbid=MBID("mb-uuid-001"),
            discogs_master_id=DiscogsMasterId(69017),
            sources=frozenset({Source.MUSICBRAINZ}),
        ),
        styles=frozenset({"Prog Rock"}),
    )
    dc = _dc_album(styles=frozenset({"Rock", "Pop"}))
    result = reconcile_albums([mb], [dc])

    assert result[0].styles == frozenset({"Rock", "Pop", "Prog Rock"})


def test_discogs_release_id_preserved_from_mb():
    mb = _mb_album(
        ids=ExternalIds(
            mbid=MBID("mb-uuid-001"),
            discogs_master_id=DiscogsMasterId(69017),
            discogs_release_id=DiscogsReleaseId(123456),
            sources=frozenset({Source.MUSICBRAINZ}),
        ),
    )
    dc = _dc_album()
    result = reconcile_albums([mb], [dc])

    assert result[0].ids.discogs_release_id == DiscogsReleaseId(123456)
