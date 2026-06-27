from pathlib import Path

from tidalist.config import AppConfig, default_cache_path


def _write(tmp_path, text):
    p = tmp_path / "config.yaml"
    p.write_text(text)
    return p


def test_loads_discogs_and_musicbrainz(tmp_path):
    p = _write(tmp_path, "discogs:\n  token: TOK\n  rate_limit: 30\n"
                         "musicbrainz:\n  contact: me@example.com\n")
    cfg = AppConfig.load(p)
    assert cfg.discogs_token == "TOK"
    assert cfg.discogs_rate_limit == 30
    assert cfg.musicbrainz_contact == "me@example.com"


def test_defaults_when_file_missing(tmp_path):
    cfg = AppConfig.load(tmp_path / "nope.yaml")
    assert cfg.discogs_token is None
    assert cfg.discogs_rate_limit == 60
    assert cfg.musicbrainz_contact is None


def test_session_file_lives_in_config_dir(tmp_path):
    p = _write(tmp_path, "discogs:\n  token: X\n")
    assert AppConfig.load(p).session_file == tmp_path / "tidal_session.json"


def test_defaults_when_sections_absent(tmp_path):
    cfg = AppConfig.load(_write(tmp_path, "{}\n"))
    assert cfg.discogs_token is None and cfg.discogs_rate_limit == 60


def test_cache_path_respects_xdg_cache_home(monkeypatch):
    monkeypatch.setenv("XDG_CACHE_HOME", "/var/cache-x")
    assert default_cache_path() == Path("/var/cache-x") / "tidalist"


def test_cache_path_falls_back_to_dot_cache(monkeypatch):
    monkeypatch.delenv("XDG_CACHE_HOME", raising=False)
    assert default_cache_path() == Path.home() / ".cache" / "tidalist"


def test_mb_cache_dir_is_under_cache_path(monkeypatch, tmp_path):
    monkeypatch.setenv("XDG_CACHE_HOME", str(tmp_path))
    cfg = AppConfig.load(_write(tmp_path, "{}\n"))
    assert cfg.mb_cache_dir == tmp_path / "tidalist" / "mb"


def test_mirrors_block_sets_db_paths(tmp_path):
    p = _write(tmp_path, "mirrors:\n"
                         "  musicbrainz_db: /custom/mb.db\n"
                         "  discogs_db: /custom/dc.db\n")
    cfg = AppConfig.load(p)
    assert cfg.musicbrainz_db == "/custom/mb.db"
    assert cfg.discogs_db == "/custom/dc.db"


def test_mirrors_defaults_when_absent(tmp_path):
    from tidalist.config import DEFAULT_MUSICBRAINZ_DB, DEFAULT_DISCOGS_DB
    cfg = AppConfig.load(_write(tmp_path, "{}\n"))
    assert cfg.musicbrainz_db == DEFAULT_MUSICBRAINZ_DB
    assert cfg.discogs_db == DEFAULT_DISCOGS_DB


def test_mirrors_defaults_when_file_missing(tmp_path):
    from tidalist.config import DEFAULT_MUSICBRAINZ_DB, DEFAULT_DISCOGS_DB
    cfg = AppConfig.load(tmp_path / "nope.yaml")
    assert cfg.musicbrainz_db == DEFAULT_MUSICBRAINZ_DB
    assert cfg.discogs_db == DEFAULT_DISCOGS_DB
