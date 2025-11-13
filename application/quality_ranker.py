"""
Application layer: Quality-based ranking for Tidal albums.
"""

from typing import Optional
from domain.tidal import TidalAlbum
from domain.discogs import DiscogsRelease
from domain.recording import Recording
from domain.canonical import (
    is_canonical_performer, is_canonical_label,
    get_canonical_performer_score, get_canonical_label_score
)


class QualityRanker:
    """
    Ranks Tidal albums by quality rather than just metadata match.
    
    Prioritizes:
    1. Exact match to Discogs/Scaruffi (if available) = 1.0
    2. Canonical performers/conductors = high weight
    3. Canonical labels = medium weight
    4. Tidal popularity = low weight (tiebreaker)
    
    Philosophy: Find the best available recording, not just the closest match.
    """
    
    # Scoring weights (must sum to 1.0)
    WEIGHT_EXACT_MATCH = 1.0  # If exact match found, score = 1.0 immediately
    WEIGHT_CANONICAL_PERFORMER = 0.50
    WEIGHT_CANONICAL_LABEL = 0.35
    WEIGHT_POPULARITY = 0.15
    
    def score_album(
        self,
        album: TidalAlbum,
        recording: Recording,
        discogs_release: Optional[DiscogsRelease] = None,
        max_popularity: int = 100
    ) -> float:
        """
        Score a Tidal album for quality (0.0-1.0).
        
        Args:
            album: Tidal album to score
            recording: Original Scaruffi recording
            discogs_release: Discogs release if found (for exact matching)
            max_popularity: Maximum popularity in result set (for normalization)
        
        Returns:
            Score from 0.0 (worst) to 1.0 (perfect/exact match)
        """
        # Check for exact match using Discogs metadata
        if discogs_release and self._is_exact_match(album, discogs_release):
            return 1.0
        
        # Otherwise, score by quality indicators
        score = 0.0
        
        # Canonical performer scoring
        performer_score = self._score_performers(album.artists)
        score += self.WEIGHT_CANONICAL_PERFORMER * performer_score
        
        # Canonical label scoring (Tidal doesn't expose label, so score = 0)
        # Note: In real implementation, might extract from album metadata
        # For now, this weight goes unused
        
        # Popularity scoring (normalized)
        if max_popularity > 0:
            popularity_score = album.popularity / max_popularity
            score += self.WEIGHT_POPULARITY * popularity_score
        
        return min(score, 1.0)
    
    def _is_exact_match(
        self,
        album: TidalAlbum,
        discogs_release: DiscogsRelease
    ) -> bool:
        """
        Check if Tidal album exactly matches Discogs release.
        
        Requires strong evidence:
        - Year match (±1 year)
        - At least 2 of 3 metadata fields match strongly
        """
        matches = 0
        
        # Check title match
        title_match = (
            discogs_release.title.lower() in album.title.lower() or
            album.title.lower() in discogs_release.title.lower()
        )
        if title_match:
            matches += 1
        
        # Check artist match
        artist_match = album.matches_discogs_metadata(
            discogs_artists=discogs_release.artists,
            discogs_year=None,  # Don't check year in this helper
            discogs_title=discogs_release.title
        )
        if artist_match:
            matches += 1
        
        # Check year match (±1 year tolerance)
        if discogs_release.year and album.year:
            if abs(discogs_release.year - album.year) <= 1:
                matches += 1
        
        # Need at least 2 strong matches for confidence
        return matches >= 2
    
    def _score_performers(self, artists: tuple[str, ...]) -> float:
        """
        Score performers/artists by canonical status.
        
        Returns highest score among all artists (not average).
        Rationale: One canonical performer makes it a quality recording.
        """
        if not artists:
            return 0.0
        
        scores = [get_canonical_performer_score(artist) for artist in artists]
        return max(scores)
    
    def rank_albums(
        self,
        albums: list[TidalAlbum],
        recording: Recording,
        discogs_release: Optional[DiscogsRelease] = None
    ) -> list[tuple[TidalAlbum, float]]:
        """
        Rank a list of Tidal albums by quality.
        
        Args:
            albums: List of Tidal albums to rank
            recording: Original Scaruffi recording
            discogs_release: Discogs release if found
        
        Returns:
            List of (album, score) tuples, sorted by score (descending)
        """
        if not albums:
            return []
        
        # Find max popularity for normalization
        max_popularity = max(album.popularity for album in albums)
        
        # Score each album
        scored = [
            (album, self.score_album(album, recording, discogs_release, max_popularity))
            for album in albums
        ]
        
        # Sort by score (descending)
        scored.sort(key=lambda x: x[1], reverse=True)
        
        return scored
    
    def find_best_match(
        self,
        albums: list[TidalAlbum],
        recording: Recording,
        discogs_release: Optional[DiscogsRelease] = None,
        min_score: float = 0.3
    ) -> Optional[tuple[TidalAlbum, float]]:
        """
        Find the best matching album from a list.
        
        Args:
            albums: List of Tidal albums to search
            recording: Original Scaruffi recording
            discogs_release: Discogs release if found
            min_score: Minimum acceptable score
        
        Returns:
            (best_album, score) or None if no album meets threshold
        """
        ranked = self.rank_albums(albums, recording, discogs_release)
        
        if not ranked:
            return None
        
        best_album, best_score = ranked[0]
        
        if best_score >= min_score:
            return (best_album, best_score)
        
        return None
