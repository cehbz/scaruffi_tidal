# TODO

Active work only. Completed work is in git history; architecture/design in `docs/superpowers/`.

## Redesign (2026-06): intent → curate → render, Go reimplementation

Specs (gitignored): `docs/superpowers/specs/2026-06-28-intent-curate-render-design.md` (overall) + `2026-06-28-cli-grammar-intent-schema-design.md` (CLI grammar + intent markdown). Work is on the unmerged `slice1-credit-set-model` branch (git log is the record). The full pipeline runs end-to-end unattended (2026-07-02 measured run: `docs/superpowers/runs/2026-07-02-scaruffi-e2e/RUN.md`). **Slices 1–6 complete** (render rename, `diff`/`sync` productized dry-run-validated live, descriptor discovery 4A+4B shipped 2026-07-03).

- [ ] Apply the Scaruffi sync plan: `tidalist sync … --apply` (additive; 1,972 additions, 0 removals; from the 2026-07-04 full re-render with the shipped render mechanisms — `docs/superpowers/runs/2026-07-04-render-search-quality/delta-final.json` supersedes the 2026-07-03 1,715-addition plan) — user decision, not agent-initiated.

### Performance resolution — open follow-ups

**Decision (2026-07-02): keep the existing GM model (`Album | Recording`).** The Work/Performance core-model collapse and the unified derived DB were considered and deferred (not declined; the questions could not be settled now). Scaruffi items are Album entries; Winwood is a Recording with a loose spec (`studio`, prefer earliest performance and release, no pin).
- [ ] **Candidate-generation coverage residual.** Candidates are (master, credited release) pairs from the performer intersection: a constraint credited only on releases *outside* the credited release of a master is missed, and masterless Discogs releases are not candidates (the primitive's unit is the master). Full credit-anywhere at scale stays build-time-concordance scope.
- [ ] Intersection semantics trade recorded: partial-credit Discogs albums (orchestra not separately credited) aren't candidates → those performances stay MB-only Medium. Revisit only if real Medium admissions prove to need the partial-credit High path.
- [ ] **Explicit Work/Performance domain model** (first-class `core` entities) — **DEFERRED 2026-07-02: keeping `Album | Recording`; revisit if a consumer needs it.** Hierarchy: **Work** (owns composer, composed-year, and recursively movement sub-Works — NOT tracks) → **Performance** (a performance OF a Work by performers; owns the forces conductor/orchestra/soloist/chorus + the pop "artist", and performed-year; is NOT a Work) → **Album** (a collection of Performances — usually 1, sometimes several; owns album/session credits: producer/engineer/mixer/mastering) → **Release/Edition** (a publication of an Album; owns label/catno/format/country/release-year + the tracklist; a Track is a Release-level slot; 1 Album → many Releases). Two payoffs it delivers: (1) **three distinct years** — composed / performed / released — so "discriminate performances by first-release date, not reissue year" = use `Performance.performed` (≈ earliest Release year), never a later Release's year; (2) the **role vocabulary partitions onto owners** (composer→Work; forces→Performance; production→Album) instead of one flat `core.Credits` bag. Reconstructed from mirror evidence at the provider boundary (neither DB models Work first-class), clean in `core`. Revisits the "Work is a discovery input, not stored" decision (made before performance-resolution existed). Open: does `Performance` subsume today's `Recording` golden unit (→ golden = `Performance | Album`)? name for a Work's components (Movement = a sub-Work)? Home: the slice-4 curate boundary (where golden units get stored); modelling it makes the grain-check *structural* (manipulate `Performance`, can't slip to track rows). (Deletable elaboration: `docs/superpowers/specs/2026-07-01-domain-model-work-performance.md`.)
- [ ] Build-time materialised concordance table (corpus-scale batch): the query path resolves on-the-fly via the `artist.discogs_artist_id` bridge, so this is a scale optimisation. Trigger: resolving a whole Scaruffi corpus at once; the *right* home for Discogs work-group reconstruction. Refresh tied to the mirror rebuild. (2026-07-02: the discussed generalization, a full domain-model-shaped derived DB fusing MB+Discogs with per-field provenance, is the same idea at larger scope; deferred with the domain model.)
### Classical resolution — open refinements
- [ ] **Same-composer sibling works are flagged, not discriminated**: a `performer-fallback` (or ambiguous) resolution can land on a sibling (Matthäus/Johannes); arcs cannot discriminate same-composer same-form works, so the mandatory cross-check in `CURATE.md` (keyed on `work_resolution`/warnings) is the control. A deterministic discriminator (prefix-stemmed distinctive tokens with the full must-pass/must-reject matrix) stays open only if flagged-weak proves insufficient in practice.
### Render search quality — residual (mechanisms shipped 2026-07-04)
The four 2026-07-03 mechanisms (fold, credit variants, anchor generation, VA survivor) plus the track-fallback verification gate and the composer-conditioned `work_alias` harvest are shipped; full-corpus re-render measured 24/41 prior gaps converted, zero weak-quality items, 34 honest gaps (`docs/superpowers/runs/2026-07-04-render-search-quality/RUN.md`). Remaining, by mechanism:
- [ ] **Title-variant materialization** (German↔English work wording: "Symphonie Nr. 7" vs "Symphony No. 7", indices 78/20/24/34-class): harvest `work_alias` *title* forms at materialize time — requires resolving each album's work-group at curate; the known next mechanism if the German-wording class dominates the 34-gap residual. Leading collector-prefix titles (index 24) have no generic rule; candidate: composer-led-title anchor or LLM rephrase.
- [ ] **Anchor-budget tuning bucket**: `_credit_forms` dedups by `casefold` not `_fold` (fold-equal punctuation variants burn cap slots); the `_MAX_CREDIT_QUERIES` cap applies before yield-time dedup (same person as conductor+artist burns two slots); only the first composer credit joins two-credit combos (two-composer couplings miss the second). No measured starvation on this corpus — tune together when the residual warrants.
- [ ] Go `core.NormalizeName` omits U+2010 (the GM carries it, e.g. "Yo‐Yo Ma"); the Python render fold compensates. Add U+2010 to `punctuationFolder` and re-run the Go credit-match live gate if a curate-side match ever misses on it.
- [ ] `nameVariantsForRole`'s alias query lacks `ORDER BY` — which-4-under-cap variants can flip across mirror rebuilds (spurious GM diffs). One-line `ORDER BY name` for a hard determinism guarantee.
- [ ] Track credits (`m2TrackEntry`) carry no `variants` — matters only if Recording-entry rendering ever anchors per-credit; align with the album path then.
- [ ] `_credit_from_dict` accepts malformed `variants` (bare string → per-char tuple would poison `_artist_match_album`; non-string entries crash at render). Unreachable from the Go writer; validate list-of-str if hand-edited GMs enter scope.
- [ ] materialize-golden now ≈7.5 min on the 267-item corpus (was 2.9 s pre-variants; 14.5 min unmemoized): residual is per-unique-key artist FTS resolution in `catalog` (1,216 unique (role,name) keys). Batch or precompute if curate wall time matters.
- [ ] B1 EXPLAIN test only rules out `SCAN work_alias`, not `SCAN l_artist_work`; the 30 s latency budget is the backstop. One-line strengthening.
- [ ] **Post-render LLM audit pass**: for each rendered entry (survivor album or track-fallback assembly), an LLM judges the platform result against the golden credits + canonical tracklist and emits a typed verdict (`confirmed | suspect | fabrication`) with cited evidence (title/credit/tracklist agreement, ref duplication, quality grades). Mechanizes the hand audit that caught the 36/103 fabrications (`runs/2026-07-04-render-search-quality/RUN.md`); complements the structural assembly gate by catching classes the deterministic matchers can't (their documented ceiling — the realize-stage LLM-judge item below is the selection-time counterpart). Batch, read-only, post-render; verdicts feed the diff/sync review.

### Review follow-ups (2026-07-03 whole-branch triage)
- [ ] Extract a shared `climbToRoot` helper for the duplicated bounded-ascent loop (`resolveWorkGroups` step (c) vs `workGroupFromPerformers`, catalog/work.go).
- [ ] Test gaps: `filterByLabel` name-equality fallback path; `matchedForces` non-conductor umbrella branches; repeated `- edition:` bullet merge in intent parse.
- [ ] `- edition: label=` (empty cue value) is silently dropped — emit a diagnostic like unknown keys.
- [ ] `ReportItem.ID` carries a bare Discogs master id next to MBIDs in the same report section — consider a `master:` prefix.
- [ ] Discogs-only album selections still run the always-empty `AlbumByRG("")` query first — reorder the branch.
- [ ] Normalize the `match{}` grammar: `find-by-attributes` omits `year_match` when no window; `find-album` emits `null`. Pick one.
- [ ] `TidalPlatform.playlist_tracks` termination assumes a firm 100-item server page; a mid-list short page would truncate silently.
- [ ] `sync --apply` mid-failure output carries neither the DRY RUN banner nor the "applied." tail — print the plan after applying, or an explicit failure marker.
- [ ] README: verb list still omits `diff`/`sync`; add usage examples for both.

### Go catalog — follow-ups
- [ ] **Discogs mirror rebuild must re-run the full ANALYZE (stat4)** or the unpinned `dc.track` queries regress to the idx_track_parent walk (~6 min): the 2026-07-03 stat4 experiment landed histograms on the live mirror and the pins were dropped (`tracksFor`, `track_count`); the durable writer step belongs in the cehbz/discogs importer (its repo TODO); tidalist's `//go:build integration` latency gates are the regression guard.

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
