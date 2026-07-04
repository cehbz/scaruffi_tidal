"""End-to-end live proof of edition selection by distance from the golden.

The golden carries MusicBrainz's canonical tracklist (the ~10-track standard
"Mr. Fantasy"); Tidal album search returns only the 22-track mono+stereo deluxe,
but the renderer enumerates every edition via the artist discography and picks the
one nearest the golden tracklist — the original. This is the exact case surfaced
during review (tidal.com/album/639224, the 10-track edition, vs the 22-track deluxe).
"""
import pytest
from pathlib import Path

from tidalist.config import AppConfig
from tidalist.core.recording import Candidate, Kind
from tidalist.core.edition import EditionPolicy

_MR_FANTASY = Candidate("Traffic", "Mr. Fantasy", kind=Kind.ALBUM)


@pytest.mark.integration
def test_mr_fantasy_resolves_to_the_original_not_the_deluxe_live():
    cfg = AppConfig.load()
    if not Path(cfg.musicbrainz_db).exists():
        pytest.skip("mirror not mounted")
    if not cfg.session_file.exists():
        pytest.skip("no Tidal session cached")

    from tidalist.metadata.mirror import MirrorDB
    from tidalist.metadata.mb_mirror import MusicBrainzMetadata
    from tidalist.tidal.session import authenticate
    from tidalist.tidal.platform import TidalPlatform
    from tidalist.render.tidal import TidalRenderer

    # Curate: the golden Album gains MB's canonical tracklist (the standard edition,
    # NOT the 22-track deluxe outlier).
    albums = MusicBrainzMetadata(MirrorDB(cfg.musicbrainz_db, cfg.discogs_db)).albums_for(_MR_FANTASY)
    assert albums, "expected release-groups for Mr. Fantasy"
    album = next((a for a in albums if a.title == "Mr. Fantasy"), albums[0])
    assert 8 <= len(album.tracklist) <= 14, \
        f"canonical tracklist should be the ~10-track standard, got {len(album.tracklist)}"

    # Render: distance-from-golden must pick the original over the 22-track deluxe
    # that Tidal album search surfaces first.
    renderer = TidalRenderer(TidalPlatform(authenticate(cfg.session_file)))
    items, _, _ = renderer.resolve_album(album, EditionPolicy.default())
    assert items, "expected the album to resolve"
    assert len(items) <= 14, \
        f"resolved {len(items)} tracks — picked the deluxe, not the original"
    titles = " | ".join(i.title.casefold() for i in items)
    assert "fantasy" in titles  # "Dear Mr. Fantasy" is on the original tracklist
