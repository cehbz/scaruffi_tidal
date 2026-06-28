"""Single application configuration, loaded from one XDG YAML file."""

import os
from dataclasses import dataclass
from pathlib import Path

import yaml


DEFAULT_MUSICBRAINZ_DB = "/Volumes/Crucial X10/musicbrainz/musicbrainz.db"
DEFAULT_DISCOGS_DB = "/Volumes/Crucial X10/discogs/discogs.db"


@dataclass(frozen=True)
class AppConfig:
    config_dir: Path
    musicbrainz_db: str = DEFAULT_MUSICBRAINZ_DB
    discogs_db: str = DEFAULT_DISCOGS_DB

    @property
    def session_file(self) -> Path:
        return self.config_dir / "tidal_session.json"

    @classmethod
    def load(cls, path: Path | None = None) -> "AppConfig":
        path = path or default_config_path()
        data = {}
        if path.exists():
            data = yaml.safe_load(path.read_text()) or {}
        mirrors = data.get("mirrors") or {}
        return cls(
            config_dir=path.parent,
            musicbrainz_db=mirrors.get("musicbrainz_db", DEFAULT_MUSICBRAINZ_DB),
            discogs_db=mirrors.get("discogs_db", DEFAULT_DISCOGS_DB),
        )


def default_config_path() -> Path:
    base = os.environ.get("XDG_CONFIG_HOME")
    base = Path(base) if base else Path.home() / ".config"
    return base / "tidalist" / "config.yaml"
