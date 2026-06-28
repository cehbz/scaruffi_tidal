"""Live integration proof: Trout Mask Replica assembles from individual tracks.

Captain Beefheart's *Trout Mask Replica* is absent on Tidal as a standalone album,
but many of its tracks appear on compilations.  resolve_album() should fall through
to _assemble_from_tracks(), yielding a partial set of PlatformItems and an
'album-source' Compromise rather than an empty (gap) result.

Artist spelling: MusicBrainz indexes this album under "Captain Beefheart & His Magic Band"
(not the shorter "Captain Beefheart").  The test tries the short form first; if that
returns no TMR result it falls back to the full band name.
"""

import pytest
from pathlib import Path

from tidalist.config import AppConfig
from tidalist.core.recording import Candidate, Kind
from tidalist.core.edition import EditionPolicy

_TMR_SHORT = Candidate("Captain Beefheart", "Trout Mask Replica", kind=Kind.ALBUM)
_TMR_FULL = Candidate("Captain Beefheart & His Magic Band", "Trout Mask Replica", kind=Kind.ALBUM)


@pytest.mark.integration
def test_trout_mask_replica_assembles_from_compilations_live():
    cfg = AppConfig.load()
    if not Path(cfg.musicbrainz_db).exists():
        pytest.skip("mirror not mounted")
    if not cfg.session_file.exists():
        pytest.skip("no Tidal session cached")

    from tidalist.metadata.mirror import MirrorDB
    from tidalist.metadata.mb_mirror import MusicBrainzMetadata
    from tidalist.tidal.session import authenticate
    from tidalist.tidal.platform import TidalPlatform
    from tidalist.realize.tidal import TidalRealizer

    provider = MusicBrainzMetadata(MirrorDB(cfg.musicbrainz_db, cfg.discogs_db))

    # "Captain Beefheart" alone doesn't surface TMR in MusicBrainz; the correct
    # credited artist is "Captain Beefheart & His Magic Band".
    album = None
    for candidate in (_TMR_SHORT, _TMR_FULL):
        albums = provider.albums_for(candidate)
        album = next(
            (a for a in albums
             if a.tracklist and "trout mask replica" in a.title.casefold()),
            None,
        )
        if album is not None:
            break

    assert album is not None, (
        "MusicBrainz returned no release-group with a tracklist for Trout Mask Replica "
        "(tried both 'Captain Beefheart' and 'Captain Beefheart & His Magic Band')"
    )

    realizer = TidalRealizer(TidalPlatform(authenticate(cfg.session_file)))
    items, comps = realizer.resolve_album(album, EditionPolicy.default())

    # Must be a partial (or full) assembly — NOT a gap.
    assert items, (
        f"expected partial assembly but got an empty result "
        f"(album='{album.title}', tracklist={len(album.tracklist)} tracks)"
    )

    # The album-source compromise must be present, confirming fallback path was used.
    assert any(c.facet == "album-source" for c in comps), (
        f"expected 'album-source' compromise; got: {[c.facet for c in comps]}"
    )
