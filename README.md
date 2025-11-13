# Scaruffi-to-Tidal: Classical Music Playlist Generator

Automatically create Tidal playlists from Piero Scaruffi's classical music recommendations using quality-based ranking and exact release matching via Discogs.

## Features

✅ **Parse Scaruffi's Classical Recommendations** - Extract 270+ entries with metadata
✅ **Discogs Integration** - Find exact releases for authoritative matching  
✅ **Quality-Based Ranking** - Prioritize canonical performers (Karajan, Gardiner, Gould, etc.)
✅ **Tidal Integration** - Search, rank, and create playlists automatically
✅ **Intelligent Fallback** - Try alternate recordings when primary not found
✅ **Rate Limiting** - Respect API limits (60 req/min for Discogs)
✅ **52 Passing Tests** - Comprehensive test coverage across all layers
✅ **Clean Architecture** - DDD, SOLID, TDD throughout

## Quick Start

```bash
# 1. Install dependencies
pip install beautifulsoup4 lxml pyyaml discogs_client tidalapi

# 2. Configure (get Discogs token from https://www.discogs.com/settings/developers)
mkdir -p ~/.config/scaruffi-tidal
cat > ~/.config/scaruffi-tidal/config.yaml << EOF
discogs:
  token: "your_discogs_token_here"
EOF

# 3. Run (will prompt for Tidal login on first run)
wget https://www.scaruffi.com/music/classica.html
python scaruffi_tidal.py classical.html
```

See [QUICKSTART.md](QUICKSTART.md) for detailed instructions.

## How It Works

### Three-Stage Pipeline

```
Scaruffi HTML
    ↓ (parse)
Recording Entry (composer, work, performer, year, label)
    ↓ (lookup)
Discogs Release (exact metadata, master ID)
    ↓ (search)
Tidal Candidates (50 albums per work)
    ↓ (rank by quality)
Best Match (exact match = 1.0, canonical performer = high, popularity = low)
    ↓ (add to playlist)
Tidal Playlist
```

### Quality Ranking Algorithm

```python
IF exact_match_via_discogs:
    score = 1.0  # Perfect
ELSE:
    score = (
        50% × canonical_performer_score +  # Karajan, Gardiner, etc.
        35% × canonical_label_score +      # DG, Decca, ECM, etc.
        15% × popularity_normalized        # Tiebreaker only
    )

RETURN best_album if score >= min_threshold (default: 0.3)
```

### Canonical Quality Indicators

**80+ Canonical Performers**:
- Conductors: Karajan, Bernstein, Gardiner, Harnoncourt, Klemperer, Solti
- Pianists: Gould, Pollini, Richter, Perahia, Schiff, Uchida
- String players: Grumiaux, Fournier, Isserlis, Perlman
- Ensembles: Il Giardino Armonico, Tallis Scholars, Alban Berg Quartet
- Orchestras: Berlin Phil, Vienna Phil, Concertgebouw

**20+ Canonical Labels**:
- Prestige: Deutsche Grammophon (DG), Decca, EMI, Philips
- Audiophile: ECM, Hyperion, BIS
- Period: Harmonia Mundi, Archiv

## Architecture

```
┌───────────────────────────────────────────┐
│         PlaylistOrchestrator              │
│   (Application Layer - Use Cases)         │
└─────┬─────────────┬──────────────┬────────┘
      │             │              │
      ▼             ▼              ▼
┌──────────┐  ┌──────────┐  ┌──────────┐
│ Scaruffi │  │ Discogs  │  │  Tidal   │
│  Parser  │  │  Client  │  │  Client  │
│          │  │  +Rate   │  │  +Ranker │
│(Phase 1) │  │ Limiter  │  │(Phase 3) │
│          │  │(Phase 2) │  │          │
└──────────┘  └──────────┘  └──────────┘
      │             │              │
      └─────────────┴──────────────┘
                    │
            ┌───────▼────────┐
            │  Domain Models │
            │  - Recording   │
            │  - Release     │
            │  - Album       │
            │  - Canonical   │
            └────────────────┘
```

### Layers

**Domain Layer** (Pure Business Logic):
- Value objects: `Recording`, `DiscogsRelease`, `TidalAlbum`
- Canonical lists: Quality criteria for performers/labels
- No dependencies on infrastructure

**Application Layer** (Use Cases):
- `QualityRanker`: Quality-based scoring algorithm
- `PlaylistOrchestrator`: Complete workflow coordination
- Uses domain objects, delegates to infrastructure

**Infrastructure Layer** (External Systems):
- `ScaruffiParser`: HTML parsing with BeautifulSoup
- `DiscogsClient`: Discogs API with rate limiting
- `TidalClient`: Tidal API wrapper
- `ConfigManager`: XDG-compliant configuration
- `RateLimiter`: Leaky bucket algorithm

## Test Coverage

**52 tests, all passing** (~1.1 seconds):

### By Phase
- **Phase 1 (Scaruffi)**: 11 tests - Parser with all format variations
- **Phase 2 (Discogs)**: 16 tests - Rate limiter, config, Discogs client
- **Phase 3 (Tidal)**: 25 tests - Canonical lists, quality ranker

### By Layer
- **Domain**: 15 tests (canonical lists)
- **Application**: 10 tests (quality ranker)
- **Infrastructure**: 27 tests (parsers, clients, utilities)

Run tests:
```bash
python -m unittest discover -s tests -p "test_*.py"
```

## Performance

For Scaruffi's 270-entry classical page:
- **Parsing**: < 1 second
- **Discogs lookups**: ~4.5 minutes (60 requests/min rate limit)
- **Tidal searches**: ~4.5 minutes (~1 second each)
- **Playlist creation**: < 1 second
- **Album additions**: ~2 minutes
- **Total**: ~11-12 minutes

**Bottleneck**: Discogs rate limiting (60 req/min)

## Project Structure

```
scaruffi-tidal/
├── domain/                    # Pure business logic
│   ├── recording.py           # Recording value object
│   ├── discogs.py             # Discogs release models
│   ├── tidal.py               # Tidal album models
│   ├── scaruffi_entry.py      # Parsed Scaruffi entry
│   └── canonical.py           # Quality indicator lists
│
├── application/               # Use cases
│   ├── quality_ranker.py      # Quality-based ranking
│   └── orchestrator.py        # Complete workflow
│
├── infrastructure/            # External systems
│   ├── scaruffi_parser.py     # HTML parsing
│   ├── discogs_client.py      # Discogs API
│   ├── tidal_client.py        # Tidal API
│   ├── config.py              # Configuration management
│   └── rate_limiter.py        # Request throttling
│
├── tests/                     # Comprehensive tests (52)
│   ├── domain/
│   ├── application/
│   └── infrastructure/
│
└── scaruffi_tidal.py          # Main CLI script
```

## Configuration

```yaml
# ~/.config/scaruffi-tidal/config.yaml

discogs:
  token: "your_discogs_api_token"
  rate_limit: 60

matching:
  min_score: 0.3  # Quality threshold
```

Tidal session is saved automatically after OAuth login.

## Usage Examples

### Basic

```bash
python scaruffi_tidal.py classical.html
```

### Custom Playlist Name

```bash
python scaruffi_tidal.py classical.html --name "Baroque Masterpieces"
```

### Higher Quality Threshold

```bash
python scaruffi_tidal.py classical.html --min-score 0.5
```

### Skip Discogs (Faster, Less Accurate)

```bash
python scaruffi_tidal.py classical.html --no-discogs
```

### From URL

```bash
python scaruffi_tidal.py https://www.scaruffi.com/music/classica.html
```

## Documentation

- **[QUICKSTART.md](QUICKSTART.md)** - Installation and basic usage
- **[PHASE3_README.md](PHASE3_README.md)** - Complete technical documentation
- **[PHASE3_SUMMARY.md](PHASE3_SUMMARY.md)** - Architecture and design decisions

## Design Principles

- **Test-Driven Development**: All features test-driven (52 tests)
- **Domain-Driven Design**: Rich domain model with ubiquitous language
- **SOLID Principles**: Single responsibility, dependency inversion
- **Clean Architecture**: Domain → Application → Infrastructure
- **Immutability**: All domain objects frozen (thread-safe)
- **Comprehensive Logging**: Detailed logging at each step
- **Error Resilience**: Graceful degradation on failures

## Dependencies

```
beautifulsoup4  # HTML parsing
lxml            # HTML parser backend
pyyaml          # Configuration files
discogs_client  # Discogs API
tidalapi        # Tidal API
```

## Known Limitations

1. **Album Granularity**: Works at album level. Classical albums may contain multiple works.
2. **Tidal Label Metadata**: Tidal API doesn't expose labels, so 35% weight unused.
3. **Discogs Rate Limit**: 60 requests/min bottleneck (unavoidable with free tier).
4. **Interactive OAuth**: First-time Tidal login requires browser.

## Future Enhancements

- Track-level matching for precise work extraction
- Label extraction from Tidal metadata
- Parallel processing for multiple pages
- Caching layer for repeated lookups
- Web UI for browsing and manual overrides
- Support for other streaming services

## Development

Built in three phases:
1. **Phase 1**: Scaruffi HTML parsing
2. **Phase 2**: Discogs integration with rate limiting
3. **Phase 3**: Tidal integration with quality ranking

All phases follow TDD, DDD, and SOLID principles with comprehensive test coverage.

## License

See individual file headers for license information.

## Credits

- Classical music recommendations by Piero Scaruffi (https://www.scaruffi.com)
- Metadata provided by Discogs (https://www.discogs.com)
- Streaming via Tidal (https://www.tidal.com)
