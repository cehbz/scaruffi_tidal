"""
Discogs API client.
Infrastructure layer - handles Discogs API communication.
"""

import discogs_client
from typing import Optional
import logging

from domain.recording import Recording
from domain.discogs import DiscogsRelease, DiscogsSearchResult
from infrastructure.rate_limiter import LeakyBucketRateLimiter


logger = logging.getLogger(__name__)

# Discogs API limits
MAX_RESULTS_PER_PAGE = 100  # Maximum per Discogs API
MAX_TOTAL_RESULTS = 200  # Stop after this many results (2 pages)


class DiscogsClient:
    """
    Client for Discogs API with rate limiting.
    
    Handles:
    - Authentication
    - Rate limiting (60 requests/minute)
    - Search and release lookup
    - Metadata extraction
    """
    
    def __init__(
        self,
        token: str,
        rate_limit: int = 60,
        user_agent: str = "ScaruffiTidal/1.0",
        cache_manager: Optional['CacheManager'] = None
    ):
        """
        Initialize Discogs client.
        
        Args:
            token: Discogs API token
            rate_limit: Requests per minute (default 60)
            user_agent: User agent for API requests
            cache_manager: Optional cache manager for result caching
        """
        if not token:
            raise ValueError("Discogs token is required")
        
        self._client = discogs_client.Client(user_agent, user_token=token)
        self._rate_limiter = LeakyBucketRateLimiter(requests_per_minute=rate_limit)
        self._cache = cache_manager
    
    def search_recording(self, recording: Recording) -> DiscogsSearchResult:
        """
        Search Discogs for a recording.
        
        Searches for the work, then filters results by metadata
        (performer, label, year) to find exact match.
        
        Fetches all available pages (up to MAX_TOTAL_RESULTS) to ensure
        comprehensive coverage.
        
        Args:
            recording: Recording to search for
        
        Returns:
            DiscogsSearchResult with matched release (if found)
        """
        # Check cache first
        if self._cache:
            cached_result = self._cache.get_discogs_result(recording)
            if cached_result:
                return cached_result
        
        # Build search query (composer + work)
        query = f"{recording.composer} {recording.work}"
        
        logger.info(f"Searching Discogs: {query}")
        
        # Fetch all results (multiple pages if needed)
        all_results = []
        page_num = 1
        
        while len(all_results) < MAX_TOTAL_RESULTS:
            # Rate-limited API call
            with self._rate_limiter:
                try:
                    search_results = self._client.search(
                        query,
                        type='release',
                        per_page=MAX_RESULTS_PER_PAGE
                    )
                    
                    # Get specific page
                    page_results = search_results.page(page_num)
                    
                    if not page_results:
                        break  # No more results
                    
                    all_results.extend(page_results)
                    
                    # Check if we got fewer results than requested (last page)
                    if len(page_results) < MAX_RESULTS_PER_PAGE:
                        break
                    
                    page_num += 1
                    logger.debug(f"Fetched page {page_num} ({len(all_results)} results so far)")
                    
                except Exception as e:
                    logger.error(f"Discogs search failed on page {page_num}: {e}")
                    break
        
        total_results = len(all_results)
        
        # Log if we hit the limit
        if total_results >= MAX_TOTAL_RESULTS:
            logger.warning(
                f"Discogs search returned {total_results}+ results for '{query}' "
                f"(limited to {MAX_TOTAL_RESULTS})"
            )
        
        logger.debug(f"Found {total_results} total Discogs results across {page_num} page(s)")
        
        if not all_results:
            result = DiscogsSearchResult(
                recording=recording,
                discogs_release=None,
                search_query=query,
                results_found=0
            )
            
            # Cache negative result
            if self._cache:
                self._cache.set_discogs_result(recording, result)
            
            return result
        
        # Filter results by metadata
        best_match = self._find_best_match(recording, all_results)
        
        if best_match:
            logger.info(f"Found Discogs match: {best_match}")
        else:
            logger.warning(
                f"No exact Discogs match for {recording.composer}: {recording.work} "
                f"(searched {total_results} results)"
            )
        
        result = DiscogsSearchResult(
            recording=recording,
            discogs_release=best_match,
            search_query=query,
            results_found=total_results
        )
        
        # Cache result
        if self._cache:
            self._cache.set_discogs_result(recording, result)
        
        return result
    
    def _find_best_match(
        self,
        recording: Recording,
        results: list
    ) -> Optional[DiscogsRelease]:
        """
        Find best matching release from search results.
        
        Filters by performer, label, and year if available.
        Returns first result that matches metadata.
        """
        for result in results:
            try:
                disc_release = self._parse_discogs_result(result)
                
                # Check if metadata matches
                if disc_release.matches_recording_metadata(
                    performer=recording.performer,
                    label=recording.label,
                    year=recording.year
                ):
                    return disc_release
            except Exception as e:
                logger.debug(f"Error parsing Discogs result: {e}")
                continue
        
        return None
    
    def _parse_discogs_result(self, result) -> DiscogsRelease:
        """
        Parse Discogs API result into domain model.
        
        Handles both releases and masters.
        """
        # Extract artists
        artists_list = getattr(result, 'artists', [])
        artists = tuple(artist.name for artist in artists_list) if artists_list else ()
        
        # Extract labels
        labels_list = getattr(result, 'labels', [])
        labels = tuple(label.name for label in labels_list) if labels_list else ()
        
        # Extract formats
        formats_data = getattr(result, 'formats', [])
        if formats_data:
            if isinstance(formats_data[0], dict):
                formats = tuple(
                    fmt.get('name', '') for fmt in formats_data
                )
            else:
                formats = tuple(str(fmt) for fmt in formats_data)
        else:
            formats = ()
        
        # Get year
        year = getattr(result, 'year', None)
        
        # Check if it's a master
        result_type = getattr(result, 'type', None)
        is_master = result_type == 'master'
        
        # Get master ID
        master_id = None
        if not is_master:
            master_id = getattr(result, 'master_id', None)
        
        # Extract community ratings - handle both attribute and data dict access
        rating = None
        have = 0
        want = 0
        
        # Try attribute-based access first (real API)
        community = getattr(result, 'community', None)
        if community:
            rating_obj = getattr(community, 'rating', None)
            if rating_obj:
                # Try average attribute or get method
                if hasattr(rating_obj, 'average'):
                    rating = getattr(rating_obj, 'average', None)
                elif hasattr(rating_obj, 'get'):
                    rating = rating_obj.get('average')
            have = getattr(community, 'have', 0)
            want = getattr(community, 'want', 0)
        
        return DiscogsRelease(
            id=result.id,
            title=result.title,
            artists=artists,
            year=year,
            labels=labels,
            formats=formats,
            master_id=master_id,
            is_master=is_master,
            community_rating=rating,
            community_have=have,
            community_want=want
        )
    
    def get_master_releases(self, master_id: int) -> list[DiscogsRelease]:
        """
        Get all releases for a master.
        
        Args:
            master_id: Discogs master ID
        
        Returns:
            List of releases for this master
        """
        logger.info(f"Fetching releases for master {master_id}")
        
        with self._rate_limiter:
            try:
                master = self._client.master(master_id)
                versions = master.versions.page(1)  # Get first page
                
                releases = []
                for version in versions[:10]:  # Limit to first 10
                    try:
                        disc_release = self._parse_discogs_result(version)
                        releases.append(disc_release)
                    except Exception as e:
                        logger.debug(f"Error parsing release: {e}")
                        continue
                
                return releases
            except Exception as e:
                logger.error(f"Failed to fetch master releases: {e}")
                return []
