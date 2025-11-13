"""
Configuration management for Scaruffi-Tidal application.
Infrastructure layer - handles config file I/O.
"""

import os
from dataclasses import dataclass
from pathlib import Path
from typing import Optional
import yaml


@dataclass
class AppConfiguration:
    """
    Application configuration value object.
    
    Contains credentials and settings for all external services.
    """
    # Tidal credentials
    tidal_session_token: Optional[str] = None
    tidal_oauth_token: Optional[str] = None
    
    # Discogs credentials
    discogs_token: Optional[str] = None
    discogs_rate_limit: int = 60  # requests per minute
    
    # Matching thresholds
    match_threshold: float = 0.5
    
    def has_tidal_credentials(self) -> bool:
        """Check if Tidal credentials are configured."""
        return bool(self.tidal_session_token or self.tidal_oauth_token)
    
    def has_discogs_credentials(self) -> bool:
        """Check if Discogs credentials are configured."""
        return bool(self.discogs_token)


class ConfigManager:
    """
    Manages application configuration with XDG compliance.
    
    Configuration file location follows XDG Base Directory specification:
    - Default: ~/.config/scaruffi-tidal/config.yaml
    - Override with XDG_CONFIG_HOME environment variable
    """
    
    def __init__(self, config_path: Optional[Path] = None):
        """
        Initialize config manager.
        
        Args:
            config_path: Optional custom path. If None, uses XDG default.
        """
        if config_path:
            self.config_path = config_path
        else:
            self.config_path = self._get_default_config_path()
        
        # Ensure directory exists
        self.config_path.parent.mkdir(parents=True, exist_ok=True)
    
    def _get_default_config_path(self) -> Path:
        """Get XDG-compliant default config path."""
        xdg_config_home = os.environ.get('XDG_CONFIG_HOME')
        
        if xdg_config_home:
            base_dir = Path(xdg_config_home)
        else:
            base_dir = Path.home() / '.config'
        
        return base_dir / 'scaruffi-tidal' / 'config.yaml'
    
    def load(self) -> AppConfiguration:
        """
        Load configuration from file.
        
        Returns default configuration if file doesn't exist.
        """
        if not self.config_path.exists():
            return AppConfiguration()
        
        with open(self.config_path, 'r') as f:
            data = yaml.safe_load(f) or {}
        
        return self._parse_config(data)
    
    def _parse_config(self, data: dict) -> AppConfiguration:
        """Parse YAML data into AppConfiguration."""
        config = AppConfiguration()
        
        # Load Tidal config
        if 'tidal' in data:
            tidal = data['tidal']
            config.tidal_session_token = tidal.get('session_token')
            config.tidal_oauth_token = tidal.get('oauth_token')
        
        # Load Discogs config
        if 'discogs' in data:
            discogs = data['discogs']
            config.discogs_token = discogs.get('token')
            config.discogs_rate_limit = discogs.get('rate_limit', 60)
        
        # Load matching config
        if 'matching' in data:
            matching = data['matching']
            config.match_threshold = matching.get('threshold', 0.5)
        
        return config
    
    def save(self, config: AppConfiguration) -> None:
        """
        Save configuration to file.
        
        Args:
            config: Configuration to save
        """
        data = {}
        
        # Save Tidal config
        tidal = {}
        if config.tidal_session_token:
            tidal['session_token'] = config.tidal_session_token
        if config.tidal_oauth_token:
            tidal['oauth_token'] = config.tidal_oauth_token
        if tidal:
            data['tidal'] = tidal
        
        # Save Discogs config
        discogs = {}
        if config.discogs_token:
            discogs['token'] = config.discogs_token
        if config.discogs_rate_limit != 60:
            discogs['rate_limit'] = config.discogs_rate_limit
        if discogs:
            data['discogs'] = discogs
        
        # Save matching config
        if config.match_threshold != 0.5:
            data['matching'] = {'threshold': config.match_threshold}
        
        with open(self.config_path, 'w') as f:
            yaml.safe_dump(data, f, default_flow_style=False)
