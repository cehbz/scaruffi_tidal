#!/usr/bin/env python3
"""
Phase 2 Demo: Discogs Integration
Demonstrates Scaruffi parsing + Discogs lookup
"""

from pathlib import Path
from infrastructure.scaruffi_parser import ScaruffiParser
from infrastructure.config import ConfigManager, AppConfiguration

# Parse Scaruffi HTML
print("=" * 60)
print("PHASE 2 DEMO: Scaruffi + Discogs Integration")
print("=" * 60)
print()

html_path = Path("/mnt/user-data/uploads/classical.html")
if not html_path.exists():
    print("classical.html not found!")
    exit(1)

parser = ScaruffiParser()
with open(html_path, 'r', encoding='utf-8') as f:
    html = f.read()

entries = parser.parse_html(html)
print(f"✓ Parsed {len(entries)} Scaruffi entries")
print()

# Show sample entries
print("Sample parsed entries:")
for i, entry in enumerate(entries[:5], 1):
    print(f"{i}. {entry}")
    rec = entry.primary_recording
    details = []
    if rec.performer:
        details.append(f"Performer: {rec.performer}")
    if rec.year:
        details.append(f"Year: {rec.year}")
    if rec.label:
        details.append(f"Label: {rec.label}")
    if details:
        print(f"   {', '.join(details)}")
print()

# Check configuration
print("Configuration Status:")
config_mgr = ConfigManager()
config = config_mgr.load()

print(f"  Config file: {config_mgr.config_path}")
print(f"  Discogs credentials: {'✓ Configured' if config.has_discogs_credentials() else '✗ Missing'}")
print(f"  Tidal credentials: {'✓ Configured' if config.has_tidal_credentials() else '✗ Missing'}")
print(f"  Discogs rate limit: {config.discogs_rate_limit} req/min")
print()

if not config.has_discogs_credentials():
    print("Note: To test Discogs lookup, add your token to:")
    print(f"  {config_mgr.config_path}")
    print()
    print("Example config.yaml:")
    print("---")
    print("discogs:")
    print("  token: your_discogs_token_here")
    print()
else:
    print("✓ Ready for Discogs lookups!")
    print()
    print("Discogs client would search for each entry like:")
    for entry in entries[:3]:
        query = f"{entry.composer} {entry.work}"
        print(f"  - '{query}'")
        if entry.primary_recording.performer:
            print(f"    Filter by: {entry.primary_recording.performer}", end="")
            if entry.primary_recording.year:
                print(f" ({entry.primary_recording.year})", end="")
            print()

print()
print("=" * 60)
print("Phase 2 Complete! Ready for Phase 3 (Tidal Integration)")
print("=" * 60)
