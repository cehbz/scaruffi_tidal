"""
Domain models for classical music recordings.
Following Domain-Driven Design principles.
"""

import re
from dataclasses import dataclass
from typing import Optional


@dataclass(frozen=True)
class Recording:
    """
    Value Object: A specific performance/recording of a musical work.
    
    Represents a recommendation with metadata about the performance.
    """
    composer: str
    work: str
    performer: Optional[str] = None
    label: Optional[str] = None
    year: Optional[int] = None
    
    def __post_init__(self):
        if not self.composer or not self.composer.strip():
            raise ValueError("Composer is required")
        if not self.work or not self.work.strip():
            raise ValueError("Work is required")
        if self.year is not None and (self.year < 1000 or self.year > 2100):
            raise ValueError(f"Year must be between 1000 and 2100, got {self.year}")
    
    def search_query(self) -> str:
        """Build search query for broad matching (composer + work)."""
        return f"{self.composer} {self.work}".strip()
