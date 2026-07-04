# tidalist

Curate a platform-agnostic **golden playlist of albums and recordings**, then
**render** it onto a platform (Tidal now; Spotify / local files later).

Two decoupled stages:

1. **Golden** — an intent (a natural-language brief from an agent, or Scaruffi's
   classical page) becomes candidates; metadata providers (MusicBrainz, Discogs)
   discover which albums and recordings exist and pin their identity (MBIDs); a brief
   discriminates; the result is an ordered, persisted **golden playlist** whose units
   are whole albums or single recordings — the durable product.
2. **Render** — each golden entry is mapped best-effort to playable items on a
   platform (an album expands to its tracks), producing a playlist plus a gap report
   for anything unavailable, and a note for any edition compromise.

The golden playlist is platform-agnostic JSON: switch metadata providers, or render
the same golden onto another platform, without re-curating. Platform availability
never prunes curation — it is reported as gaps. Edition limitations of a backend are
reported as compromises and never bleed back into the golden master.

## Install

Requires [uv](https://docs.astral.sh/uv/) and Python ≥ 3.11.

```bash
uv sync
```

## Configure

A single XDG config file (override the base with `XDG_CONFIG_HOME`):

```bash
mkdir -p ~/.config/tidalist
cat > ~/.config/tidalist/config.yaml <<'EOF'
musicbrainz:
  contact: "you@example.com"    # MusicBrainz requires a contact in the User-Agent
discogs:
  token: "your_discogs_token"   # optional
  rate_limit: 60                # requests/minute
EOF
```

Tidal uses OAuth on the first `render`/`publish` (a `link.tidal.com` URL to
approve); the session is cached at `~/.config/tidalist/tidal_session.json`.

## Use

The pipeline is a set of verbs over JSON artifacts. Run via the `tidalist` command
(after `uv sync`) or `uv run python -m tidalist`.

```bash
# 1. Produce an intent.json (hand-write or agent-generate — see the contract below).
#    The old `tidalist scaruffi` auto-parser is retired; the interpret stage
#    (see INTERPRET.md) now turns a source — page, brief, or list — into role-tagged intent.

# 2. Curate the golden playlist (discovers albums & recordings, applies the brief)
uv run tidalist curate intent.json -o golden.json

# 3. Review the verdicts (what was admitted / rejected, and why)
uv run tidalist review golden.json

# 4. Resolve onto Tidal — see resolved tracks, gaps, and edition compromises, no write
uv run tidalist render golden.json

# 5. Create the Tidal playlist
uv run tidalist publish golden.json

# Or chain curate -> render -> publish:
uv run tidalist run intent.json -o golden.json
```

### Intent JSON

The hand-off a front-end (an agent, or the interpret stage — see INTERPRET.md) produces.
Playlists may mix whole **albums** and single **tracks**:

```json
{
  "name": "Steve Winwood — Albums & Classics",
  "brief": {
    "criteria": [{"type": "not_compilation"}]
  },
  "candidates": [
    {"artist": "Traffic", "title": "John Barleycorn Must Die", "kind": "album",
     "note": "Traffic's folk-jazz masterwork, 1970"},
    {"artist": "Steve Winwood", "title": "Higher Love", "kind": "track",
     "note": "1986 solo comeback",
     "criteria": [{"type": "performed_by", "artist": "Steve Winwood"}]}
  ]
}
```

Per candidate:

- **`kind`** — `"album"` (a whole release-group, the default golden unit for
  album-oriented playlists) or `"track"` (a single recording). Defaults to `"track"`.
- **`criteria`** — per-candidate criteria, combined with the brief's at judging time.
  A closed, validated tag union (`performed_by`, `studio`, `not_compilation`,
  `not_live`) — model output is never eval'd. Criteria are type-aware: recording
  criteria are no-ops on albums and vice-versa.
- **`edition`** — `{"markers": ["steven wilson"], "prefer_original": true}` overrides
  the render-time edition policy for an album. Editions are selected by **minimum
  weighted distance from the golden Album**: the golden carries a canonical tracklist
  (ordered, ISRC-identified, from MusicBrainz), and render picks the available edition
  nearest it — tracklist overlap dominates, with title, year, and reissue/kind markers
  as further dimensions. A requested marker (Steven Wilson / MoFi) is the highest-
  weighted dimension, so it wins when present and is reported as a *compromise* when no
  edition offers it. The default policy prefers Steven Wilson and Mobile Fidelity
  remasters. (This is how a 22-track mono+stereo deluxe loses to the 10-track original.)
- **`artist_mbid`** — an identity hint that bypasses the artist-search step in the
  MusicBrainz provider, pinning the candidate to an exact artist.

Each `note` becomes that entry's provenance rationale.

## Architecture

Ports & adapters around a pure, I/O-free domain core.

```
src/tidalist/
  core/       domain: recording, album, catalog, criteria, edition, ranking, brief,
              golden, render (Renderer port + Rendering), spec (JSON), ports, errors
  metadata/   MetadataProvider adapters: musicbrainz, discogs (+ rate_limit)
  render/     Renderer adapters: tidal (composes the Catalog port)
  tidal/      Tidal Catalog adapter + OAuth session
  nl/         the agent intent contract (parse_intent)
  cli.py      verbs: curate, review, render, publish, run
  config.py   AppConfig
```

- **MetadataProvider** (`recordings_for` + `albums_for`) feeds the golden stage:
  providers discover albums and recordings, pin identity (MBIDs, release-group
  secondary types for comp/live), and attach the album's **canonical tracklist** (the
  distance reference); the Curator discriminates via the brief.
- **Renderer** (`resolve` + `resolve_album` + `emit`) feeds the render stage: the
  Tidal renderer composes a Catalog, finds an anchor album by search, then enumerates
  **every** edition via the artist's discography (Tidal search dedupes editions) and
  picks the one of minimum distance to the golden's canonical tracklist
  (`Catalog.album_editions` + `core.fidelity.edition_distance`). Spotify / local-file /
  torrent renderers are drop-in later — the port is ready; impls are built on demand.
  (A local/torrent renderer with rich edition metadata can honor edition preferences
  Tidal cannot.)
- The domain core is stdlib-only frozen value objects; adapters never leak into it.

## Develop

```bash
uv run pytest -m "not integration"   # fast offline suite
uv run pytest                        # includes integration tests (need creds)
```

## Credits

Recommendations by Piero Scaruffi (scaruffi.com); metadata from MusicBrainz and
Discogs; streaming via Tidal.
