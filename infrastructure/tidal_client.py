"""
Tidal API client.
Infrastructure layer - handles Tidal API communication.
"""

import tidalapi
from typing import Optional, List
import logging

from domain.tidal import TidalAlbum
from domain.recording import Recording
from domain.discogs import DiscogsRelease
from application.quality_ranker import QualityRanker


logger = logging.getLogger(__name__)


class TidalClient:
    """
    Client for Tidal API with search and playlist operations.
    
    Handles:
    - Authentication
    - Album search
    - Quality-based ranking
    - Playlist creation and population
    """
    
    def __init__(
        self,
        session: tidalapi.Session,
        ranker: Optional[QualityRanker] = None,
        cache_manager: Optional['CacheManager'] = None
    ):
        """
        Initialize Tidal client.
        
        Args:
            session: Authenticated tidalapi Session
            ranker: Quality ranker (creates default if not provided)
            cache_manager: Optional cache manager for result caching
        """
        if not session or not session.check_login():
            raise ValueError("Session must be authenticated")
        
        self._session = session
        self._ranker = ranker or QualityRanker()
        self._cache = cache_manager
    
    def search_albums(
        self,
        query: str,
        limit: int = 50
    ) -> list[TidalAlbum]:
        """
        Search Tidal for albums.
        
        Args:
            query: Search query (typically "Composer Work")
            limit: Maximum results to return
        
        Returns:
            List of TidalAlbum objects
        """
        # Check cache first
        if self._cache:
            cached_albums = self._cache.get_tidal_albums(query)
            if cached_albums:
                return cached_albums[:limit]  # Respect limit
        
        logger.info(f"Searching Tidal: {query}")
        
        try:
            results = self._session.search(query, models=[tidalapi.album.Album])
            
            if not results or not results.get('albums'):
                logger.debug(f"No albums found for: {query}")
                albums = []
            else:
                albums = []
                for tidal_album in results['albums'][:limit]:
                    try:
                        album = self._parse_tidal_album(tidal_album)
                        albums.append(album)
                    except Exception as e:
                        logger.debug(f"Error parsing album: {e}")
                        continue
                
                logger.info(f"Found {len(albums)} Tidal albums")
            
            # Cache results
            if self._cache:
                self._cache.set_tidal_albums(query, albums)
            
            return albums
            
        except Exception as e:
            logger.error(f"Tidal search failed: {e}")
            return []
    
    def find_best_album(
        self,
        recording: Recording,
        discogs_release: Optional[DiscogsRelease] = None,
        min_score: float = 0.3
    ) -> Optional[tuple[TidalAlbum, float]]:
        """
        Find the best Tidal album for a recording.
        
        Searches Tidal, ranks results by quality, returns best match.
        
        Args:
            recording: Scaruffi recording to search for
            discogs_release: Discogs release for exact matching
            min_score: Minimum acceptable quality score
        
        Returns:
            (best_album, score) or None if no suitable album found
        """
        # Build search query from recording
        query = recording.search_query()
        
        # Search Tidal
        albums = self.search_albums(query, limit=50)
        
        if not albums:
            logger.warning(f"No Tidal results for: {query}")
            return None
        
        # Rank by quality
        result = self._ranker.find_best_match(
            albums=albums,
            recording=recording,
            discogs_release=discogs_release,
            min_score=min_score
        )
        
        if result:
            album, score = result
            logger.info(f"Best match: {album} (score: {score:.2f})")
        else:
            logger.warning(f"No suitable match found for: {recording.composer} - {recording.work}")
        
        return result
    
    def create_playlist(self, name: str, description: Optional[str] = None) -> str:
        """
        Create a new Tidal playlist.
        
        Args:
            name: Playlist name
            description: Optional description
        
        Returns:
            Playlist ID (UUID string)
        """
        logger.info(f"Creating playlist: {name}")
        
        try:
            user = self._session.user
            if not user:
                raise ValueError("No authenticated user")
            
            playlist = user.create_playlist(name, description or "")
            
            logger.info(f"Created playlist: {playlist.id}")
            return playlist.id
            
        except Exception as e:
            logger.error(f"Failed to create playlist: {e}")
            raise
    
    def add_album_to_playlist(self, playlist_id: str, album_id: int) -> bool:
        """
        Add all tracks from an album to a playlist.
        
        Args:
            playlist_id: Tidal playlist ID
            album_id: Tidal album ID
        
        Returns:
            True if successful
        """
        logger.info(f"Adding album {album_id} to playlist {playlist_id}")
        
        try:
            # Get playlist
            playlist = self._session.playlist(playlist_id)
            
            # Get album
            album = self._session.album(album_id)
            
            # Get all track IDs from album
            tracks = album.tracks()
            track_ids = [str(track.id) for track in tracks]
            
            if not track_ids:
                logger.warning(f"No tracks found in album {album_id}")
                return False
            
            # Add tracks to playlist
            playlist.add(track_ids)
            
            logger.info(f"Added {len(track_ids)} tracks to playlist")
            return True
            
        except Exception as e:
            logger.error(f"Failed to add album to playlist: {e}")
            return False
    
    def _parse_tidal_album(self, tidal_album) -> TidalAlbum:
        """
        Parse tidalapi Album object into domain model.
        
        Args:
            tidal_album: tidalapi.album.Album object
        
        Returns:
            TidalAlbum domain object
        """
        # Extract artists
        artists = []
        if hasattr(tidal_album, 'artists') and tidal_album.artists:
            for artist in tidal_album.artists:
                if hasattr(artist, 'name'):
                    artists.append(artist.name)
        elif hasattr(tidal_album, 'artist') and tidal_album.artist:
            if hasattr(tidal_album.artist, 'name'):
                artists.append(tidal_album.artist.name)
        
        if not artists:
            artists = ["Unknown"]
        
        # Extract release date
        release_date = None
        if hasattr(tidal_album, 'release_date'):
            release_date = tidal_album.release_date
        elif hasattr(tidal_album, 'year'):
            # Some albums only have year
            year = tidal_album.year
            if year:
                release_date = f"{year}-01-01"
        
        # Extract other metadata
        duration = getattr(tidal_album, 'duration', None)
        num_tracks = getattr(tidal_album, 'num_tracks', 0)
        popularity = getattr(tidal_album, 'popularity', 0)
        audio_quality = getattr(tidal_album, 'audio_quality', None)
        
        return TidalAlbum(
            id=tidal_album.id,
            title=tidal_album.name or tidal_album.title,
            artists=tuple(artists),
            release_date=release_date,
            duration_seconds=duration,
            number_of_tracks=num_tracks,
            popularity=popularity,
            audio_quality=audio_quality
        )
