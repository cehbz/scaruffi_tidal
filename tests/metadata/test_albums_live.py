import pytest

from tidalist.config import AppConfig
from tidalist.core.recording import Candidate, Kind

_JBMD = Candidate("Traffic", "John Barleycorn Must Die", kind=Kind.ALBUM)


@pytest.mark.integration
def test_musicbrainz_albums_for_discovers_the_release_group_live():
    cfg = AppConfig.load()
    if not cfg.musicbrainz_contact:
        pytest.skip("no musicbrainz.contact configured")
    import musicbrainzngs
    from tidalist.metadata.musicbrainz import MusicBrainzMetadata
    musicbrainzngs.set_useragent("tidalist", "1.0", cfg.musicbrainz_contact)
    albums = MusicBrainzMetadata(musicbrainzngs).albums_for(_JBMD)
    assert albums, "expected release-groups for the album"
    top = albums[0]
    assert top.title == "John Barleycorn Must Die"
    assert top.ids.mbid and top.first_released == 1970
    assert all("Traffic Sound" not in a.artist for a in albums)   # identity filter holds


@pytest.mark.integration
def test_discogs_albums_for_returns_masters_live():
    cfg = AppConfig.load()
    if not cfg.discogs_token:
        pytest.skip("no discogs token configured")
    import discogs_client
    from tidalist.metadata.discogs import DiscogsMetadata
    client = discogs_client.Client("tidalist/1.0", user_token=cfg.discogs_token)
    albums = DiscogsMetadata(client).albums_for(_JBMD)
    assert albums and all(a.ids.mbid is None for a in albums)   # bounded, no hang; Discogs has no MBID
