# Scaruffi-Tidal Phase 1: HTML Parser

## Summary

Successfully implemented and tested HTML parser for Scaruffi's classical music recommendations.

## What's Complete

### Domain Layer
- `Recording`: Value object for a musical recording (composer, work, performer, label, year)
- `ScaruffiEntry`: Value object for a parsed Scaruffi entry with primary + alternate recordings

### Infrastructure Layer
- `ScaruffiParser`: Robust HTML parser handling multiple format variations

### Tests
- 11 comprehensive unit tests, all passing
- Tested against real classical.html file (270 entries parsed successfully)

## Parser Features

✅ Handles multiple entry formats:
- Basic: `Performer (year)`
- With label: `Performer (Label)` or `Performer on Label`
- Year ranges: `(1963-73)` or `(1985 & 1988)`
- Conductor & Orchestra: `Karajan & Berliner Philharmoniker`
- Alternate recordings: `(also Alt1, Alt2)`
- Multiple alternates with OR: `Pollini or Zimerman`

✅ Extracts metadata:
- Composer, work title (always)
- Performer/conductor (usually)
- Year (33% of entries)
- Label (6% of entries)
- Alternate recommendations (25 entries have alternates)

## Statistics from classical.html

- **Total entries**: 270
- **Entries with year**: 90 (33.3%)
- **Entries with label**: 15 (5.6%)
- **Entries with alternates**: 25
- **Total alternate recordings**: 36

Top composers: Shostakovich (17), Bach (12), Beethoven (9)

## Dependencies

```
beautifulsoup4
lxml
```

## Usage

```python
from infrastructure.scaruffi_parser import ScaruffiParser
from pathlib import Path

parser = ScaruffiParser()

with open('classical.html', 'r', encoding='utf-8') as f:
    html = f.read()

entries = parser.parse_html(html)

for entry in entries:
    print(f"{entry.composer}: {entry.work}")
    print(f"  Performer: {entry.primary_recording.performer}")
    if entry.primary_recording.year:
        print(f"  Year: {entry.primary_recording.year}")
    if entry.alternate_recordings:
        print(f"  Alternates: {len(entry.alternate_recordings)}")
```

## Next Phase: Discogs Integration

Phase 2 will implement:
1. Discogs API client with rate limiting
2. Discogs release lookup and matching
3. Master/release handling
4. Quality signal extraction
