"""
Domain models for Scaruffi entries.
"""

from dataclasses import dataclass
from typing import Optional

from domain.recording import Recording


@dataclass(frozen=True)
class ScaruffiEntry:
    """
    Value Object: A parsed entry from Scaruffi's classical music page.
    
    Represents one work recommendation with primary recording and optional alternates.
    Raw text is preserved for debugging/logging.
    """
    composer: str
    work: str
    primary_recording: Recording
    alternate_recordings: tuple[Recording, ...]  # Immutable sequence
    raw_text: str
    
    def __post_init__(self):
        if not self.composer or not self.composer.strip():
            raise ValueError("Composer is required")
        if not self.work or not self.work.strip():
            raise ValueError("Work is required")
        if not self.raw_text:
            raise ValueError("Raw text is required for logging")
        
        # Validate alternates is a tuple
        if not isinstance(self.alternate_recordings, tuple):
            raise ValueError("Alternate recordings must be a tuple")
    
    def all_recordings(self) -> list[Recording]:
        """Get primary recording plus all alternates."""
        return [self.primary_recording] + list(self.alternate_recordings)
    
    def __str__(self) -> str:
        """Human-readable representation for logging."""
        performer = self.primary_recording.performer or 'unknown'
        return f"{self.composer}: {self.work} [{performer}]"
