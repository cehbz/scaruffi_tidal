"""
Domain models for Tidal releases.
"""

from dataclasses import dataclass
from typing import Optional


@dataclass(frozen=True)
class TidalAlbum:
    """
    Value Object: An album/release on Tidal.
    
    Represents a classical music release that might match a Scaruffi recommendation.
    """
    id: int
    title: str
    artists: tuple[str, ...]
    release_date: Optional[str] = None  # ISO format YYYY-MM-DD
    duration_seconds: Optional[int] = None
    number_of_tracks: int = 0
    # Tidal-specific metadata
    popularity: int = 0
    audio_quality: Optional[str] = None  # "LOSSLESS", "HI_RES", etc.
    
    def __post_init__(self):
        if self.id <= 0:
            raise ValueError(f"Album ID must be positive, got {self.id}")
        if not self.title or not self.title.strip():
            raise ValueError("Album title is required")
        if not self.artists:
            raise ValueError("At least one artist is required")
        if self.number_of_tracks < 0:
            raise ValueError(f"Number of tracks must be non-negative, got {self.number_of_tracks}")
        if self.popularity < 0:
            raise ValueError(f"Popularity must be non-negative, got {self.popularity}")
    
    @property
    def primary_artist(self) -> str:
        """Get the first/primary artist."""
        return self.artists[0] if self.artists else ""
    
    @property
    def year(self) -> Optional[int]:
        """Extract year from release date."""
        if self.release_date and len(self.release_date) >= 4:
            try:
                return int(self.release_date[:4])
            except (ValueError, TypeError):
                return None
        return None
    
    def matches_discogs_metadata(
        self,
        discogs_artists: tuple[str, ...],
        discogs_year: Optional[int],
        discogs_title: str
    ) -> bool:
        """
        Check if this Tidal album matches Discogs release metadata.
        
        Used to identify exact matches (Scaruffi's recommended release).
        """
        # Title must be similar
        title_match = (
            discogs_title.lower() in self.title.lower() or
            self.title.lower() in discogs_title.lower()
        )
        if not title_match:
            return False
        
        # Year must match (within 1 year tolerance)
        if discogs_year and self.year:
            if abs(discogs_year - self.year) > 1:
                return False
        
        # At least one artist must match
        if discogs_artists:
            artist_match = any(
                any(
                    discogs_artist.lower() in tidal_artist.lower() or
                    tidal_artist.lower() in discogs_artist.lower()
                    for tidal_artist in self.artists
                )
                for discogs_artist in discogs_artists
            )
            if not artist_match:
                return False
        
        return True
    
    def __str__(self) -> str:
        """Human-readable representation for logging."""
        parts = [self.primary_artist, self.title]
        if self.year:
            parts.append(str(self.year))
        return " - ".join(parts)


@dataclass(frozen=True)
class TidalTrack:
    """
    Value Object: A track on Tidal.
    
    Used when we need track-level granularity (less common for classical).
    """
    id: int
    title: str
    album_id: int
    track_number: int
    duration_seconds: int
    
    def __post_init__(self):
        if self.id <= 0:
            raise ValueError(f"Track ID must be positive, got {self.id}")
        if not self.title:
            raise ValueError("Track title is required")
        if self.album_id <= 0:
            raise ValueError(f"Album ID must be positive, got {self.album_id}")
        if self.track_number < 0:
            raise ValueError(f"Track number must be non-negative, got {self.track_number}")
        if self.duration_seconds < 0:
            raise ValueError(f"Duration must be non-negative, got {self.duration_seconds}")
