# Scaruffi-to-Tidal Quickstart Guide

## Installation

```bash
# Extract the code
tar xzf scaruffi-tidal-phase3.tar.gz
cd scaruffi-tidal-phase3

# Install dependencies
pip install beautifulsoup4 lxml pyyaml discogs_client tidalapi --break-system-packages

# Verify installation
python -m unittest discover -s tests -p "test_*.py"
# Should see: Ran 52 tests... OK
```

## Configuration

1. **Get a Discogs API token** (optional but recommended):
   - Go to https://www.discogs.com/settings/developers
   - Generate a new token
   - Copy the token

2. **Create config file**:

```bash
mkdir -p ~/.config/scaruffi-tidal
cat > ~/.config/scaruffi-tidal/config.yaml << EOF
discogs:
  token: "your_discogs_token_here"
  rate_limit: 60
EOF
```

3. **Authenticate with Tidal** (on first run):
   - Script will open browser for OAuth
   - Follow prompts to login
   - Session is saved for future use

## Usage

### Basic Usage

```bash
# Download Scaruffi's classical music page
wget https://www.scaruffi.com/music/classica.html

# Create playlist (will prompt for Tidal login on first run)
python scaruffi_tidal.py classical.html
```

### Advanced Usage

```bash
# Custom playlist name
python scaruffi_tidal.py classical.html --name "My Classical Favorites"

# Higher quality threshold (more selective)
python scaruffi_tidal.py classical.html --min-score 0.5

# Skip Discogs for faster (but less accurate) matching
python scaruffi_tidal.py classical.html --no-discogs

# Verbose logging
python scaruffi_tidal.py classical.html --verbose

# From URL directly
python scaruffi_tidal.py https://www.scaruffi.com/music/classica.html
```

## Expected Output

```
Authenticating with Tidal...
✓ Loaded existing Tidal session
✓ Discogs client initialized
✓ Tidal client initialized

============================================================
Creating playlist: Scaruffi: Classical Masterpieces
Minimum quality score: 0.3
============================================================

Step 1: Parsing Scaruffi HTML...
Parsed 270 entries

Step 2: Matching to Tidal...
[1/270] Processing: Bach: Brandenburg Concertos [Il Giardino Armonico]
  → ✓ Bach: Brandenburg Concertos [Il Giardino Armonico] → Il Giardino Armonico - Brandenburg Concertos - 1997 (quality: EXACT) [via Discogs 12345]

[2/270] Processing: Mozart: Requiem, K.626 [Gardiner]
  → ✓ Mozart: Requiem [Gardiner] → John Eliot Gardiner - Mozart: Requiem - 1986 (quality: EXACT)

[...]

Step 3: Creating Tidal playlist...
Created playlist: abc-123-def-456

Step 4: Adding albums to playlist...
Added 245/270 albums

============================================================
COMPLETE!
============================================================
Playlist ID: abc-123-def-456
URL: https://listen.tidal.com/playlist/abc-123-def-456

Results:
  Total entries: 270
  Exact matches: 180
  Good matches: 65
  Not found: 25
============================================================
```

## Typical Runtime

For Scaruffi's 270-entry classical page:
- **With Discogs**: ~10-15 minutes (rate limited to 60/min)
- **Without Discogs**: ~5 minutes (Tidal searches only)

## Troubleshooting

### Tidal Authentication Fails

```bash
# Clear saved session
rm ~/.config/scaruffi-tidal/tidal_session.json

# Try again
python scaruffi_tidal.py classical.html
```

### Discogs Rate Limit Errors

The script respects the 60 requests/minute limit automatically. If you see errors:
- Check your token is valid
- Ensure no other processes are using the same token
- Wait a minute and retry

### No Tidal Results Found

Some recordings may not be available on Tidal:
- Rare/obscure recordings
- Very new releases
- Historical recordings not yet digitized
- Region-specific availability

Lower the `--min-score` threshold to be less selective:
```bash
python scaruffi_tidal.py classical.html --min-score 0.2
```

### ImportError: No module named 'X'

Install missing dependencies:
```bash
pip install beautifulsoup4 lxml pyyaml discogs_client tidalapi --break-system-packages
```

## Understanding Quality Scores

- **1.0 (EXACT)**: Exact match to Scaruffi's recommendation via Discogs metadata
- **0.7-0.9**: High-quality canonical performer (e.g., Karajan, Gardiner)
- **0.4-0.6**: Good recording by respected performer/ensemble
- **0.3-0.4**: Acceptable recording (above minimum threshold)
- **< 0.3**: Excluded from playlist (quality too low)

## Customizing Quality Criteria

To modify which performers/labels are considered "canonical", edit:
- `domain/canonical.py`

Add your preferred performers to the appropriate sets:
```python
CANONICAL_CONDUCTORS = frozenset({
    "Karajan", "Bernstein", "Gardiner",
    # Add more here
    "Your Favorite Conductor",
})
```

## Next Steps

- **Browse the playlist** on Tidal
- **Adjust `--min-score`** to be more/less selective
- **Try other Scaruffi pages** (jazz, rock, etc.)
- **Contribute improvements** to the canonical lists

## Support

For issues, questions, or contributions:
- Review test suite: `python -m unittest discover -s tests`
- Check logs with `--verbose` flag
- See [PHASE3_README.md](PHASE3_README.md) for detailed documentation
