# TODO

Active work only. Completed work is in git history; architecture/design in `docs/superpowers/`.

## Redesign (2026-06): intent → curate → render, Go reimplementation

Specs (gitignored): `docs/superpowers/specs/2026-06-28-intent-curate-render-design.md` (overall) + `2026-06-28-cli-grammar-intent-schema-design.md` (CLI grammar + intent markdown). Work is on the unmerged `slice1-credit-set-model` branch (git log is the record). The full pipeline runs end-to-end unattended (2026-07-02 measured run: `docs/superpowers/runs/2026-07-02-scaruffi-e2e/RUN.md`).

- [ ] Slice 5 remainder: the realize→render rename; diff/sync productization (`scripts/compare_playlists.py` is the prototype); spot-check the e2e run's 41 reasoned gaps (track-fallback 0/N — genuine platform absence vs per-track search quality); publish/sync flow for the realized Scaruffi playlist.
- [ ] Slice 6: descriptor discovery (4A LLM-knowledge materialize; 4B catalog attribute-queries — high value).
- [ ] Slice-4 follow-ups: a durable Go↔Python golden conformance test (the check was run manually via `from_golden` subprocess); `materialize-golden` accepting Discogs-only album selections (rg-required today — ~5 e2e-run absents traced to it, e.g. Suzuki organ works, Perahia Chopin); per-item curate-report markdown rendering.

### Performance resolution — open follow-ups

**Decision (2026-07-02): keep the existing GM model (`Album | Recording`).** The Work/Performance core-model collapse and the unified derived DB were considered and deferred (not declined; the questions could not be settled now). Scaruffi items are Album entries; Winwood is a Recording with a loose spec (`studio`, prefer earliest performance and release, no pin).
- [ ] **Candidate-generation coverage residual.** Candidates are (master, credited release) pairs from the performer intersection: a constraint credited only on releases *outside* the credited release of a master is missed, and masterless Discogs releases are not candidates (the primitive's unit is the master). Full credit-anywhere at scale stays build-time-concordance scope.
- [ ] Intersection semantics trade recorded: partial-credit Discogs albums (orchestra not separately credited) aren't candidates → those performances stay MB-only Medium. Revisit only if real Medium admissions prove to need the partial-credit High path.
- [ ] **Explicit Work/Performance domain model** (first-class `core` entities) — **DEFERRED 2026-07-02: keeping `Album | Recording`; revisit if a consumer needs it.** Hierarchy: **Work** (owns composer, composed-year, and recursively movement sub-Works — NOT tracks) → **Performance** (a performance OF a Work by performers; owns the forces conductor/orchestra/soloist/chorus + the pop "artist", and performed-year; is NOT a Work) → **Album** (a collection of Performances — usually 1, sometimes several; owns album/session credits: producer/engineer/mixer/mastering) → **Release/Edition** (a publication of an Album; owns label/catno/format/country/release-year + the tracklist; a Track is a Release-level slot; 1 Album → many Releases). Two payoffs it delivers: (1) **three distinct years** — composed / performed / released — so "discriminate performances by first-release date, not reissue year" = use `Performance.performed` (≈ earliest Release year), never a later Release's year; (2) the **role vocabulary partitions onto owners** (composer→Work; forces→Performance; production→Album) instead of one flat `core.Credits` bag. Reconstructed from mirror evidence at the provider boundary (neither DB models Work first-class), clean in `core`. Revisits the "Work is a discovery input, not stored" decision (made before performance-resolution existed). Open: does `Performance` subsume today's `Recording` golden unit (→ golden = `Performance | Album`)? name for a Work's components (Movement = a sub-Work)? Home: the slice-4 curate boundary (where golden units get stored); modelling it makes the grain-check *structural* (manipulate `Performance`, can't slip to track rows). (Deletable elaboration: `docs/superpowers/specs/2026-07-01-domain-model-work-performance.md`.)
- [ ] Build-time materialised concordance table (corpus-scale batch): the query path resolves on-the-fly via the `artist.discogs_artist_id` bridge, so this is a scale optimisation. Trigger: resolving a whole Scaruffi corpus at once; the *right* home for Discogs work-group reconstruction. Refresh tied to the mirror rebuild. (2026-07-02: the discussed generalization, a full domain-model-shaped derived DB fusing MB+Discogs with per-field provenance, is the same idea at larger scope; deferred with the domain model.)
- [ ] Interpret `edition` field: extract Scaruffi label/catno/year cues into a structured `edition` field so `resolve-performance` gets `--label`/`--catno`/`--year` from interpret (currently CLI-only).
- [ ] Deferred slice-2e review Minors (per-task Opus reviews; triage in a whole-branch review): `filterByLabel` is name-equality not id-family `sameLabelFamily` (+ stale doc-comment header); `matchedForces` empty under the conductor→chorus_master umbrella (→ candidates not captured); reconcile re-bridges force ids `discogsPerformances` already computed; `discogsRoles` comma-inside-brackets edge; `albumMatchesWork` "no" is a content token.

### Classical resolution — open refinements
- [ ] **`albumMatchesWork` sibling-work false match**: the ≥1-shared-content-token rule lets the performer fallback substitute a same-form sibling (St Matthew Passion → St John Passion via shared "st"/"passion"). Needs a distinctive-token or majority-overlap rule — but note the design tension: strict-majority coverage breaks the cross-language title-twin rescue ("Goldberg-Variationen" vs "Goldberg Variations" shares one token) and canonical titles with qualifier cruft ("in C minor, op. 67"); the rule needs the full test matrix, likely with prefix-stemming (variations/variationen). (Curate agents caught these by cross-checking; the 2026-07-02 e2e run confirmed the trap fires live — Matthew/John, Fantastique/Roméo.)
- [ ] **find-recording `--credit` filter is not alias-aware** — only `ResolvePerformance`'s matching was upgraded; the same variant-set expansion should back the find-recording filter.
- [ ] **work_alias unconsulted**: work-title resolution is name-FTS only; a query in another language than the recording-rich family's name depends entirely on the performer fallback (e.g. "The Rite of Spring" vs "Le Sacre du printemps"). Consider `work_alias` in candidate generation.
- [ ] Ensemble-name suffix variance: bare "Leningrad Philharmonic" / "Moscow Philharmonic" FTS-resolves to wrong entities (a trio, a choir); the "... Orchestra" suffix is needed. Consider artist-type-aware ranking in `resolveArtistID`. (The 2026-07-02 e2e curate also hit "Berlin Philharmonic"→"Berliner Philharmoniker" misses — agents recovered via `resolve-artist` diagnosis; alias-aware ensemble resolution would remove that friction.)
- [ ] Curate-agent friction from the 2026-07-02 e2e run, candidates for resolver upgrades: composer names surname-first ("Bartók Béla") or native-script primary ("Александр Скрябин") require alias diagnosis before `resolve-performance` accepts them; work-title language variants still depend on the performer fallback (the `work_alias` item above).

### Go catalog — follow-ups
- [ ] Perf: first MB `recording_fts` MATCH ~2.6s cold (mmap of the 55GB file); measure warm vs cold, consider warming for interactive use.
- [ ] **Planner mis-costing of `parent_track_id IS NULL`** — currently mitigated by query pins (unary-`+` in `tracksFor`, CROSS JOIN in the `track_count` join). Facts: `idx_track_parent`'s NULL bucket (~120M of 178M rows) is inexpressible in stat1's single per-index average (sampled ANALYZE wrote 13 rows/key; a full one computes ~4), so `IS NULL` costs as selective with or without stat1. Testable fixes, cheapest first:
  - **stat4 histograms** (expresses the skew stat1 can't): modernc v1.53.0 — tidalist's own driver — is built with `SQLITE_ENABLE_STAT4` (verified in its darwin_arm64 generator flags), so it can both WRITE stat4 (a small Go program running ANALYZE on the mirror) and READ it at query time; the stock system CLI lacks stat4 but can be recompiled with it if a shell-side writer is preferred. Test: stat4-ANALYZE the dc mirror → EXPLAIN the 16-param `tracksFor` shape through modernc → un-pin → `//go:build integration` latency gates.
  - **Hand-authored stat1** for `idx_track_parent` (documented SQLite technique): `UPDATE sqlite_stat1 SET stat='178224810 60000000' WHERE idx='idx_track_parent'` makes `IS NULL` cost honestly; one EXPLAIN to test.
  - **Delete `idx_track_parent`'s stat1 row**: planner defaults for that index only, real stats elsewhere; one EXPLAIN to test.
  - **Partial index** `ON track(release_id) WHERE parent_track_id IS NULL` (discogs repo TODO): encodes the top-level-tracks query directly; needs plan-preference + size measurement.
  Pins come out when one of these passes the un-pinned latency gates; the winning writer step lands in the discogs importer.
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
- [ ] Recording selection sometimes picks a later re-recording (e.g. "I'm a Man" -> 1988); prefer the original. (The performance-resolution design addresses this via first-release-date clustering.) **Winwood is the validation case: `studio` gate plus prefer earliest performance and release, no pin.**
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
