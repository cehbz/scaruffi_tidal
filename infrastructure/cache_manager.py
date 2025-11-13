"""
Cache manager for Discogs and Tidal API results.
Infrastructure layer - handles persistent caching with SQLite.
"""

import sqlite3
import json
import hashlib
import logging
from pathlib import Path
from typing import Optional, Any
from datetime import datetime, timedelta
from dataclasses import asdict

from domain.recording import Recording
from domain.discogs import DiscogsSearchResult, DiscogsRelease
from domain.tidal import TidalAlbum


logger = logging.getLogger(__name__)


class CacheManager:
    """
    Persistent cache for API results using SQLite.
    
    Caches:
    - Discogs search results (30-day expiry)
    - Tidal search results (7-day expiry)
    
    Benefits:
    - First run: Full API calls (~12 minutes for 270 entries)
    - Subsequent runs: Cache hits (~30 seconds)
    - Partial invalidation: Only refetch expired entries
    """
    
    # Cache expiry settings
    DISCOGS_EXPIRY_DAYS = 30
    TIDAL_EXPIRY_DAYS = 7
    
    def __init__(self, cache_path: Optional[Path] = None):
        """
        Initialize cache manager.
        
        Args:
            cache_path: Path to SQLite database (default: XDG cache dir)
        """
        if cache_path is None:
            cache_path = self._get_default_cache_path()
        
        self.cache_path = cache_path
        self.cache_path.parent.mkdir(parents=True, exist_ok=True)
        
        self.db = sqlite3.connect(str(cache_path), check_same_thread=False)
        self.db.row_factory = sqlite3.Row
        
        self._init_schema()
        
        logger.info(f"Cache initialized at {cache_path}")
    
    def _get_default_cache_path(self) -> Path:
        """Get XDG-compliant cache path."""
        import os
        xdg_cache = os.environ.get('XDG_CACHE_HOME')
        
        if xdg_cache:
            base = Path(xdg_cache)
        else:
            base = Path.home() / '.cache'
        
        return base / 'scaruffi-tidal' / 'cache.db'
    
    def _init_schema(self):
        """Initialize database schema."""
        self.db.execute("""
            CREATE TABLE IF NOT EXISTS discogs_cache (
                query_hash TEXT PRIMARY KEY,
                composer TEXT,
                work TEXT,
                performer TEXT,
                label TEXT,
                year INTEGER,
                result_json TEXT,
                timestamp INTEGER
            )
        """)
        
        self.db.execute("""
            CREATE TABLE IF NOT EXISTS tidal_cache (
                query_hash TEXT PRIMARY KEY,
                query TEXT,
                results_json TEXT,
                timestamp INTEGER
            )
        """)
        
        # Create indexes for timestamp-based queries
        self.db.execute("""
            CREATE INDEX IF NOT EXISTS idx_discogs_timestamp 
            ON discogs_cache(timestamp)
        """)
        
        self.db.execute("""
            CREATE INDEX IF NOT EXISTS idx_tidal_timestamp 
            ON tidal_cache(timestamp)
        """)
        
        self.db.commit()
    
    def get_discogs_result(self, recording: Recording) -> Optional[DiscogsSearchResult]:
        """
        Get cached Discogs search result.
        
        Args:
            recording: Recording to lookup
        
        Returns:
            Cached DiscogsSearchResult or None if not found/expired
        """
        query_hash = self._hash_recording(recording)
        
        cursor = self.db.execute("""
            SELECT result_json, timestamp
            FROM discogs_cache
            WHERE query_hash = ?
        """, (query_hash,))
        
        row = cursor.fetchone()
        if not row:
            logger.debug(f"Cache miss (Discogs): {recording.composer} - {recording.work}")
            return None
        
        # Check expiry
        timestamp = row['timestamp']
        age_days = (datetime.now().timestamp() - timestamp) / 86400
        
        if age_days > self.DISCOGS_EXPIRY_DAYS:
            logger.debug(f"Cache expired (Discogs): {recording.composer} - {recording.work} ({age_days:.1f} days old)")
            return None
        
        # Deserialize
        try:
            result_data = json.loads(row['result_json'])
            result = self._deserialize_discogs_result(result_data, recording)
            logger.debug(f"Cache hit (Discogs): {recording.composer} - {recording.work}")
            return result
        except Exception as e:
            logger.warning(f"Failed to deserialize cached Discogs result: {e}")
            return None
    
    def set_discogs_result(self, recording: Recording, result: DiscogsSearchResult):
        """
        Cache Discogs search result.
        
        Args:
            recording: Recording that was searched
            result: Search result to cache
        """
        query_hash = self._hash_recording(recording)
        result_json = json.dumps(self._serialize_discogs_result(result))
        timestamp = int(datetime.now().timestamp())
        
        self.db.execute("""
            INSERT OR REPLACE INTO discogs_cache
            (query_hash, composer, work, performer, label, year, result_json, timestamp)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        """, (
            query_hash,
            recording.composer,
            recording.work,
            recording.performer,
            recording.label,
            recording.year,
            result_json,
            timestamp
        ))
        
        self.db.commit()
        logger.debug(f"Cached Discogs result: {recording.composer} - {recording.work}")
    
    def get_tidal_albums(self, query: str) -> Optional[list[TidalAlbum]]:
        """
        Get cached Tidal search results.
        
        Args:
            query: Search query string
        
        Returns:
            List of cached TidalAlbum objects or None if not found/expired
        """
        query_hash = self._hash_string(query)
        
        cursor = self.db.execute("""
            SELECT results_json, timestamp
            FROM tidal_cache
            WHERE query_hash = ?
        """, (query_hash,))
        
        row = cursor.fetchone()
        if not row:
            logger.debug(f"Cache miss (Tidal): {query}")
            return None
        
        # Check expiry
        timestamp = row['timestamp']
        age_days = (datetime.now().timestamp() - timestamp) / 86400
        
        if age_days > self.TIDAL_EXPIRY_DAYS:
            logger.debug(f"Cache expired (Tidal): {query} ({age_days:.1f} days old)")
            return None
        
        # Deserialize
        try:
            results_data = json.loads(row['results_json'])
            albums = [self._deserialize_tidal_album(data) for data in results_data]
            logger.debug(f"Cache hit (Tidal): {query} ({len(albums)} albums)")
            return albums
        except Exception as e:
            logger.warning(f"Failed to deserialize cached Tidal results: {e}")
            return None
    
    def set_tidal_albums(self, query: str, albums: list[TidalAlbum]):
        """
        Cache Tidal search results.
        
        Args:
            query: Search query string
            albums: List of TidalAlbum objects to cache
        """
        query_hash = self._hash_string(query)
        results_json = json.dumps([self._serialize_tidal_album(album) for album in albums])
        timestamp = int(datetime.now().timestamp())
        
        self.db.execute("""
            INSERT OR REPLACE INTO tidal_cache
            (query_hash, query, results_json, timestamp)
            VALUES (?, ?, ?, ?)
        """, (query_hash, query, results_json, timestamp))
        
        self.db.commit()
        logger.debug(f"Cached Tidal results: {query} ({len(albums)} albums)")
    
    def clear(self, cache_type: Optional[str] = None):
        """
        Clear cache.
        
        Args:
            cache_type: 'discogs', 'tidal', or None for all
        """
        if cache_type == 'discogs':
            self.db.execute("DELETE FROM discogs_cache")
            logger.info("Cleared Discogs cache")
        elif cache_type == 'tidal':
            self.db.execute("DELETE FROM tidal_cache")
            logger.info("Cleared Tidal cache")
        else:
            self.db.execute("DELETE FROM discogs_cache")
            self.db.execute("DELETE FROM tidal_cache")
            logger.info("Cleared all caches")
        
        self.db.commit()
    
    def expire_old_entries(self):
        """Remove entries older than expiry thresholds."""
        now = int(datetime.now().timestamp())
        
        discogs_cutoff = now - (self.DISCOGS_EXPIRY_DAYS * 86400)
        tidal_cutoff = now - (self.TIDAL_EXPIRY_DAYS * 86400)
        
        cursor = self.db.execute("""
            DELETE FROM discogs_cache WHERE timestamp < ?
        """, (discogs_cutoff,))
        discogs_removed = cursor.rowcount
        
        cursor = self.db.execute("""
            DELETE FROM tidal_cache WHERE timestamp < ?
        """, (tidal_cutoff,))
        tidal_removed = cursor.rowcount
        
        self.db.commit()
        
        if discogs_removed or tidal_removed:
            logger.info(f"Expired {discogs_removed} Discogs + {tidal_removed} Tidal entries")
    
    def get_stats(self) -> dict[str, Any]:
        """Get cache statistics."""
        cursor = self.db.execute("SELECT COUNT(*) as count FROM discogs_cache")
        discogs_count = cursor.fetchone()['count']
        
        cursor = self.db.execute("SELECT COUNT(*) as count FROM tidal_cache")
        tidal_count = cursor.fetchone()['count']
        
        # Get size
        cache_size = self.cache_path.stat().st_size if self.cache_path.exists() else 0
        
        return {
            'cache_path': str(self.cache_path),
            'cache_size_mb': cache_size / (1024 * 1024),
            'discogs_entries': discogs_count,
            'tidal_entries': tidal_count,
            'total_entries': discogs_count + tidal_count
        }
    
    def _hash_recording(self, recording: Recording) -> str:
        """Generate hash for recording."""
        key = f"{recording.composer}:{recording.work}:{recording.performer}:{recording.label}:{recording.year}"
        return hashlib.sha256(key.encode()).hexdigest()
    
    def _hash_string(self, s: str) -> str:
        """Generate hash for string."""
        return hashlib.sha256(s.encode()).hexdigest()
    
    def _serialize_discogs_result(self, result: DiscogsSearchResult) -> dict:
        """Serialize DiscogsSearchResult to dict."""
        data = {
            'search_query': result.search_query,
            'results_found': result.results_found,
            'discogs_release': None
        }
        
        if result.discogs_release:
            release = result.discogs_release
            data['discogs_release'] = {
                'id': release.id,
                'title': release.title,
                'artists': list(release.artists),
                'year': release.year,
                'labels': list(release.labels),
                'formats': list(release.formats),
                'master_id': release.master_id,
                'is_master': release.is_master,
                'community_rating': release.community_rating,
                'community_have': release.community_have,
                'community_want': release.community_want
            }
        
        return data
    
    def _deserialize_discogs_result(self, data: dict, recording: Recording) -> DiscogsSearchResult:
        """Deserialize dict to DiscogsSearchResult."""
        from domain.discogs import DiscogsRelease, DiscogsSearchResult
        
        discogs_release = None
        if data['discogs_release']:
            r = data['discogs_release']
            discogs_release = DiscogsRelease(
                id=r['id'],
                title=r['title'],
                artists=tuple(r['artists']),
                year=r['year'],
                labels=tuple(r['labels']),
                formats=tuple(r['formats']),
                master_id=r['master_id'],
                is_master=r['is_master'],
                community_rating=r['community_rating'],
                community_have=r['community_have'],
                community_want=r['community_want']
            )
        
        return DiscogsSearchResult(
            recording=recording,
            discogs_release=discogs_release,
            search_query=data['search_query'],
            results_found=data['results_found']
        )
    
    def _serialize_tidal_album(self, album: TidalAlbum) -> dict:
        """Serialize TidalAlbum to dict."""
        return {
            'id': album.id,
            'title': album.title,
            'artists': list(album.artists),
            'release_date': album.release_date,
            'duration_seconds': album.duration_seconds,
            'number_of_tracks': album.number_of_tracks,
            'popularity': album.popularity,
            'audio_quality': album.audio_quality
        }
    
    def _deserialize_tidal_album(self, data: dict) -> TidalAlbum:
        """Deserialize dict to TidalAlbum."""
        return TidalAlbum(
            id=data['id'],
            title=data['title'],
            artists=tuple(data['artists']),
            release_date=data['release_date'],
            duration_seconds=data['duration_seconds'],
            number_of_tracks=data['number_of_tracks'],
            popularity=data['popularity'],
            audio_quality=data['audio_quality']
        )
    
    def close(self):
        """Close database connection."""
        self.db.close()
    
    def __enter__(self):
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        self.close()
