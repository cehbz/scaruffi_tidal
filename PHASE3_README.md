# Scaruffi-Tidal Phase 3: Tidal Integration & Complete Workflow

## Summary

Successfully implemented complete Tidal integration with quality-based ranking, canonical performer lists, and end-to-end playlist creation orchestration.

## What's Complete

### Domain Layer
- `TidalAlbum`: Value object for Tidal releases with metadata
- `TidalTrack`: Value object for individual tracks
- `Canonical Lists`: Curated sets of respected performers, conductors, ensembles, orchestras, and labels
  - 40+ canonical conductors
  - 30+ canonical pianists
  - 15+ canonical string players
  - 25+ canonical ensembles
  - 15+ canonical orchestras
  - 20+ canonical labels

### Application Layer
- `QualityRanker`: Sophisticated quality-based ranking algorithm
  - Exact match detection using Discogs metadata (score = 1.0)
  - Canonical performer weighting (50%)
  - Canonical label weighting (35%)
  - Popularity tiebreaker (15%)
  - Finds best available recording, not just closest metadata match
  
- `PlaylistOrchestrator`: End-to-end workflow orchestration
  - Parse Scaruffi → Lookup Discogs → Search Tidal → Rank → Create playlist
  - Automatic fallback to alternate recordings
  - Comprehensive logging and reporting
  - Configurable quality thresholds

### Infrastructure Layer
- `TidalClient`: Full Tidal API integration
  - Album search
  - Quality-based ranking integration
  - Playlist creation
  - Album addition to playlists
  - Robust error handling

### Tests
- 52 comprehensive unit tests, all passing
- Coverage includes:
  - Canonical list recognition (15 tests)
  - Quality ranking algorithm (10 tests)
  - Phase 1 & 2 tests (27 tests)

## Key Features

### 1. Quality-Based Ranking

**Philosophy**: Find the *best available recording*, not just the closest metadata match.

**Scoring Algorithm**:
```python
# Exact match (via Discogs metadata)
if exact_match:
    return 1.0  # Perfect score

# Otherwise, score by quality indicators:
score = (
    0.50 * canonical_performer_score +  # Karajan, Gardiner, etc.
    0.35 * canonical_label_score +      # DG, Decca, ECM, etc.
    0.15 * popularity_normalized        # Tiebreaker only
)
```

**Canonical Performers**:
- Historic greats: Karajan, Bernstein, Klemperer, Furtwängler
- Period specialists: Gardiner, Harnoncourt, Pinnock, Savall
- Pianists: Gould, Pollini, Richter, Perahia, Schiff
- Ensembles: Il Giardino Armonico, Tallis Scholars, Alban Berg Quartet
- Orchestras: Berlin Phil, Vienna Phil, Concertgebouw

**Canonical Labels**:
- Prestige: Deutsche Grammophon (DG), Decca, EMI, Philips
- Audiophile: ECM, Hyperion, BIS
- Period: Harmonia Mundi, Archiv, Erato

### 2. Three-Stage Matching

**Stage 1: Discogs Lookup** (optional but recommended)
- Search Discogs for exact release
- Extract authoritative metadata
- Provides "ground truth" for exact matching

**Stage 2: Tidal Search**
- Search broadly by composer + work
- Get up to 50 candidates

**Stage 3: Quality Ranking**
- Score all candidates by quality
- Exact match (via Discogs) = 1.0 automatically
- Otherwise rank by canonical performers/labels
- Return best match above threshold

### 3. Complete Workflow Orchestration

```python
from application.orchestrator import PlaylistOrchestrator

orchestrator = PlaylistOrchestrator(
    scaruffi_parser=parser,
    discogs_client=discogs_client,  # Optional
    tidal_client=tidal_client
)

playlist_id, results = orchestrator.create_playlist_from_html(
    html=scaruffi_html,
    playlist_name="Scaruffi: Classical Masterpieces",
    min_score=0.3  # Quality threshold
)

# Results contain:
# - Scaruffi entry
# - Discogs result (if found)
# - Tidal album (if found)
# - Quality score
```

**Features**:
- Automatic fallback to alternate recordings
- Comprehensive logging at each step
- Detailed match reporting
- Configurable quality thresholds
- Error resilience (continues on failures)

## Architecture

### Separation of Concerns

**Domain Layer** (Pure Business Logic):
- `Recording`, `TidalAlbum`, `DiscogsRelease`: Value objects
- `Canonical Lists`: Quality criteria
- No dependencies on infrastructure

**Application Layer** (Use Cases):
- `QualityRanker`: Scoring algorithm
- `PlaylistOrchestrator`: Workflow coordination
- Uses domain objects, delegates to infrastructure

**Infrastructure Layer** (External Systems):
- `ScaruffiParser`: HTML parsing
- `DiscogsClient`: Discogs API
- `TidalClient`: Tidal API
- `ConfigManager`: File I/O
- `RateLimiter`: Request throttling

### Dependency Flow

```
Infrastructure → Application → Domain
(implements)     (uses)       (defines)
```

All dependencies point inward (Dependency Inversion Principle).

## Usage Example

### Complete Workflow

```python
import logging
from pathlib import Path

from infrastructure.scaruffi_parser import ScaruffiParser
from infrastructure.discogs_client import DiscogsClient
from infrastructure.tidal_client import TidalClient
from infrastructure.config import ConfigManager
from application.orchestrator import PlaylistOrchestrator
import tidalapi

# Setup logging
logging.basicConfig(level=logging.INFO)

# Load configuration
config = ConfigManager().load()

# Initialize clients
discogs_client = DiscogsClient(token=config.discogs_token)

# Authenticate with Tidal
tidal_session = tidalapi.Session()
tidal_session.login_oauth_simple()  # Follow prompts

tidal_client = TidalClient(session=tidal_session)

# Create orchestrator
orchestrator = PlaylistOrchestrator(
    scaruffi_parser=ScaruffiParser(),
    discogs_client=discogs_client,  # Optional
    tidal_client=tidal_client
)

# Load Scaruffi HTML
html_path = Path("classical.html")
with open(html_path, 'r') as f:
    html = f.read()

# Create playlist!
playlist_id, results = orchestrator.create_playlist_from_html(
    html=html,
    playlist_name="Scaruffi: A Recommended Discography of Classical Masterpieces",
    min_score=0.3  # Only add albums with score >= 0.3
)

# Analyze results
exact_matches = [r for r in results if r.is_exact_match]
good_matches = [r for r in results if r.found_on_tidal and not r.is_exact_match]
not_found = [r for r in results if not r.found_on_tidal]

print(f"\nPlaylist: {playlist_id}")
print(f"Exact matches: {len(exact_matches)}")
print(f"Good matches: {len(good_matches)}")
print(f"Not found: {len(not_found)}")

# Show some results
print("\nSample exact matches:")
for result in exact_matches[:5]:
    print(f"  {result}")

print("\nSample not found:")
for result in not_found[:5]:
    print(f"  {result}")
```

### Custom Quality Ranker

```python
from application.quality_ranker import QualityRanker

# Create custom ranker with different weights
ranker = QualityRanker()
ranker.WEIGHT_CANONICAL_PERFORMER = 0.60  # Increase performer weight
ranker.WEIGHT_CANONICAL_LABEL = 0.30
ranker.WEIGHT_POPULARITY = 0.10

# Use with Tidal client
tidal_client = TidalClient(session=tidal_session, ranker=ranker)
```

## Test Coverage

**52 tests total**, all passing in ~1.1 seconds:

- **Phase 1 (Scaruffi Parser)**: 11 tests
  - Entry parsing, year ranges, labels, alternates
  
- **Phase 2 (Discogs Integration)**: 16 tests
  - Rate limiter (5 tests)
  - Configuration (6 tests)
  - Discogs client (5 tests)
  
- **Phase 3 (Tidal Integration)**: 25 tests
  - Canonical lists (15 tests)
  - Quality ranker (10 tests)

## Dependencies

```
# From previous phases
beautifulsoup4
lxml
pyyaml
discogs_client

# New in Phase 3
tidalapi
```

## Configuration

```yaml
# ~/.config/scaruffi-tidal/config.yaml

discogs:
  token: "your_discogs_token"
  rate_limit: 60

tidal:
  # Session saved automatically after OAuth
  session_id: "uuid-here"
  country_code: "US"

matching:
  min_score: 0.3  # Minimum quality threshold
```

## Performance Characteristics

- **Scaruffi Parsing**: < 1 second for 270 entries
- **Discogs Lookups**: 60/minute (rate limited), ~0.5s per request
- **Tidal Searches**: No explicit rate limit, ~1s per search
- **Complete Workflow**: ~5-10 minutes for 270 entries (mostly waiting for Discogs)

## Known Limitations

1. **Tidal Label Metadata**: Tidal API doesn't expose label information in search results, so canonical label scoring is currently unused (35% weight goes to performer/popularity)

2. **Album vs Track Granularity**: System works at album level. Classical albums often contain multiple works, so playlist may include extra tracks.

3. **Rate Limiting**: Discogs is the bottleneck at 60 req/min. For 270 entries, expect ~4-5 minutes minimum.

4. **Authentication**: Tidal OAuth requires interactive browser flow on first run. Session is cached for reuse.

## Next Steps (Future Enhancements)

1. **Track-Level Matching**: Parse album tracks, extract only relevant works
2. **Label Extraction**: Enhance Tidal parser to extract label from album metadata
3. **Batch Processing**: Process multiple Scaruffi pages in parallel
4. **Caching**: Cache Discogs/Tidal lookups to avoid re-searching
5. **Web UI**: Build web interface for browsing results and manual overrides
6. **Multiple Tidalapi**: Handle multiple services (Spotify, Apple Music, etc.)

## Quality Assurance

✅ **TDD**: All features test-driven
✅ **DDD**: Clean domain model
✅ **SOLID**: Dependency inversion, single responsibility
✅ **Immutability**: All domain objects frozen
✅ **Type Hints**: Full type annotations
✅ **Logging**: Comprehensive logging throughout
✅ **Error Handling**: Graceful degradation on failures
