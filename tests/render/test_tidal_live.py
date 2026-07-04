import pytest

from tidalist.config import AppConfig
from tidalist.core.album import Album
from tidalist.core.edition import EditionPolicy


def _renderer_or_skip():
    cfg = AppConfig.load()
    if not cfg.session_file.exists():
        pytest.skip("no Tidal session cached")
    from tidalist.tidal.session import authenticate
    from tidalist.tidal.platform import TidalPlatform
    from tidalist.render.tidal import TidalRenderer
    return TidalRenderer(TidalPlatform(authenticate(cfg.session_file)))


@pytest.mark.integration
def test_resolve_album_picks_original_edition_live():
    renderer = _renderer_or_skip()
    items, _, _ = renderer.resolve_album(
        Album(artist="Traffic", title="John Barleycorn Must Die"), EditionPolicy.default())
    assert items, "expected album tracks"
    assert any("Glad" in i.title for i in items)
    assert len(items) <= 12   # the 8-track original, not the 16-track Deluxe Edition


@pytest.mark.integration
def test_resolve_album_empty_for_nonexistent_album_live():
    renderer = _renderer_or_skip()
    items, _, _ = renderer.resolve_album(
        Album(artist="Zzqx Nonexistent Band", title="No Such Album Qwerty Plugh"),
        EditionPolicy.default())
    assert items == []
