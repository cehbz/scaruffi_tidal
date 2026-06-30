# Structure reading — Scaruffi Classical Discography

Source acquired with:

```bash
node ~/projects/defuddle/dist/cli.js parse examples/classical.html --md
```

Defuddle yields 268 clean `Composer: Work` / `Recommended recording:` pairs, one
recommendation per work, separated by blank lines (the HTML `<br><br>`).

- **unit:** one recommended recording per `"Composer: Work"` entry.
- **delimiter:** a blank line (HTML `<br><br>`) between entries; within an entry the
  `"Recommended recording:"` line immediately follows the `"Composer: Work"` line.
- **fields:**
  - `title:`     `"<Composer>: <Work>"` — the verbatim source heading.
  - `composer:`  the text before the first `":"`, enriched to the full name by world
                 knowledge (`Schumann` → `Robert Schumann`).
  - `work:`      the text after the first `":"`.
  - performers:  the compound string after `"Recommended recording:"` —
                 **decompose by world knowledge** into role-tagged credits:
                 `conductor` / `soloist (+instrument)` / `orchestra` / `chorus`.
                 Delimiters seen: `&`, `,`, `and`, `with` — all read as "and also".
                 An ensemble that *is* the performing body (e.g. The Tallis Scholars) is
                 tagged `orchestra`; its director is `conductor`.
  - `year:`      a trailing `"(YYYY)"` when present; a *range* like `"(1975-77)"` is left
                 in the provenance `note:` (the `year` field is a single integer).
- **alternates:** secondary picks appear as `"(also …)"` or `"X or Y"`. They are routed
  into the provenance `note:`, not promoted to separate items.
- **brief:** none — Scaruffi's single pick per work *is* the discrimination, so there is no
  playlist-wide criteria line.

## Provenance

Each emitted item carries one `note:` preserving the verbatim Scaruffi recommendation line
(including any `"(also …)"` alternate), so the decomposition is always auditable back to
the source.

## Sampling depth (agreed)

The full source is 268 entries. This demonstration interprets a **representative sample**
(9 entries) chosen to exercise every decomposition mechanism rather than all 268:

- compound `soloist + conductor + orchestra` with an alternate → `note:`
  (Schumann concerto);
- `conductor + 2 soloists (soprano/baritone) + orchestra + chorus`
  (Brahms *Ein Deutsches Requiem*);
- ensemble-as-orchestra + its director (Palestrina *Missa Papae Marcelli*);
- `conductor + chorus + orchestra` (Mozart *Requiem*);
- bare soloist with instrument + alternates (Bach *Goldberg Variations*);
- a box-set `album` with a year *range* (Beethoven *The Nine Symphonies*);
- three soloists on three instruments with `,`/`and`/`with` delimiters
  (Beethoven *Triple Concerto*);
- `conductor + orchestra`, no year (Mahler *Symphony no.8*);
- `conductor + orchestra` with multiple `(also …)` alternates (Dvořák *New World*).
