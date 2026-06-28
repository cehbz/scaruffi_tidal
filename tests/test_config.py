from pathlib import Path

from tidalist.config import AppConfig


def _write(tmp_path, text):
    p = tmp_path / "config.yaml"
    p.write_text(text)
    return p


def test_session_file_lives_in_config_dir(tmp_path):
    p = _write(tmp_path, "discogs:\n  token: X\n")
    assert AppConfig.load(p).session_file == tmp_path / "tidal_session.json"


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
