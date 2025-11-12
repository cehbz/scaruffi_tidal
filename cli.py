#!/usr/bin/env python3
"""
Scaruffi-Tidal CLI Application
Main entry point with authentication and Scaruffi integration.
"""

import sys
import argparse
from pathlib import Path
from typing import Optional
import tidalapi
from tidalapi.session import Session

from application.auth import create_application, get_default_config_path
from domain.auth import AuthenticationError


class ScaruffiTidalCLI:
    """CLI interface for Scaruffi-Tidal application"""
    
    def __init__(self, config_path: Optional[Path] = None):
        self.auth_app = create_application(config_path)
        self.session: Optional[Session] = None
    
    def authenticate(self) -> bool:
        """Authenticate with Tidal"""
        print("Authenticating with Tidal...")
        
        # Create new session
        self.session = Session()
        
        # Try authentication with stored credentials
        if self.auth_app.authenticate(self.session):
            return True
        
        print("\nStored credentials are invalid or expired.")
        return False
    
    def interactive_login(self) -> bool:
        """Perform interactive login"""
        print("\nChoose authentication method:")
        print("1. OAuth login (recommended)")
        print("2. Session token (if you have one)")
        print("3. Exit")
        
        choice = input("\nSelect option (1-3): ").strip()
        
        if choice == '1':
            return self._oauth_login()
        elif choice == '2':
            return self._session_token_login()
        else:
            return False
    
    def _oauth_login(self) -> bool:
        """Perform OAuth login flow"""
        print("\nStarting OAuth login...")
        
        if not self.session:
            self.session = Session()
        
        try:
            # Try simple OAuth login first
            print("Please follow the instructions to login...")
            self.session.login_oauth_simple()
            
            # login_oauth_simple() blocks until login completes or times out
            if self.session.check_login():
                print("✓ OAuth login successful!")
                self._save_oauth_credentials()
                return True
            else:
                print("OAuth login completed but session check failed")
                
        except Exception as e:
            print(f"OAuth simple login failed: {e}")
            
            # Try browser-based OAuth
            try:
                link_login, future = self.session.login_oauth()
                print(f"\nPlease visit this URL to login:")
                print(f"{link_login.verification_uri_complete}")
                print(f"\nThe code will expire in {link_login.expires_in} seconds")
                print("Waiting for authentication (5 minute timeout)...")
                
                future.result(timeout=300)
                
                if self.session.check_login():
                    print("✓ Browser OAuth login successful!")
                    self._save_oauth_credentials()
                    return True
                        
            except Exception as e:
                print(f"Browser OAuth failed: {e}")
        
        return False
    
    def _session_token_login(self) -> bool:
        """Login using session token"""
        print("\nSession Token Login")
        print("Note: Session tokens expire and need to be renewed periodically.")
        
        token = input("Enter your Tidal session token: ").strip()
        
        if not token:
            print("✗ No token provided")
            return False
        
        # Validate token format (should be UUID)
        if len(token) != 36 or token.count('-') != 4:
            print("✗ Invalid token format (expected UUID)")
            return False
        
        # Save token and try authentication
        self.auth_app.update_session_token(token)
        
        if not self.session:
            self.session = Session()
        
        if self.auth_app.authenticate(self.session):
            print("✓ Session token login successful!")
            self.auth_app.save_session()
            return True
        else:
            print("✗ Session token login failed (token might be expired)")
            return False
    
    def _save_oauth_credentials(self) -> None:
        """Save OAuth credentials after successful login"""
        try:
            # Extract OAuth tokens from session
            access_token = None
            refresh_token = None
            
            # Try different attribute names
            for attr in ['_access_token', 'access_token']:
                if hasattr(self.session, attr):
                    access_token = getattr(self.session, attr)
                    break
            
            for attr in ['_refresh_token', 'refresh_token']:
                if hasattr(self.session, attr):
                    refresh_token = getattr(self.session, attr)
                    break
            
            if access_token:
                self.auth_app.update_oauth_tokens(access_token, refresh_token)
                print("✓ Credentials saved for future use")
            else:
                # Fallback: just save the session
                self.auth_app.save_session()
                
        except Exception as e:
            print(f"Warning: Could not save credentials: {e}")
    
    def test_connection(self) -> bool:
        """Test the Tidal connection"""
        if not self.session:
            print("Not authenticated")
            return False
        
        try:
            if self.session.check_login():
                print("\n✓ Connection test successful!")
                
                # Try to get user info
                user = self.session.user
                if user:
                    print(f"  User ID: {user.id}")
                    print(f"  Country: {self.session.country_code}")
                
                # Try search
                results = self.session.search("Test", limit=1)
                if results:
                    print("✓ Search functionality working")
                
                return True
            else:
                print("✗ Not logged in")
                return False
                
        except Exception as e:
            print(f"✗ Connection test failed: {e}")
            return False
    
    def process_scaruffi_url(self, url: str) -> None:
        """Process a Scaruffi URL and create playlists"""
        print(f"\nProcessing: {url}")
        
        # Import your existing Scaruffi processing code here
        # For now, this is a placeholder
        try:
            # You would add your existing scaruffi parsing and matching logic here
            print("TODO: Integrate existing Scaruffi parsing and matching code")
            print("Session is authenticated and ready for Tidal API calls")
        except Exception as e:
            print(f"Error processing URL: {e}")
    
    def run(self, scaruffi_url: Optional[str] = None) -> int:
        """Main application flow"""
        print("=" * 60)
        print("Scaruffi-Tidal")
        print("=" * 60)
        
        # Try authentication with stored credentials
        if self.authenticate():
            print("✓ Authenticated using stored credentials")
            
            if not self.test_connection():
                print("\nConnection test failed, trying re-authentication...")
                if not self.interactive_login():
                    return 1
        else:
            # If stored auth failed, try interactive login
            print("\nStarting interactive authentication...")
            
            if not self.interactive_login():
                print("\n✗ Authentication failed.")
                return 1
            
            if not self.test_connection():
                print("\n✗ Authentication succeeded but connection test failed.")
                return 1
        
        print("\n✓ Ready to process Scaruffi data!")
        
        # Process Scaruffi URL if provided
        if scaruffi_url:
            self.process_scaruffi_url(scaruffi_url)
        else:
            print("\nNo Scaruffi URL provided.")
            print("Usage: python cli.py [SCARUFFI_URL]")
        
        return 0


def main():
    """Main entry point"""
    parser = argparse.ArgumentParser(
        description='Scaruffi-Tidal: Parse Scaruffi ratings and create Tidal playlists'
    )
    
    parser.add_argument(
        'url',
        nargs='?',
        help='Scaruffi URL to process (e.g., https://www.scaruffi.com/music/classica.html)'
    )
    
    parser.add_argument(
        '--config',
        type=Path,
        help='Path to configuration file'
    )
    
    parser.add_argument(
        '--test',
        action='store_true',
        help='Test authentication and exit'
    )
    
    parser.add_argument(
        '--reset',
        action='store_true',
        help='Reset stored credentials'
    )
    
    args = parser.parse_args()
    
    # Handle reset
    if args.reset:
        config_path = args.config or get_default_config_path()
        if config_path.exists():
            config_path.unlink()
            print(f"✓ Reset configuration at {config_path}")
        session_file = config_path.parent / 'session.json'
        if session_file.exists():
            session_file.unlink()
            print(f"✓ Reset session file at {session_file}")
        return 0
    
    # Create and run CLI
    cli = ScaruffiTidalCLI(args.config)
    
    # Handle test mode
    if args.test:
        if cli.authenticate():
            return 0 if cli.test_connection() else 1
        else:
            print("✗ Authentication failed")
            return 1
    
    # Run normal flow
    return cli.run(args.url)


if __name__ == "__main__":
    sys.exit(main())
