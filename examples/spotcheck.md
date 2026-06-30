# Resolution spot-check — interpret demonstration

**The golden lint test (`intent/examples_test.go`,
`TestExampleIntentsAreCleanAndCanonical`) is the automated gate.** This spot-check is the
INTERPRET Pass-4 "resolution spot-check": it samples interpreted items through the catalog
tools to look for a *systematic* cluster of misses that would imply mis-decomposition.

> **Environment note (correction to the task brief).** The brief assumed the MB+Discogs
> mirrors were *not* mounted. In this environment they **were**: `/Volumes/Crucial X10`
> was mounted with `musicbrainz.db` (59 GB) and `discogs.db` (49 GB), and the default
> paths in `cmd/tidalist/main.go` resolved. So the commands below were **actually run**
> and their observed output is recorded inline. A controller re-running them needs the
> same mirrors mounted (or `--musicbrainz-db` / `--discogs-db` / the `TIDALIST_*_DB`
> env vars pointing at them).

## Finding

The decomposition is sound. The misses below are **not** mis-decomposition — they trace to
the catalog's *work → recording* resolution, not to any role tag:

- **Winwood (rock/pop) — artist/album/title paths resolve correctly.** `find-album` and
  `find-recording --title … --credit artist:…` return exact, artist-confirmed matches.
- **Scaruffi (classical) — `find-recording --work` is limited by `resolveWorkID`.** That
  helper takes the single top FTS hit, and MusicBrainz splits a work into many sub-work
  entities (movements, "op. 45" variants, arrangements). So the top hit is often either a
  work row with *no* `l_recording_work` performance arcs (→ empty) or a sub-work
  (→ off-target recordings). The role decomposition itself is correct; the work-anchor is
  the limiting factor. This is a catalog characteristic to address separately — out of
  scope for the interpret demonstration.

---

## Scaruffi — the three compound items the old flat parser could not resolve

The point of these three is that the *compound* performer string was decomposed into
role-tagged credits (`conductor` / `soloist` / `orchestra` / `chorus`). The `--credit`
filter exercises that decomposition.

### Brahms — *Ein Deutsches Requiem* (Klemperer, 1961)

```bash
go run ./cmd/tidalist find-recording --work "Ein Deutsches Requiem" \
  --credit conductor:"Otto Klemperer" --credit orchestra:"Philharmonia Orchestra"
```

Observed: `{"candidates":[]}`. `resolve-work --name "Ein Deutsches Requiem"` resolves the
work fine, but the top-FTS work entity (`f91ae703…`) carries no performance arcs, so the
work-anchored recording lookup returns nothing even before the credit filter.

### Schumann — Concerto in A minor, Op. 54 (Kempff / Krips / LSO, 1953)

```bash
go run ./cmd/tidalist find-recording --work "Concerto in A minor" \
  --credit soloist:"Wilhelm Kempff" --credit conductor:"Josef Krips" \
  --credit orchestra:"London Symphony Orchestra"
```

Observed: `{"candidates":[]}` (same work-anchor limitation).

### Palestrina — *Missa Papae Marcelli* (Peter Phillips / Tallis Scholars, 2015)

```bash
go run ./cmd/tidalist find-recording --work "Missa Papae Marcelli" \
  --credit conductor:"Peter Phillips" --credit orchestra:"The Tallis Scholars"
```

Observed: `{"candidates":[]}` with the credit filter. *Without* `--credit`, the same work
query returns recordings anchored on a sub-work
(`"Missa Papae Marcelli: Communio: Optimam partem (Plainchant)"`, a Pro Cantione Antiqua
chant movement) — confirming the off-target work-anchor rather than a tagging fault.

---

## Winwood — artist/album/title paths (these resolve)

### Album — *John Barleycorn Must Die* (Traffic)

```bash
go run ./cmd/tidalist find-album --title "John Barleycorn Must Die" --credit artist:"Traffic"
```

Observed: artist-confirmed exact-title match —
`{"mbid":"3770d5ce-e0e1-3389-9acf-cd38f0722baf","discogs_master_id":69017,`
`"title":"John Barleycorn Must Die",…,"year":1970,`
`"match":{"artist_confirmed":true,"title_distance":0}}` (plus Discogs peers).

### Recording — "Gimme Some Lovin'" (The Spencer Davis Group)

```bash
go run ./cmd/tidalist find-recording --title "Gimme Some Lovin'" \
  --credit artist:"The Spencer Davis Group"
```

Observed: artist-confirmed exact-title match —
`{"mbid":"de045981-d7db-4d1e-bf45-eeadc536623e","title":"Gimme Some Lovin'",`
`"duration_s":173,"match":{"artist_confirmed":true,"title_distance":0},`
`"credits":[{"role":"artist","name":"The Spencer Davis Group"}]}` (plus alternates).
