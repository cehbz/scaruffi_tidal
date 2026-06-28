# TODO

Active work only. Completed work is in git history; architecture/design in `docs/superpowers/`.

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
- [ ] Recording selection sometimes picks a later re-recording (e.g. "I'm a Man" -> 1988); prefer the original.
- [ ] Per-artist caching: memoize artist-MBID resolution and the discography per run.
- [ ] Local backends: a local MusicBrainz/Discogs mirror (removes the 1 req/s curate throttle) and a local-file realizer / persistent Tidal lookup cache.

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
- [ ] Integration tests for remaining live shapes: MB `search_recordings`/`recordings_for`, the recording `resolve`/`emit` path, Discogs result shapes.
