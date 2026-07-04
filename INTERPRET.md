# INTERPRET — the two-pass interpret method

**Interpret** turns a source into a lint-clean, role-tagged `intent.md` that the curate
stage consumes. The LLM does the interpreting; the only code is `tidalist lint-intent`.

Sources accepted: a URL (fetched via `defuddle`), a saved page, a natural-language brief,
or a plain list. The method is source-general — no source-specific rules.

---

## The procedure

Two passes — *infer structure*, then *extract against it* — closed by an assessment loop
that iterates until coverage is complete, lint is clean, and the spot-check resolves at
the expected rate.

### Pass 0 — Acquire

One of:
- Read a local file directly.
- `defuddle parse <url> --md` → clean markdown to stdout (see [defuddle](#defuddle) below).
- Take a natural-language brief or list directly from the user.

### Pass 1 — Infer structure → explicit artifact

Read the whole source. Sample a representative span if it is very large. Emit a
[structure reading](#the-structure-reading-artifact) and **present it for confirmation
before extracting.** Do not parse per-item yet.

Catching a misread at this stage is far cheaper than correcting N mis-extracted items.

### Pass 2 — Extract (structure-guided)

Apply the confirmed structure *uniformly* to every item. The single global structure pass
enforces that item 1 and item 267 are parsed the same way — per-item ad-hoc reasoning
drifts.

- **Decompose compound credit strings into role-tagged credits using world knowledge** —
  the irreducibly-LLM step. E.g. `"Kempff & Krips & London Symphony Orchestra"` →
  `soloist: Wilhelm Kempff (piano)` / `conductor: Josef Krips` /
  `orchestra: London Symphony Orchestra`.
- Route alternates to `note:` or to additional items per the structure.
- Label / catalog-number / vintage-year cues attached to a recommendation (e.g. "DG
  2530 516, 1975") go to `edition: label=…, catno=…, year=…`, not into `note:`.
- Map playlist-wide signals to the `Brief:` line.
- Emit intent markdown per the [intent schema](#the-intent-schema).

### Pass 3 — Lint

```bash
tidalist lint-intent --write intent.md
```

Resolve any unknown-vocabulary or structural diagnostics before proceeding.

### Pass 4 — Assess & iterate ("how did we do?")

Judge against three signals. If unsatisfactory, **revise the structure reading and re-run
passes 2–3** — the structure reading is the single artifact to iterate on.

- **Coverage** — extracted item count (from `lint-intent`'s summary line) vs the source's
  expected unit count. A shortfall means the structure missed a variant (e.g. a work with
  two `Recommended recording:` lines, or alternates mis-split).
- **Lint cleanliness** — *recurring* diagnostics of the same shape signal a systematic
  misread, not a one-off (e.g. every soloist attribute malformed ⇒ the pass-2 paren rule
  is wrong).
- **Resolution spot-check** — sample items through the catalog tools; a *systematic*
  cluster of misses (not random absence) implies mis-decomposition — e.g. an orchestra
  tagged as conductor — and points back at a specific structure-reading rule.

Converge when coverage is complete, lint is clean, and the spot-check resolves at the
expected rate. The converged intent is the slice's deliverable; curate (slice 4) consumes
it.

---

## Descriptor briefs (discover-by-descriptor)

A **descriptor brief** is not a source that names items — it's a *predicate* ("dark gothic
trance, at least 5 minutes"). There is nothing to extract yet; the candidate set has to be
**generated** before Pass 2's "apply the structure uniformly" even makes sense. This is Pass
1's extra job for a descriptor brief: the structure reading doubles as a generation plan.

**Materialize, don't go live.** Generation runs once. Its output is named, ordinary intent
items — after that point this is an ordinary resolve-by-identity document, not a
standing filter. The predicate itself doesn't need to be re-evaluated on every future
resolution; it already did its job picking these items out of the whole catalog. What
survives is **provenance**, not a live constraint: the predicate in the playlist name (H1)
and, per item, a `note: descriptor: <clause(s) it satisfies>` bullet. If a clause happens to
coincide with the closed `Brief:` criteria vocabulary (`studio`, `no-compilation`,
`no-live`, `performed-by:`) — e.g. "album-oriented, the canonical records" implying "not a
live album, not a compilation" — put it on `Brief:` too, since that part *does* keep
constraining resolution. But don't force a genre/style/era clause onto `Brief:`: those
tokens are lint-validated against the closed set, and free descriptor text there is a lint
error, not a warning.

**Decompose the predicate before generating**, into three kinds of clause, each with its own
tool:

- **Categorical** (genre, style, year) → `tidalist find-by-attributes --style … --genre …
  --year-from … --year-to …`.
- **Quantitative** (duration, track count) → the catalog has no duration filter of its own;
  compose it yourself: shortlist candidates categorically, then `tidalist tracklist --master
  <id>` per candidate, and filter by the returned track lengths.
- **Subjective** ("dark", "best", "the canonical records") → your judgment. No catalog query
  resolves canonicity or mood; this is the irreducibly-LLM part of the generation step.

**Generate in two passes:**

- **4A (always).** Propose candidates from your own knowledge — this is what answers the
  subjective clauses, and for a well-known descriptor it's usually most of the list.
- **4B (recall extension).** Sweep the catalog with `find-by-attributes` on the categorical
  clauses (checking the exact style/genre string against Discogs vocabulary — a plausible
  term like "Gothic Rock" can return `{"candidates":[]}` while "Goth Rock" is the real one;
  try both spellings before concluding a style has no matches). Use the sweep two ways: to
  **surface** candidates 4A's memory missed (the obscure or regional acts that never made it
  into general knowledge), and to **corroborate** 4A's picks with a real
  `discogs_master_id` — cite it in the item's `note:` as provenance. The sweep alone is not
  sufficient: it returns every genuine tag match with no ranking by importance, so 4A's
  judgment is still what filters it down to "canonical."

Every generated item carries `note: descriptor: <the predicate clause(s) it satisfies>` —
this is the auditable link back to the brief, the same role Scaruffi's provenance `note:`
plays for a "(also …)" alternate.

**After generation, nothing about the pipeline changes.** The generated items are ordinary
named intent entries; Pass 3 (`lint-intent`) and Pass 4 (coverage / lint cleanliness /
resolution spot-check) apply exactly as written above, with "coverage" now read against the
generation plan's candidate count rather than a source's unit count. Curate (slice 4) needs
no special handling — it never sees the predicate, only the materialized items and their
provenance notes.

See the worked example: `examples/descriptor-brief.md` (the NL predicate),
`examples/descriptor-structure.md` (the structure-reading-as-generation-plan, including the
actual `find-by-attributes` sweep commands and hit counts), and
`examples/descriptor-intent.md` (eight canonical dark-gothic-post-punk albums, 1979–1986,
every one independently corroborated in the mirror).

---

## The structure-reading artifact

Lightweight markdown, **human-confirmed, not machine-parsed** (`lint-intent` never reads
it — it touches only `intent.md`). It captures, at minimum:

- **unit** — what constitutes one item.
- **delimiter** — how items are separated.
- **fields** — which intent field each source datum maps to, and explicitly how *compound
  credits* are to be decomposed by knowledge into roles.
- **alternates** — how secondary recommendations appear and where they go (`note:` vs
  item).
- **brief** — any playlist-wide criteria signal (or "none").

Scaruffi example (the hard case — compound classical performers):

```markdown
# Structure reading — Scaruffi Classical Discography
- unit: one recommended recording per "Composer: Work" entry
- delimiter: blank line (HTML <br><br>) between entries
- fields:
  - title:      "<Composer>: <Work>"
  - composer:   text before the first ":"
  - work:       text after the first ":"
  - performers: compound after "Recommended recording:" —
                decompose by knowledge → conductor / soloist(+instrument) / orchestra / chorus
  - year:       trailing "(YYYY)" when present
  - edition:    label/catno/year cues on the recommendation (e.g. "DG 2530 516,
                1975") → `edition: label=DG, catno=2530 516, year=1975`
- alternates: "X or Y" and "(also …)" → note:
- brief: none (Scaruffi's single pick is the discrimination)
```

Winwood collapses to a few lines (unit = one track or album named in prose; field =
`artist`; brief from the stated constraints). **Same artifact shape, opposite structures**
— that two structurally-opposite sources are handled by one unchanged method is the
generality test that guards against re-overfitting to Scaruffi.

---

## The intent schema

The markdown schema is defined in
`docs/superpowers/specs/2026-06-28-cli-grammar-intent-schema-design.md` §3.

Structure: an `# H1` playlist name; an optional `Brief:` line of playlist-wide criteria
(`;`-separated); then one `## H2` section per item with the heading
`## <title> · <kind>`, `kind ∈ {album, recording}`. Each field is a bullet
`- <key>: <value>`; credits are repeatable (`- soloist: Emma Kirkby (soprano)`).

The **role vocabulary** is the 10 `core.Role` values: `composer`, `conductor`, `soloist`,
`orchestra`, `chorus`, `chorus_master`, `artist`, `producer`, `engineer`, `mastering`.
`lint-intent` validates against this closed set; the existing `--credit` flag uses the
same source of truth.

---

## defuddle

`defuddle parse <url> --md` fetches a URL and emits clean markdown to stdout. It is the
[Defuddle CLI](https://github.com/kepano/defuddle) — the extraction engine behind the
Obsidian Web Clipper — running from the user's `~/projects/defuddle` fork, exposed on
PATH via `npm link` after `npm run build`.

```bash
defuddle parse https://www.scaruffi.com/classical/ --md
```

Best suited for article-like pages. Two classes of source require the fallback instead:

- **JS-rendered SPAs** — no static content to fetch.
- **Malformed HTML** — orphan or premature close tags can collapse Defuddle's extraction
  root to a stub.

**Fallback (always available):** save the page and read the file directly. A JS-heavy,
mangled, or non-article source uses this path; it is a first-class input.

`defuddle` is **not a tidalist build dependency** — tidalist never calls it; the agent
does. A clone without defuddle still works via the fallback.
