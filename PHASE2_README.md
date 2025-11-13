# Scaruffi-Tidal Phase 2: Discogs Integration

## Summary

Successfully implemented Discogs API integration with rate limiting, configuration management, and metadata matching.

## What's Complete

### Domain Layer
- `DiscogsRelease`: Value object for Discogs releases with metadata and quality metrics
- `DiscogsSearchResult`: Result of searching Discogs with matched release
- Enhanced `Recording` model for metadata matching

### Infrastructure Layer
- `DiscogsClient`: Full Discogs API client with:
  - Authentication via user token
  - Search and release lookup
  - Master release handling
  - Metadata extraction (artists, labels, year, formats, community ratings)
- `LeakyBucketRateLimiter`: Thread-safe rate limiter implementing leaky bucket algorithm
  - Respects 60 requests/minute limit
  - Uses response time for accurate rate calculation
  - Supports burst requests up to capacity
- `ConfigManager`: XDG-compliant configuration management
  - Loads/saves Tidal and Discogs credentials
  - Supports custom rate limits
  - YAML format for human readability

### Tests
- 27 comprehensive unit tests, all passing
- Coverage includes:
  - Rate limiter behavior (burst, leaking, blocking)
  - Configuration load/save with Discogs credentials
  - Discogs search and metadata filtering
  - Mock-based testing without real API calls

## Features

✅ **Rate Limiting**:
- Leaky bucket algorithm for smooth rate control
- 60 requests/minute default (configurable)
- Thread-safe for concurrent use
- Response-time-aware for accuracy

✅ **Discogs Integration**:
- Search by composer + work title
- Filter results by performer/label/year metadata
- Extract community ratings (if available)
- Handle both master releases and specific pressings
- Master release version enumeration

✅ **Metadata Matching**:
- Fuzzy matching on performer names
- Label substring matching
- Year tolerance (±2 years)
- Returns first exact match or None

✅ **Configuration**:
- XDG Base Directory compliant (~/.config/scaruffi-tidal/config.yaml)
- Stores Tidal and Discogs credentials
- YAML format for easy editing
- Validates credentials presence

## Configuration Format

```yaml
discogs:
  token: "your_discogs_api_token_here"
  rate_limit: 60  # optional, defaults to 60

tidal:
  session_token: "your_tidal_session_token"  # or
  oauth_token: "your_tidal_oauth_token"

matching:
  threshold: 0.5  # optional, for future use
```

## Dependencies

```
beautifulsoup4
lxml
pyyaml
discogs_client
```

## Usage Example

```python
from infrastructure.discogs_client import DiscogsClient
from infrastructure.config import ConfigManager
from domain.recording import Recording

# Load configuration
config_manager = ConfigManager()
config = config_manager.load()

if not config.has_discogs_credentials():
    print("Discogs token not configured!")
    exit(1)

# Create client
client = DiscogsClient(
    token=config.discogs_token,
    rate_limit=config.discogs_rate_limit
)

# Search for a recording
recording = Recording(
    composer="Bach",
    work="Brandenburg Concertos",
    performer="Il Giardino Armonico",
    year=1997
)

result = client.search_recording(recording)

if result.found_exact_match:
    release = result.discogs_release
    print(f"Found on Discogs: {release}")
    print(f"  Release ID: {release.id}")
    print(f"  Artists: {', '.join(release.artists)}")
    print(f"  Year: {release.year}")
    if release.primary_label:
        print(f"  Label: {release.primary_label}")
    if release.community_rating:
        print(f"  Rating: {release.community_rating}/5.0")
else:
    print(f"Not found on Discogs (searched {result.results_found} results)")
```

## Architecture Notes

### Rate Limiter Design
The leaky bucket implementation allows natural burst behavior while maintaining average rate:
- Bucket fills with each request
- Leaks at constant rate (requests_per_minute / 60)
- Blocks when bucket reaches capacity
- Uses `with` context manager for clean acquire/release

### Discogs Matching Strategy
1. Search broadly by composer + work
2. Get first page of results (up to 50)
3. Filter by metadata (performer, label, year) if provided
4. Return first exact match
5. If master found, can enumerate specific pressings via `get_master_releases()`

### Why Not Cache Discogs Results?
Each Scaruffi entry is a different work, so there's minimal opportunity for cache hits. Rate limiting provides sufficient protection.

## Test Coverage

- **Rate Limiter**: 5 tests covering burst, leaking, blocking, response tracking
- **Config Manager**: 6 tests covering load, save, defaults, validation
- **Discogs Client**: 5 tests covering search, filtering, masters, rate limiting
- **Scaruffi Parser**: 11 tests (from Phase 1, still passing)

All 27 tests pass in < 1 second.

## Next Phase: Tidal Integration

Phase 3 will implement:
1. Tidal API client wrapper
2. Album search and ranking
3. Quality-based scoring with canonical performer/label lists
4. Exact match detection using Discogs metadata
5. Playlist creation and population
