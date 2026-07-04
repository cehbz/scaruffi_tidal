# Structure reading — Descriptor brief: dark gothic post-punk canon

Source: a natural-language **descriptor brief** (`examples/descriptor-brief.md`) — a
*predicate* ("dark gothic post-punk, 1979-1986, album-oriented; the canonical records"),
not a list of named items. Pass 1 for a descriptor brief has an extra step the other
structure readings don't: before "unit / fields" can even apply, the candidate set has to
be **generated** — this section doubles as the generation plan.

- **unit:** one generated candidate album satisfying the predicate.
- **delimiter:** n/a — there is no source item list to delimit; items are generated, not
  segmented out of prose.
- **fields:**
  - `title:`  the album title.
  - `kind:`   `album` (the brief is explicit: "album-oriented").
  - `artist:` the releasing act (pop/rock convention — one `artist` credit, no role
              decomposition).
  - `year:`   the original studio release year.
  - `note:`   `descriptor: dark gothic post-punk` + the specific reason the record is
              canonical + (for every item below — this brief's sweep corroborated all of
              them) `discogs_master_id: <id>` and the query that surfaced it.
- **alternates:** none — each candidate is one item; there is no secondary-pick routing.
- **brief:** here is the one place a descriptor brief structurally diverges from Winwood's
  ordinary playlist-wide criteria. "Studio, not live" and "not a compilation" map onto the
  existing closed criteria vocabulary (`studio`, `no-compilation`) exactly the way Winwood's
  did, so those two tokens go on the `Brief:` line and keep applying at resolution time. But
  "dark gothic post-punk, 1979-1986" is **not** a resolution-time constraint — it already
  did its job during generation (it's what picked these eight albums out of the whole
  catalog) and has no analogue in the closed criteria set (confirmed by testing: putting the
  raw descriptor text on the `Brief:` line trips `lint-intent`'s criteria validator —
  `error: Brief: unknown criterion "dark gothic post-punk"` — because `Brief:` tokens are
  validated against `{studio, no-compilation, no-live, performed-by:<name>}`, not free text).
  So the descriptor clause is preserved as **provenance**, not as a live constraint: once in
  the playlist name (H1), and once per item in `note:`. Curate never needs to re-evaluate
  "is this dark gothic post-punk?" — that question was already answered at generation time.

## Predicate decomposition

- **categorical** (genre/style, year range) → `tidalist find-by-attributes --style … --year-from … --year-to …`.
- **quantitative** (duration): none in this brief — no length constraint was stated. Where
  one is, the pattern is `tidalist find-by-attributes` for the categorical shortlist, then
  `tidalist tracklist --master <id>` per candidate, filtering durations yourself (the catalog
  has no duration-filtered search of its own).
- **subjective** ("dark", "the canonical records"): judgment — no catalog tool resolves
  canonicity. This is exactly what 4A supplies, and exactly why 4B alone (a bare style/year
  sweep) is not sufficient: it returns hundreds of genuine genre matches with no ranking by
  importance.

## Generation plan

**4A (model knowledge, always).** Proposed from world knowledge of the genre and era: Joy
Division (*Unknown Pleasures*, *Closer*), Killing Joke (the 1980 debut), Bauhaus (*In the
Flat Field*), The Cure (*Seventeen Seconds*, *Pornography* — the outer two records of their
"dark trilogy"), Siouxsie and the Banshees (*Juju*), The Sisters of Mercy (*First and Last
and Always*). Judgment calls: The Cure's middle trilogy record, *Faith*, was dropped in
favor of *Seventeen Seconds* + *Pornography* to keep one Cure record at each end of the
trilogy rather than three Cure albums crowding out other acts; no 1986 record made the cut
because the sweep below shows the classic wave clustering 1979-1985, with the scene
splintering into other genres by 1986.

**4B (catalog sweep, recall extension).** Actual commands run against the live mirror
(`/Volumes/Crucial X10/discogs/discogs.db`, mounted, read-only; each query <1s):

```bash
$ tidalist find-by-attributes --style "Gothic Rock" --year-from 1979 --year-to 1986 --limit 25
{"candidates":[]}
```
"Gothic Rock" is not Discogs vocabulary — 0 hits, as flagged as a risk up front.

```bash
$ tidalist find-by-attributes --style "Goth Rock" --year-from 1979 --year-to 1986 --limit 25
```
"Goth Rock" (no "-ic") is the real term — 25 hits (the limit), all from 1979 (chronological
order means a low `--limit` only reaches the earliest year in a 750-hit range). Confirms
Bauhaus *Bela Lugosi's Dead* (1979, single, not used) and establishes the term is live.

```bash
$ tidalist find-by-attributes --style "Goth Rock" --year-from 1979 --year-to 1986 --limit 5000
```
750 total hits 1979-1986. Corroborates (with real `discogs_master_id`s, recorded in each
item's `note:` below): Bauhaus *In The Flat Field* (1980, master 2439), The Cure *Seventeen
Seconds* (1980, master 20278).

```bash
$ tidalist find-by-attributes --style "Post-Punk" --year-from 1979 --year-to 1986 --limit 25
```
25 hits (limit reached, all 1979). Corroborates Joy Division *Unknown Pleasures* (1979,
master 4805).

```bash
$ tidalist find-by-attributes --style "Post-Punk" --year-from 1980 --year-to 1980 --limit 500
```
169 hits (well under the limit). Corroborates Killing Joke *Killing Joke* (1980, master
15387) and Joy Division *Closer* (1980, master 4734).

```bash
$ tidalist find-by-attributes --style "Goth Rock" --year-from 1981 --year-to 1981 --limit 100
```
44 hits. Corroborates Siouxsie & The Banshees *Juju* (1981, master 42327).

```bash
$ tidalist find-by-attributes --style "Goth Rock" --year-from 1982 --year-to 1982 --limit 200
```
76 hits — of which only two are The Cure: *Pornography* (1982, master 20238, the album) and
*A Single* (1982, master 21145, a compilation of non-album singles — excluded). Corroborates
The Cure *Pornography*.

```bash
$ tidalist find-by-attributes --style "Goth Rock" --year-from 1985 --year-to 1985 --limit 200
```
189 hits — of which 8 are The Sisters of Mercy, one being the studio album *First And Last
And Always* (1985, master 2795); the rest are singles/live/compilations of the same era,
correctly excluded by the "album-oriented" + `no-compilation` criteria. Corroborates The
Sisters of Mercy *First and Last and Always*.

**Merge.** Every 4A pick was independently corroborated in the mirror by one of the sweeps
above (all eight items below carry a real `discogs_master_id`) — comfortably past the "at
least two" bar. The sweep's actual value shows up in what it *also* returned and 4A
correctly excluded: dozens of minor and adjacent acts (Cherry Vanilla, Gloria Mundi, UK
Decay, Play Dead, Virgin Prunes, The Damned's non-goth output, …) that genuinely carry the
"Goth Rock" or "Post-Punk" style tag but aren't part of the canon this brief asked for —
exactly the false-positive filtering 4A's judgment is for.
