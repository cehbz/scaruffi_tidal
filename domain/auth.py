"""
Domain models for Tidal authentication.
Following Domain-Driven Design principles.
"""

from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import Optional, Protocol
from enum import Enum, auto

from tidalapi.session import Session


class AuthenticationType(Enum):
    """Value Object: Types of authentication methods supported"""
    SESSION = auto()
    OAUTH = auto()
    PKCE = auto()
    SIMPLE_OAUTH = auto()


@dataclass(frozen=True)
class SessionToken:
    """Value Object: Represents a Tidal session token"""
    value: str
    
    def __post_init__(self):
        if not self.value or not isinstance(self.value, str):
            raise ValueError("Session token must be a non-empty string")
        if len(self.value) != 36:  # UUID format
            raise ValueError("Session token must be a valid UUID")


@dataclass(frozen=True)
class OAuthCredentials:
    """Value Object: OAuth authentication credentials"""
    access_token: str
    token_type: str = "Bearer"
    refresh_token: Optional[str] = None
    expires_at: Optional[str] = None
    
    def __post_init__(self):
        if not self.access_token:
            raise ValueError("Access token is required for OAuth authentication")


@dataclass(frozen=True)
class CountryCode:
    """Value Object: ISO country code"""
    code: str
    
    def __post_init__(self):
        if not self.code or len(self.code) != 2:
            raise ValueError("Country code must be a 2-letter ISO code")


class AuthenticationStrategy(ABC):
    """Strategy pattern for different authentication methods"""
    
    @abstractmethod
    def authenticate(self, session: Session) -> bool:
        """Authenticate with Tidal using specific strategy"""
        pass
    
    @abstractmethod
    def get_type(self) -> AuthenticationType:
        """Return the authentication type"""
        pass


class SessionTokenAuthentication(AuthenticationStrategy):
    """Concrete strategy for session token authentication"""
    
    def __init__(self, token: SessionToken, country_code: Optional[CountryCode] = None):
        self.token = token
        self.country_code = country_code or CountryCode("US")
    
    def authenticate(self, session: Session) -> bool:
        """Authenticate using session token"""
        return session.load_session(
            session_id=self.token.value,
            country_code=self.country_code.code
        )
    
    def get_type(self) -> AuthenticationType:
        return AuthenticationType.SESSION


class OAuthAuthentication(AuthenticationStrategy):
    """Concrete strategy for OAuth authentication"""
    
    def __init__(self, credentials: OAuthCredentials):
        self.credentials = credentials
    
    def authenticate(self, session: Session) -> bool:
        """Authenticate using OAuth credentials"""
        from datetime import datetime
        
        expiry_time = None
        if self.credentials.expires_at:
            expiry_time = datetime.fromisoformat(self.credentials.expires_at)
        
        return session.load_oauth_session(
            token_type=self.credentials.token_type,
            access_token=self.credentials.access_token,
            refresh_token=self.credentials.refresh_token,
            expiry_time=expiry_time
        )
    
    def get_type(self) -> AuthenticationType:
        return AuthenticationType.OAUTH


class AuthenticationService:
    """Domain Service: Handles Tidal authentication"""
    
    def __init__(self, strategy: AuthenticationStrategy):
        self.strategy = strategy
    
    def authenticate(self, session: Session) -> bool:
        """Authenticate with Tidal using configured strategy"""
        try:
            result = self.strategy.authenticate(session)
            if result:
                print(f"✓ Authenticated using {self.strategy.get_type().name}")
            return result
        except Exception as e:
            raise AuthenticationError(f"Authentication failed: {e}") from e


class AuthenticationError(Exception):
    """Domain Exception: Authentication-related errors"""
    pass
