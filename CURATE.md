# CURATE — the curate protocol

**Curate** resolves each item in a linted `intent.md` to a catalog identity, records the
pick (or the absence) in a **selections JSON**, then one `materialize-golden` run emits the
Golden Master + curate report. There is no deterministic intent parser — the LLM agent
drives the catalog CLI tools directly; `materialize-golden` is the only code in the loop.

---

## Per-item resolution recipe

Try in order; stop at the first that resolves. Every attempt's outcome (including
misses) is worth a line in the eventual `marginal` note if the final pick isn't the
obvious first try.

### 1. Classical items (a `composer:` credit is present)

`resolve-performance --mb-only`, **the intent's credits passed verbatim** — one
`--credit role:name` per credit bullet on the item, unchanged (don't re-decompose,
don't drop a force):

```bash
tidalist resolve-performance --mb-only \
  --work "Missa Papae Marcelli" \
  --credit composer:"Giovanni Pierluigi da Palestrina" \
  --credit conductor:"Peter Phillips" \
  --credit orchestra:"The Tallis Scholars"
```

- `outcome: "captured"` → one performance, full credit set matched → take it.
- `outcome: "candidates"` → apply the [picking rules](#picking-rules) below.
- `outcome: "absent"` → retry `--work` with a title variant (drop a subtitle, try the
  original-language title, try a catalog number). Still nothing → fall through to
  `find-album`/`find-recording` with the same credits, then to absent.

`--mb-only` skips Discogs discovery (minutes-scale for a prolific composer); curate
never needs Discogs cross-source High, only the MB spine. But the MB-only path never
reconciles a `discogs_master_id` — when the pick needs Discogs corroboration or
edition identity, re-run without `--mb-only` (interactive: ~6-10s) or use
`find-album`.

### 2. Non-classical album items

`find-album --title … [--credit role:name] [--year]` — try the stated title, then
title variants (subtitle dropped, "Best of"/"Complete" phrasing changed) before giving
up.

### 3. Recording (track) items

`find-recording --title … --credit artist:name [--isrc]`, or `--work …` when the
item is keyed by composition rather than a plain title.

### 4. Absent

Every fallback tried and nothing resolved → record an absent selection (schema below)
with a `marginal` note listing what was tried and why each miss doesn't count — this
keeps the GM reviewable instead of silently dropping the item.

---

## The selections JSON schema

One document per curate run, consumed by `materialize-golden`.

```json
{
  "name": "<playlist name, from intent's H1>",
  "brief": {"criteria": []},
  "selections": [ /* one entry per intent item, same order */ ]
}
```

`brief.criteria` mirrors the intent's `Brief:` line (`studio`, `not_live`,
`not_compilation`, `performed_by`); Go accepts both the markdown `no_*` and the golden
`not_*` spelling — write `not_*` so selections and the emitted GM agree.

### Resolved — album

```json
{
  "kind": "album",
  "rg_mbid": "8ec74528-b1d4-3bbb-8990-87b7ff9e24c3",
  "discogs_master_id": 593829,
  "provenance": {"source": "<intent source id>", "note": "<the intent item's note/cue>"},
  "marginal": "<only when the pick deviates from the obvious cue — omit otherwise>"
}
```

### Resolved — track

```json
{
  "kind": "track",
  "recording_mbid": "<recording MBID>",
  "provenance": {"source": "<intent source id>", "note": "<cue>"}
}
```

`rg_mbid` (album) / `recording_mbid` (track) is the only required identity field; add
`discogs_master_id` only when `resolve-performance`/`find-*` actually reconciled one —
never invent it to look more resolved. `edition`/`criteria` per-item overrides are
optional (see `curate/materialize.go`'s `Selection`).

### Absent

No `rg_mbid`/`recording_mbid` — carries the reviewable stub identity instead:

```json
{
  "kind": "album",
  "artist": "<best-guess performer string from the intent>",
  "title": "<best-guess title from the intent>",
  "provenance": {"source": "<intent source id>", "note": "<cue>"},
  "marginal": "absent: <what was tried (commands/title variants) and why each missed>"
}
```

`materialize-golden` turns this into a rejected-but-reviewable GM entry (never a silent
drop) — the `marginal` string is the only record of why, so it must name the specific
attempts, not just say "not found".

---

## Picking rules

- **Year-cue proximity.** Among `candidates`, prefer the performance/album nearest the
  intent's stated or cued year. `resolve-performance`'s own `--year` selector already
  does this for the performance path; for `find-album` candidates, compare `year`
  fields by hand.
- **Never substitute a vintage.** A different pressing/reissue of the *same*
  performance is fine; a *different* performance presented as "close enough" on year
  is not — pick the nearest genuine match and note the gap in `marginal`, or go absent.
- **Never substitute a sibling work.** The catalog's resolvers rank and recover
  aggressively (alias tables, performer-discography fallback) but never silently
  swap the work identity. If the best available result is for a work other than the
  one requested, that is not a pick — see the mandatory cross-check below before
  accepting anything from the fallback path, and go absent (with a `marginal` note)
  if the check fails.
- **Record `marginal` whenever the pick isn't the unadorned first try**: a year gap,
  a compilation standing in for a single-work recording, overriding
  `resolve-performance`'s own candidate order on judgment. Silence means "resolved
  exactly as cued"; every deviation gets a sentence.

---

## MANDATORY: performer-fallback cross-check

`resolve-performance` and `find-recording --work` both resolve a `--work`, but the
signal lives at different JSON paths and covers different value sets — don't conflate
them:

- **`resolve-performance`**: each entry in `performances[]` carries
  `performances[].work.work_resolution`: `"title"`, `"alias"`, or
  `"performer-fallback"`. On the fallback path the result also appends a matching
  top-level `warnings[]` entry ("work resolved via performer-discography fallback;
  cross-check the work identity…") — key on either the per-performance field or the
  warning text, whichever is easier to check.
- **`find-recording --work`**: the signal is a single top-level `work_resolution`
  field, values `"title"` or `"alias"` **only**. This path never calls the
  performer-discography fallback, so `"performer-fallback"` cannot appear here —
  there is nothing to cross-check on this path beyond the usual title/alias trust.

`"performer-fallback"` means the work-group was **not** found by title or alias at
all — it was reconstructed from the requested performer's own discography
(`workGroupFromPerformers`, `catalog/work.go`), because the title-resolved group held
none of that performer's recordings.

**Before accepting a `resolve-performance` result carrying
`performances[].work.work_resolution: "performer-fallback"` (or a `warnings[]` entry
mentioning "performer-discography fallback"), verify it against the intent's work by a
distinctive token** — the saint's name, the nickname, a movement/subtitle, an
opus/catalog number distinct from a sibling's. Do not accept on title-token overlap
alone.

This is not defensive boilerplate: **same-composer sibling works are exactly the case
the performer-fallback arc cannot discriminate.** The fallback ranks candidate roots by
the performer's recording mass, filtered only by composer identity and *loose*
title-token overlap (`albumMatchesWork` — share the non-digit form word(s); share a
digit token only if the work has one) — a conductor who has recorded both a
Matthäus-Passion and a Johannes-Passion, or both a Symphonie fantastique and a Roméo et
Juliette, shares composer and generic form tokens across siblings. Nothing in the
fallback path itself tells them apart.

```bash
tidalist resolve-performance --mb-only \
  --work "St Matthew Passion" \
  --credit composer:"Johann Sebastian Bach" \
  --credit conductor:"Nikolaus Harnoncourt"
# -> check the returned work.name / work.mbid for "Matthäus", not "Johannes",
#    before accepting — even if performances[].work.work_resolution ==
#    "performer-fallback" and the outcome is otherwise "captured".
```

If the cross-check fails (the resolved work is the wrong sibling, or ambiguous),
do not accept: retry with a title variant closer to the alias table's stored form
(quoted example in `catalog/work.go`'s `workAliasCandidates` doc comment), or record
the item absent with a `marginal` note naming the sibling collision.

`"title"` and `"alias"` resolutions do not require this cross-check — both are anchored
to the requested title text itself (directly, or via a `work_alias` row that is itself
a title match), not merely to a performer's discography.
