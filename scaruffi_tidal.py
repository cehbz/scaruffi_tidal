#!/usr/bin/env python3
"""
Scaruffi-to-Tidal Playlist Creator
Main CLI application
"""

import sys
import argparse
import logging
from pathlib import Path

import tidalapi

from infrastructure.scaruffi_parser import ScaruffiParser
from infrastructure.discogs_client import DiscogsClient
from infrastructure.tidal_client import TidalClient
from infrastructure.config import ConfigManager
from application.orchestrator import PlaylistOrchestrator


def setup_logging(verbose: bool = False):
    """Configure logging."""
    level = logging.DEBUG if verbose else logging.INFO
    logging.basicConfig(
        level=level,
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
        datefmt='%H:%M:%S'
    )


def authenticate_tidal(config_manager: ConfigManager) -> tidalapi.Session:
    """Authenticate with Tidal."""
    print("Authenticating with Tidal...")
    
    session = tidalapi.Session()
    
    # Try loading existing session
    session_file = config_manager.config_path.parent / 'tidal_session.json'
    if session_file.exists():
        try:
            if session.load_session_from_file(session_file):
                if session.check_login():
                    print("✓ Loaded existing Tidal session")
                    return session
        except:
            pass
    
    # Need new authentication
    print("\nStarting OAuth login...")
    print("Please follow the prompts to authenticate.")
    
    try:
        session.login_oauth_simple()
        
        if session.check_login():
            print("✓ Tidal authentication successful")
            
            # Save session
            try:
                session.save_session_to_file(session_file)
                print(f"✓ Session saved to {session_file}")
            except:
                pass
            
            return session
    except Exception as e:
        print(f"✗ Tidal authentication failed: {e}")
        sys.exit(1)
    
    print("✗ Authentication failed")
    sys.exit(1)


def main():
    parser = argparse.ArgumentParser(
        description='Create Tidal playlists from Scaruffi recommendations',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Create playlist from local HTML file
  %(prog)s classical.html

  # Create playlist from URL with custom name
  %(prog)s https://www.scaruffi.com/music/classica.html --name "My Classical Playlist"

  # Use custom quality threshold
  %(prog)s classical.html --min-score 0.5

  # Verbose logging
  %(prog)s classical.html --verbose
        """
    )
    
    parser.add_argument(
        'source',
        help='Scaruffi HTML file path or URL'
    )
    
    parser.add_argument(
        '--name',
        default='Scaruffi: A Recommended Discography of Classical Masterpieces',
        help='Playlist name (default: %(default)s)'
    )
    
    parser.add_argument(
        '--min-score',
        type=float,
        default=0.3,
        help='Minimum quality score (0.0-1.0, default: %(default)s)'
    )
    
    parser.add_argument(
        '--no-discogs',
        action='store_true',
        help='Skip Discogs lookup (faster but less accurate)'
    )
    
    parser.add_argument(
        '--config',
        type=Path,
        help='Path to config file (default: ~/.config/scaruffi-tidal/config.yaml)'
    )
    
    parser.add_argument(
        '--verbose', '-v',
        action='store_true',
        help='Verbose output'
    )
    
    args = parser.parse_args()
    
    # Setup
    setup_logging(args.verbose)
    logger = logging.getLogger(__name__)
    
    # Load configuration
    config_manager = ConfigManager(args.config)
    config = config_manager.load()
    
    # Check Discogs credentials
    discogs_client = None
    if not args.no_discogs:
        if not config.has_discogs_credentials():
            print("⚠ Warning: Discogs token not configured")
            print(f"   Add 'discogs.token' to {config_manager.config_path}")
            print("   Continuing without Discogs (matching may be less accurate)")
        else:
            discogs_client = DiscogsClient(
                token=config.discogs_token,
                rate_limit=config.discogs_rate_limit
            )
            logger.info("✓ Discogs client initialized")
    
    # Authenticate with Tidal
    tidal_session = authenticate_tidal(config_manager)
    tidal_client = TidalClient(session=tidal_session)
    logger.info("✓ Tidal client initialized")
    
    # Load source
    source = args.source
    if source.startswith('http://') or source.startswith('https://'):
        # Download URL
        import urllib.request
        logger.info(f"Downloading: {source}")
        with urllib.request.urlopen(source) as response:
            html = response.read().decode('utf-8')
    else:
        # Read local file
        source_path = Path(source)
        if not source_path.exists():
            print(f"✗ File not found: {source}")
            sys.exit(1)
        
        logger.info(f"Reading: {source}")
        with open(source_path, 'r', encoding='utf-8') as f:
            html = f.read()
    
    # Create orchestrator
    orchestrator = PlaylistOrchestrator(
        scaruffi_parser=ScaruffiParser(),
        discogs_client=discogs_client,
        tidal_client=tidal_client
    )
    
    # Create playlist
    print(f"\n{'=' * 60}")
    print(f"Creating playlist: {args.name}")
    print(f"Minimum quality score: {args.min_score}")
    print(f"{'=' * 60}\n")
    
    try:
        playlist_id, results = orchestrator.create_playlist_from_html(
            html=html,
            playlist_name=args.name,
            min_score=args.min_score
        )
        
        # Summary
        print(f"\n{'=' * 60}")
        print("COMPLETE!")
        print(f"{'=' * 60}")
        print(f"Playlist ID: {playlist_id}")
        print(f"URL: https://listen.tidal.com/playlist/{playlist_id}")
        print()
        print("Results:")
        print(f"  Total entries: {len(results)}")
        print(f"  Exact matches: {len([r for r in results if r.is_exact_match])}")
        print(f"  Good matches: {len([r for r in results if r.found_on_tidal and not r.is_exact_match])}")
        print(f"  Not found: {len([r for r in results if not r.found_on_tidal])}")
        print(f"{'=' * 60}\n")
        
        # Show not found entries
        not_found = [r for r in results if not r.found_on_tidal]
        if not_found:
            print(f"Not found on Tidal ({len(not_found)} entries):")
            for result in not_found[:10]:  # Show first 10
                print(f"  - {result.scaruffi_entry}")
            if len(not_found) > 10:
                print(f"  ... and {len(not_found) - 10} more")
        
        return 0
        
    except Exception as e:
        logger.exception("Failed to create playlist")
        print(f"\n✗ Error: {e}")
        return 1


if __name__ == '__main__':
    sys.exit(main())
