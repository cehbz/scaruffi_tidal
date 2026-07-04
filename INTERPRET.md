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
