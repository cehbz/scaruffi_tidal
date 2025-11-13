"""
Application layer: Main orchestrator for Scaruffi-to-Tidal workflow.
"""

from dataclasses import dataclass
from typing import Optional
import logging

from domain.scaruffi_entry import ScaruffiEntry
from domain.discogs import DiscogsSearchResult
from domain.tidal import TidalAlbum
from infrastructure.scaruffi_parser import ScaruffiParser
from infrastructure.discogs_client import DiscogsClient
from infrastructure.tidal_client import TidalClient


logger = logging.getLogger(__name__)


@dataclass(frozen=True)
class MatchResult:
    """
    Result of matching a Scaruffi entry to Tidal.
    
    Contains the complete journey: Scaruffi → Discogs → Tidal
    """
    scaruffi_entry: ScaruffiEntry
    discogs_result: Optional[DiscogsSearchResult]
    tidal_album: Optional[TidalAlbum]
    quality_score: float
    
    @property
    def found_on_tidal(self) -> bool:
        """Check if we found a Tidal album."""
        return self.tidal_album is not None
    
    @property
    def is_exact_match(self) -> bool:
        """Check if this is an exact match (score = 1.0)."""
        return self.quality_score >= 0.99
    
    def __str__(self) -> str:
        """Human-readable summary for logging."""
        if not self.tidal_album:
            return f"✗ {self.scaruffi_entry} → Not found on Tidal"
        
        quality = "EXACT" if self.is_exact_match else f"{self.quality_score:.2f}"
        discogs_info = f" [via Discogs {self.discogs_result.discogs_release.id}]" if self.discogs_result and self.discogs_result.found_exact_match else ""
        
        return f"✓ {self.scaruffi_entry} → {self.tidal_album} (quality: {quality}){discogs_info}"


class PlaylistOrchestrator:
    """
    Orchestrates the complete Scaruffi-to-Tidal playlist creation workflow.
    
    Steps:
    1. Parse Scaruffi HTML
    2. For each entry, lookup on Discogs
    3. Search Tidal and rank by quality
    4. Create playlist and add best matches
    """
    
    def __init__(
        self,
        scaruffi_parser: ScaruffiParser,
        discogs_client: Optional[DiscogsClient],
        tidal_client: TidalClient
    ):
        """
        Initialize orchestrator.
        
        Args:
            scaruffi_parser: Parser for Scaruffi HTML
            discogs_client: Discogs client (optional, used for exact matching)
            tidal_client: Tidal client
        """
        self.scaruffi_parser = scaruffi_parser
        self.discogs_client = discogs_client
        self.tidal_client = tidal_client
    
    def create_playlist_from_html(
        self,
        html: str,
        playlist_name: str,
        min_score: float = 0.3
    ) -> tuple[str, list[MatchResult]]:
        """
        Create Tidal playlist from Scaruffi HTML.
        
        Args:
            html: Scaruffi HTML content
            playlist_name: Name for the Tidal playlist
            min_score: Minimum quality score to include in playlist
        
        Returns:
            (playlist_id, list of match results)
        """
        logger.info("=" * 60)
        logger.info(f"Creating playlist: {playlist_name}")
        logger.info("=" * 60)
        
        # Step 1: Parse Scaruffi
        logger.info("Step 1: Parsing Scaruffi HTML...")
        entries = self.scaruffi_parser.parse_html(html)
        logger.info(f"Parsed {len(entries)} entries")
        
        # Step 2: Match entries to Tidal
        logger.info("Step 2: Matching to Tidal...")
        match_results = []
        
        for i, entry in enumerate(entries, 1):
            logger.info(f"[{i}/{len(entries)}] Processing: {entry}")
            
            match_result = self._match_entry(entry, min_score)
            match_results.append(match_result)
            
            logger.info(f"  → {match_result}")
        
        # Step 3: Create playlist
        logger.info("Step 3: Creating Tidal playlist...")
        playlist_id = self.tidal_client.create_playlist(
            name=playlist_name,
            description=f"Scaruffi's classical music recommendations - {len([r for r in match_results if r.found_on_tidal])}/{len(entries)} tracks"
        )
        
        # Step 4: Add albums to playlist
        logger.info("Step 4: Adding albums to playlist...")
        added_count = 0
        
        for result in match_results:
            if result.found_on_tidal:
                try:
                    self.tidal_client.add_album_to_playlist(
                        playlist_id=playlist_id,
                        album_id=result.tidal_album.id
                    )
                    added_count += 1
                except Exception as e:
                    logger.error(f"Failed to add album {result.tidal_album.id}: {e}")
        
        logger.info("=" * 60)
        logger.info(f"Playlist created: {playlist_id}")
        logger.info(f"Added {added_count}/{len(match_results)} albums")
        logger.info(f"Exact matches: {len([r for r in match_results if r.is_exact_match])}")
        logger.info(f"Good matches: {len([r for r in match_results if r.found_on_tidal and not r.is_exact_match])}")
        logger.info(f"Not found: {len([r for r in match_results if not r.found_on_tidal])}")
        logger.info("=" * 60)
        
        return playlist_id, match_results
    
    def _match_entry(
        self,
        entry: ScaruffiEntry,
        min_score: float
    ) -> MatchResult:
        """
        Match a single Scaruffi entry to Tidal.
        
        Args:
            entry: Scaruffi entry to match
            min_score: Minimum acceptable quality score
        
        Returns:
            MatchResult with complete matching info
        """
        # Try primary recording first
        discogs_result = None
        
        if self.discogs_client:
            try:
                discogs_result = self.discogs_client.search_recording(
                    entry.primary_recording
                )
            except Exception as e:
                logger.warning(f"Discogs lookup failed: {e}")
        
        # Search Tidal
        tidal_result = self.tidal_client.find_best_album(
            recording=entry.primary_recording,
            discogs_release=discogs_result.discogs_release if discogs_result else None,
            min_score=min_score
        )
        
        if tidal_result:
            album, score = tidal_result
            return MatchResult(
                scaruffi_entry=entry,
                discogs_result=discogs_result,
                tidal_album=album,
                quality_score=score
            )
        
        # Try alternates if primary failed
        if entry.alternate_recordings:
            for alt_recording in entry.alternate_recordings:
                logger.debug(f"Trying alternate: {alt_recording.performer}")
                
                # Try Discogs for alternate
                if self.discogs_client:
                    try:
                        discogs_result = self.discogs_client.search_recording(alt_recording)
                    except Exception as e:
                        logger.debug(f"Discogs lookup failed for alternate: {e}")
                
                # Search Tidal for alternate
                tidal_result = self.tidal_client.find_best_album(
                    recording=alt_recording,
                    discogs_release=discogs_result.discogs_release if discogs_result else None,
                    min_score=min_score
                )
                
                if tidal_result:
                    album, score = tidal_result
                    logger.info(f"  → Found via alternate: {alt_recording.performer}")
                    return MatchResult(
                        scaruffi_entry=entry,
                        discogs_result=discogs_result,
                        tidal_album=album,
                        quality_score=score
                    )
        
        # No match found
        return MatchResult(
            scaruffi_entry=entry,
            discogs_result=discogs_result,
            tidal_album=None,
            quality_score=0.0
        )
