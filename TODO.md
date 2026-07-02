# TODO

Active work only. Completed work is in git history; architecture/design in `docs/superpowers/`.

## Redesign (2026-06): intent → curate → render, Go reimplementation

Specs (gitignored): `docs/superpowers/specs/2026-06-28-intent-curate-render-design.md` (overall) + `2026-06-28-cli-grammar-intent-schema-design.md` (CLI grammar + intent markdown). Work is on the unmerged `slice1-credit-set-model` branch (slices 1/1b/2a–2d landed there; git log is the record).

- [ ] Slice 4: curate best-effort + report (LLM resolves identity, deterministic type-aware criteria gate admission, marginal-confidence flagged).
- [ ] Slice 5: render (rename realize→render) + diff/sync — the Python Tidal tool behind the JSON seam.
- [ ] Slice 6: descriptor discovery (4A LLM-knowledge materialize; 4B catalog attribute-queries — high value).
- Performance resolution — **MB spine SHIPPED** (slice-2e, `catalog.ResolvePerformance` + `resolve-performance` CLI). Composer-aware work-group resolution (`l_work_work` 281, retires top-1 `resolveWorkID`), conjunctive performer-credit selection, first-co-release clustering, three-outcome exactness gate, MB↔Discogs reconciliation + confidence grading. Live-proven: the classical compounds that came back empty (Brahms Requiem, Palestrina, Beethoven 5) now resolve. Spec: `docs/superpowers/specs/2026-07-01-performance-resolution-design.md`.

### Performance resolution — follow-ups (slice-2e MB spine shipped)

**Decision (2026-07-02): keep the existing GM model (`Album | Recording`).** The Work/Performance core-model collapse and the unified derived DB were considered and deferred (not declined; the questions could not be settled now). Scaruffi items are Album entries; Winwood is a Recording with a loose spec (`studio`, prefer earliest performance and release, no pin). Consequently the Discogs cross-source High redesign drops off the slice-4 critical path: classical is admitted at Medium via the fast MB spine (composer-omitted or composer-only), and the expensive composer+performer path is a deferred confidence enhancement.
- Query-time Discogs discovery is **not viable interactively for a prolific composer** (the full-scan hang is fixed, but see the measured latency below). `discogsPerformances` candidate generation is index-driven, release-level: drive from the composer's `release_artist` credits (`idx_release_artist_artist`) + a release-level performer `EXISTS`; no more `FROM dc.master JOIN dc.release` scan, no `track_artist` arms (`track_artist.artist_id` is unindexed on the mirror). Measured on the real mirror (X10 over USB3): the Beethoven+Kleiber+VPO composer+performer query runs ~13 min cold and ~13 min warm. Warm does not help, so the cost is the work volume (iterating Beethoven's ~100k `release_artist` credits plus the 79-way per-candidate N+1), not cold I/O. This is a MINIMUM fix. Remaining work:
  - [ ] **Real grain-correct redesign** (supersedes the minimum fix). Candidate-generate from the **performers** (conductor ∩ orchestra) at release level — the album's forces, far more selective than a prolific composer; drive from the least-frequent constraint artist. Then per-candidate **reconstruct the work** from track evidence — the contiguous track-group whose titles match the work — and confirm **that group's** composer. Rationale: the composer is the composer of the WORK, a Work-level fact redundantly stamped on tracks; reconstruct the work, don't reify the track. This is correct on **multi-composer albums** (one conductor/orchestra, several composers) and kills the Brahms-vs-Mahler-5th trap that a coarse "composer appears anywhere on the album" false-matches. Decision to make: candidate = conductor ∩ orchestra (intersection → all reconciled albums grade High; partial-credit Discogs albums simply aren't candidates → those performances stay MB-only Medium; simpler + faster) vs ≥1-performer (catches partial-credit albums but a larger candidate set + OR-drive complexity). This per-candidate work-group reconstruction is *exactly* what the build-time concordance would precompute offline — they converge. (Deletable elaboration: `docs/superpowers/specs/2026-07-01-discogs-discovery-real-design.md`.)
  - [x] **Real-data corroboration, verified 2026-07-02.** MB↔Discogs reconciliation fires on real anchors. Beethoven 5 / Kleiber / Wiener Philharmoniker produced HIGH=2 (the canonical DG 2530 516, master 287096, graded High because both forces are a subset of the album artist ids), medium=2, low=75. Bridge ids, work-group, MB clusters, and Discogs candidate-gen all resolve. Two matching smells for the redesign: greedy first-match reconcile can mispair a cluster to a master (the year=0 MB cluster took the DG-1975 master, while the real 1975 cluster took a London master at Medium), and the `EXISTS(>=1 performer)` union surfaces ~75 other-conductor and reissue masters as Low noise (the intersection design collapses these).
  - [ ] **Latency, the blocker: measured ~13 min cold and ~13 min warm, not ~25s.** The composer-driven candidate-gen iterates the prolific composer's ~100k `release_artist` credits and runs a per-candidate N+1; warm does not help. Fix is the grain-correct redesign above: drive from the least-frequent constraint artist (the performers are far more selective), which shrinks both the driving scan and the candidate set. Touches the >=1-vs-intersection semantics and confidence grading.
  - [ ] **Release-level only.** A composer/performer credited *only* at track level is missed (the composer is a Work-level fact stored redundantly on tracks — a data quirk). Full credit-anywhere at scale belongs in the build-time concordance.
- [ ] **Explicit Work/Performance domain model** (first-class `core` entities) — **DEFERRED 2026-07-02: keeping `Album | Recording`; revisit if a consumer needs it.** Hierarchy: **Work** (owns composer, composed-year, and recursively movement sub-Works — NOT tracks) → **Performance** (a performance OF a Work by performers; owns the forces conductor/orchestra/soloist/chorus + the pop "artist", and performed-year; is NOT a Work) → **Album** (a collection of Performances — usually 1, sometimes several; owns album/session credits: producer/engineer/mixer/mastering) → **Release/Edition** (a publication of an Album; owns label/catno/format/country/release-year + the tracklist; a Track is a Release-level slot; 1 Album → many Releases). Two payoffs it delivers: (1) **three distinct years** — composed / performed / released — so "discriminate performances by first-release date, not reissue year" = use `Performance.performed` (≈ earliest Release year), never a later Release's year; (2) the **role vocabulary partitions onto owners** (composer→Work; forces→Performance; production→Album) instead of one flat `core.Credits` bag. Reconstructed from mirror evidence at the provider boundary (neither DB models Work first-class), clean in `core`. Revisits the "Work is a discovery input, not stored" decision (made before performance-resolution existed). Open: does `Performance` subsume today's `Recording` golden unit (→ golden = `Performance | Album`)? name for a Work's components (Movement = a sub-Work)? Home: the slice-4 curate boundary (where golden units get stored); modelling it makes the grain-check *structural* (manipulate `Performance`, can't slip to track rows). (Deletable elaboration: `docs/superpowers/specs/2026-07-01-domain-model-work-performance.md`.)
- [ ] Build-time materialised concordance table (corpus-scale batch): the query path resolves on-the-fly via the `artist.discogs_artist_id` bridge, so this is a scale optimisation. Trigger: resolving a whole Scaruffi corpus at once; the *right* home for Discogs work-group reconstruction. Refresh tied to the mirror rebuild. (2026-07-02: the discussed generalization, a full domain-model-shaped derived DB fusing MB+Discogs with per-field provenance, is the same idea at larger scope; deferred with the domain model.)
- [ ] Interpret `edition` field: extract Scaruffi label/catno/year cues into a structured `edition` field so `resolve-performance` gets `--label`/`--catno`/`--year` from interpret (currently CLI-only).
- [ ] Deferred slice-2e review Minors (per-task Opus reviews; triage in a whole-branch review): `filterByLabel` is name-equality not id-family `sameLabelFamily` (+ stale doc-comment header); `matchedForces` empty under the conductor→chorus_master umbrella (→ candidates not captured); reconcile re-bridges force ids `discogsPerformances` already computed; `discogsRoles` comma-inside-brackets edge; `albumMatchesWork` "no" is a content token; candidate-gen matches over any release but confirms on `main_release_id`; `RW-2` N+1 per candidate.

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
