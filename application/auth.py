"""
Application layer for Tidal authentication.
Following SOLID principles and Clean Architecture.
"""

import json
from tidalapi.session import Session
import yaml
from datetime import datetime, timedelta
from dataclasses import dataclass
from pathlib import Path
from typing import Optional, Dict, Any
from abc import ABC, abstractmethod

from domain.auth import (
    SessionToken, OAuthCredentials, CountryCode,
    SessionTokenAuthentication, OAuthAuthentication,
    AuthenticationService, AuthenticationError,
    AuthenticationType, AuthenticationStrategy
)


class ConfigurationRepository(ABC):
    """Repository interface for configuration (Dependency Inversion Principle)"""
    
    @abstractmethod
    def load(self) -> Dict[str, Any]:
        """Load configuration from storage"""
        pass
    
    @abstractmethod
    def save(self, config: Dict[str, Any]) -> None:
        """Save configuration to storage"""
        pass


class YamlConfigurationRepository(ConfigurationRepository):
    """YAML file-based configuration repository"""
    
    def __init__(self, config_path: Path):
        self.config_path = config_path
        self._ensure_config_dir()
    
    def _ensure_config_dir(self):
        """Ensure configuration directory exists"""
        self.config_path.parent.mkdir(parents=True, exist_ok=True)
    
    def load(self) -> Dict[str, Any]:
        """Load configuration from YAML file"""
        if not self.config_path.exists():
            return {}
        
        with open(self.config_path, 'r') as f:
            return yaml.safe_load(f) or {}
    
    def save(self, config: Dict[str, Any]) -> None:
        """Save configuration to YAML file"""
        with open(self.config_path, 'w') as f:
            yaml.safe_dump(config, f, default_flow_style=False)


@dataclass
class TidalConfiguration:
    """Application configuration value object"""
    session_token: Optional[str] = None
    oauth_access_token: Optional[str] = None
    oauth_refresh_token: Optional[str] = None
    oauth_expires_at: Optional[str] = None
    country_code: str = "US"
    match_threshold: float = 0.5
    user_id: Optional[int] = None
    
    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> 'TidalConfiguration':
        """Create configuration from dictionary"""
        config = cls(
            match_threshold=data.get('match_threshold', 0.5),
            country_code=data.get('country_code', 'US')
        )
        
        # Load session config
        if 'session' in data:
            config.session_token = data['session'].get('token')
            config.user_id = data['session'].get('user_id')
            config.country_code = data['session'].get('country_code', 'US')
        
        # Load OAuth config (takes precedence)
        if 'oauth' in data:
            config.oauth_access_token = data['oauth'].get('access_token')
            config.oauth_refresh_token = data['oauth'].get('refresh_token')
            config.oauth_expires_at = data['oauth'].get('expires_at')
            if 'user_id' in data['oauth']:
                config.user_id = data['oauth']['user_id']
        
        # Legacy format support
        if 'tidal_session_token' in data:
            config.session_token = data['tidal_session_token']
        
        return config
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert configuration to dictionary"""
        result: Dict[str, Any] = {'match_threshold': self.match_threshold}
        
        if self.session_token:
            session: dict[str, Any] = {
                'token': self.session_token,
                'country_code': self.country_code,
                'last_updated': datetime.now().isoformat()
            }
            if self.user_id:
                session['user_id'] = self.user_id
            result['session'] = session
        if self.oauth_access_token:
            oauth: dict[str, Any] = {
                'access_token': self.oauth_access_token,
                'token_type': 'Bearer',
                'last_updated': datetime.now().isoformat()
            }
            if self.oauth_refresh_token:
                result['oauth']['refresh_token'] = self.oauth_refresh_token
            if self.oauth_expires_at:
                result['oauth']['expires_at'] = self.oauth_expires_at
            if self.user_id:
                result['oauth']['user_id'] = self.user_id
        
        return result


class AuthenticationStrategyFactory:
    """Factory for creating authentication strategies (Factory Pattern)"""
    
    @staticmethod
    def create_from_config(config: TidalConfiguration) -> AuthenticationStrategy:
        """Create appropriate authentication strategy based on configuration"""
        
        # Prefer OAuth if available
        if config.oauth_access_token:
            credentials = OAuthCredentials(
                access_token=config.oauth_access_token,
                refresh_token=config.oauth_refresh_token,
                expires_at=config.oauth_expires_at
            )
            return OAuthAuthentication(credentials)
        
        # Fall back to session token
        if config.session_token:
            token = SessionToken(config.session_token)
            country = CountryCode(config.country_code)
            return SessionTokenAuthentication(token, country)
        
        raise ValueError("No authentication credentials found in configuration.")


class TidalAuthenticationApplication:
    """Application service for Tidal authentication (Single Responsibility Principle)"""
    
    def __init__(self, config_repository: YamlConfigurationRepository):
        self.config_repository = config_repository
        self.config = self._load_configuration()
        self.session = None
    
    def _load_configuration(self) -> TidalConfiguration:
        """Load configuration from repository"""
        config_dict = self.config_repository.load()
        return TidalConfiguration.from_dict(config_dict)
    
    def authenticate(self, session: Session) -> bool:
        """Authenticate with Tidal using configured credentials"""
        self.session = session
        
        # Try session file first
        if self._try_session_file():
            return True
        
        # Try configured credentials
        try:
            strategy = AuthenticationStrategyFactory.create_from_config(self.config)
            auth_service = AuthenticationService(strategy)
            return auth_service.authenticate(session)
        except ValueError as e:
            print(f"Configuration error: {e}")
            return False
        except AuthenticationError as e:
            print(f"Authentication error: {e}")
            return False
    
    def _try_session_file(self) -> bool:
        """Try to load session from file"""
        session_file = self.config_repository.config_path.parent / 'session.json'
        if session_file.exists():
            try:
                if self.session and self.session.load_session_from_file(session_file):
                    print("✓ Loaded session from file")
                    return True
            except:
                pass
        return False
    
    def save_session(self) -> None:
        """Save current session to file"""
        if not self.session:
            return
        
        session_file = self.config_repository.config_path.parent / 'session.json'
        
        try:
            # Try built-in save method first
            if hasattr(self.session, 'save_session_to_file'):
                self.session.save_session_to_file(session_file)
                print(f"✓ Session saved to {session_file}")
            else:
                # Manual save as fallback
                session_data = {}
                
                # Extract whatever data we can
                for attr in ['session_id', 'country_code', '_access_token', '_refresh_token']:
                    if hasattr(self.session, attr):
                        value = getattr(self.session, attr)
                        if value:
                            session_data[attr.lstrip('_')] = value
                
                if self.session.user:
                    session_data['user_id'] = self.session.user.id
                
                if session_data:
                    with open(session_file, 'w') as f:
                        json.dump(session_data, f, indent=2)
                    print(f"✓ Session data saved to {session_file}")
        except Exception as e:
            print(f"Warning: Could not save session: {e}")
    
    def update_oauth_tokens(self, access_token: str, 
                           refresh_token: Optional[str] = None,
                           expires_in: int = 3600) -> None:
        """Update OAuth tokens in configuration"""
        self.config.oauth_access_token = access_token
        self.config.oauth_refresh_token = refresh_token
        self.config.oauth_expires_at = (
            datetime.now() + timedelta(seconds=expires_in)
        ).isoformat()
        self.config.session_token = None  # Clear session if setting OAuth
        self._save_configuration()
        self.save_session()
    
    def update_session_token(self, token: str) -> None:
        """Update session token in configuration"""
        self.config.session_token = token
        self.config.oauth_access_token = None  # Clear OAuth if setting session
        self.config.oauth_refresh_token = None
        self.config.oauth_expires_at = None
        self._save_configuration()
    
    def _save_configuration(self) -> None:
        """Save configuration to repository"""
        config_dict = self.config.to_dict()
        self.config_repository.save(config_dict)


def get_default_config_path() -> Path:
    """Get default configuration file path"""
    config_dir = Path.home() / '.config' / 'scaruffi-tidal'
    return config_dir / 'config.yaml'


def create_application(config_path: Optional[Path] = None) -> TidalAuthenticationApplication:
    """Factory function to create application with dependencies (Dependency Injection)"""
    if config_path is None:
        config_path = get_default_config_path()
    
    config_repository = YamlConfigurationRepository(config_path)
    return TidalAuthenticationApplication(config_repository)
