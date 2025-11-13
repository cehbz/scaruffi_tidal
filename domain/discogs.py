"""
Domain models for Discogs releases.
"""

from dataclasses import dataclass
from typing import Optional


@dataclass(frozen=True)
class DiscogsRelease:
    """
    Value Object: A specific release on Discogs.
    
    Can be either a master release or a specific pressing/edition.
    Contains metadata that helps match to Tidal releases.
    """
    id: int
    title: str
    artists: tuple[str, ...]
    year: Optional[int] = None
    labels: tuple[str, ...] = ()
    formats: tuple[str, ...] = ()
    master_id: Optional[int] = None
    is_master: bool = False
    # Quality metrics
    community_rating: Optional[float] = None  # 0.0-5.0
    community_have: int = 0  # Number of users who have it
    community_want: int = 0  # Number of users who want it
    
    def __post_init__(self):
        if self.id <= 0:
            raise ValueError(f"Release ID must be positive, got {self.id}")
        if not self.title or not self.title.strip():
            raise ValueError("Release title is required")
        if not self.artists:
            raise ValueError("At least one artist is required")
        if self.year is not None and (self.year < 1000 or self.year > 2100):
            raise ValueError(f"Year must be between 1000 and 2100, got {self.year}")
        if self.community_rating is not None and (self.community_rating < 0 or self.community_rating > 5):
            raise ValueError(f"Community rating must be between 0 and 5, got {self.community_rating}")
    
    @property
    def primary_artist(self) -> str:
        """Get the first/primary artist."""
        return self.artists[0] if self.artists else ""
    
    @property
    def primary_label(self) -> Optional[str]:
        """Get the first/primary label."""
        return self.labels[0] if self.labels else None
    
    def matches_recording_metadata(
        self,
        performer: Optional[str],
        label: Optional[str],
        year: Optional[int]
    ) -> bool:
        """
        Check if this release matches the given metadata.
        
        Used to identify exact matches to Scaruffi's recommendations.
        """
        # Year must match if both specified
        if year and self.year and abs(self.year - year) > 2:
            return False
        
        # Check performer/artist match (fuzzy)
        if performer:
            performer_lower = performer.lower()
            artist_match = any(
                performer_lower in artist.lower() or artist.lower() in performer_lower
                for artist in self.artists
            )
            if not artist_match:
                return False
        
        # Check label match (fuzzy)
        if label:
            label_lower = label.lower()
            label_match = any(
                label_lower in disc_label.lower() or disc_label.lower() in label_lower
                for disc_label in self.labels
            )
            if not label_match:
                return False
        
        return True
    
    def __str__(self) -> str:
        """Human-readable representation for logging."""
        parts = [self.primary_artist, self.title]
        if self.year:
            parts.append(str(self.year))
        if self.primary_label:
            parts.append(self.primary_label)
        if self.is_master:
            parts.append("[MASTER]")
        return " - ".join(parts)


@dataclass(frozen=True)
class DiscogsSearchResult:
    """
    Value Object: Result of searching Discogs for a recording.
    
    Contains the Scaruffi recording and the matched Discogs release (if found).
    """
    recording: 'Recording'  # Forward reference
    discogs_release: Optional[DiscogsRelease]
    search_query: str
    results_found: int  # Total number of results Discogs returned
    
    def __post_init__(self):
        if self.results_found < 0:
            raise ValueError(f"Results found must be non-negative, got {self.results_found}")
        if not self.search_query:
            raise ValueError("Search query is required")
    
    @property
    def found_exact_match(self) -> bool:
        """Check if we found a Discogs release."""
        return self.discogs_release is not None
    
    def __str__(self) -> str:
        """Human-readable representation for logging."""
        if self.discogs_release:
            return f"Found: {self.discogs_release}"
        else:
            return f"Not found (searched: {self.search_query}, {self.results_found} results)"
