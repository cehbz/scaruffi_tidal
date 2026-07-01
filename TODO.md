# TODO

Active work only. Completed work is in git history; architecture/design in `docs/superpowers/`.

## Redesign (2026-06): intent → curate → render, Go reimplementation

Specs (gitignored): `docs/superpowers/specs/2026-06-28-intent-curate-render-design.md` (overall) + `2026-06-28-cli-grammar-intent-schema-design.md` (CLI grammar + intent markdown). Work is on the unmerged `slice1-credit-set-model` branch (slices 1/1b/2a–2d landed there; git log is the record).

- [ ] Slice 4: curate best-effort + report (LLM resolves identity, deterministic type-aware criteria gate admission, marginal-confidence flagged).
- [ ] Slice 5: render (rename realize→render) + diff/sync — the Python Tidal tool behind the JSON seam.
- [ ] Slice 6: descriptor discovery (4A LLM-knowledge materialize; 4B catalog attribute-queries — high value).
- [ ] Performance resolution (curate-adjacent; distinct from, not blocking, interpret). Spec: `docs/superpowers/specs/2026-07-01-performance-resolution-design.md` (brainstorm converged, mirror-validated, pre-plan). Resolve the *performance* (composer ∧ work-group ∧ conductor ∧ orchestra ∧ soloists ∧ chorus; year+label as selectors) over a federated MB+Discogs concordance — composer-aware work-group resolution (`l_work_work` 281) + conjunctive credit selection + first-co-release clustering + MB↔Discogs reconciliation via the already-materialised `artist.discogs_artist_id` bridge. Fixes the `find-recording --work` gaps (incl. the release-breadth `COUNT(isrc)` ordering that isn't original-vs-rerecording aware); retires top-1 title-only `resolveWorkID`.

### Go catalog — follow-ups
- [ ] Perf: first MB `recording_fts` MATCH ~2.6s cold (mmap of the 55GB file); measure warm vs cold, consider warming for interactive use. The Discogs `track_count` planner pathology is CROSS-JOIN-pinned; other large-mirror queries may need the same — the `//go:build integration` suite is the guard.
- [ ] `catalog/mirror.go` connector: bare `conn.(driver.ExecerContext)` assertion panics on a hypothetical future modernc conn → comma-ok + close + error.
- [ ] `catalog` ATTACH builds the URI by single-quote interpolation; `url.PathEscape` doesn't escape `'` → a DB path containing `'` breaks ATTACH. Parameterize/escape (low risk: trusted config path).

The items below predate the redesign — they describe the current Python pipeline (replaced slice-by-slice); several are real requirements the Go version must carry.

## Realize — remaining fidelity work

- [ ] Live no-silent-substitution integration test (deferred: no stable fixture).
- [ ] `ReleaseClassFacet` + `source_kind` (original vs comp) observation — needs a rich-metadata backend; title heuristics are unreliable.
- [ ] Per-track source-release attribution in the `album-source` compromise (name the releases, not just counts).
- [ ] Album-edition quality tiebreak: have `PlatformAlbum` carry `audio_quality`/`popularity` and apply `QualityPreference` in `resolve_album`.
- [ ] LLM judge for fuzzy title/identity matching (the deterministic string metrics' ceiling).

## Intent & curation

- [ ] Playlist ordering: an ordering directive in the intent (e.g. "by decreasing best-ness") the Curator preserves and publish honors.
- [ ] Dedup: drop a track whose recording is already on an album in the same playlist.
- [ ] `performed_by(member)` over-rejects band recordings credited to the group; needs band-membership / MB relationship awareness, not a performer-credit string match.
- [ ] Recording selection sometimes picks a later re-recording (e.g. "I'm a Man" -> 1988); prefer the original. (The performance-resolution design addresses this via first-release-date clustering.)
- [ ] Per-run caching: memoize the Tidal discography per artist (MB artist resolution is now a fast local mirror query, so less pressing).
- [ ] Local-file realizer (VLC / local files) and a persistent Tidal lookup cache. (The local MB/Discogs SQLite mirrors are done.)

## Metadata / edition reliability

- [ ] Canonical-tracklist choice for divergent national editions (US "Heaven Is in Your Mind" vs UK "Mr. Fantasy"); validate against a corpus.
- [ ] Cross-source drift when MB / Discogs / Tidal disagree on the release.
- [ ] Edition-distance weights (`core/fidelity.py`) are first-cut; the `year=None` bound and dominance invariant are unproven. Revisit if a playlist misselects.
- [ ] `num_tracks` coarse-shortlist (skip far-off editions' track fetches) was dropped; re-add if a discography is deep enough to matter.
- [ ] Narrow `album_editions`' broad `except` (`tidal/platform.py`) if it ever masks a real bug.
- [ ] `_track_matches` (ISRC-on-ref-absent-on-track -> title fallback) lacks a unit test.
- [ ] Discogs track-level recording corroboration: build a track-title FTS in the cehbz/discogs mirror (we own it), then `DiscogsMetadata.recordings_for` can discover tracks by artist+title (a track has a `(release, position)` identity, no global recording id). Deferred until valuable.
- [ ] Artist-first discovery: resolve the artist, then fuzzy-match the title within that artist's (bounded) discography, for title-variant recall. Both the MB and Discogs mirror adapters currently use title-FTS + artist filter.
- [ ] FK-direct Discogs fetch: when an MB album carries `discogs_master_id`, look up the Discogs master by primary key (`DiscogsMetadata.album_by_master_id`) instead of relying on title-FTS + exact-artist re-discovery — the FK is the reliable reconciliation signal, currently used only as a comparison key.
- [ ] Discogs format -> `ReleaseTrait` mapping (Live / Compilation) so a Discogs-only album can be filtered by `NotLive` / `NotCompilation` (Discogs-only albums currently carry no traits).

## Other

- [ ] Golden JSON back-compat reader (`traits` else legacy `secondary_types`) — only if external pre-`ReleaseTrait` golden files need to load.
- [ ] Integration test for the recording `resolve`/`emit` realize path. (The MB/Discogs HTTP-shape tests are obsolete; the mirrors have their own live tests.)
