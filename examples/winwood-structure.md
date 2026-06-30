# Structure reading — Winwood career brief

Source: a natural-language prose brief (`examples/winwood-brief.md`), taken directly from
the user. No `defuddle` step — prose is a first-class input.

Structurally the *opposite* of Scaruffi: there is no per-item performer compound to
decompose. The performer thread ("Steve Winwood") is a single playlist-wide constraint, and
each named song or record is one item attributed to the band that released it.

- **unit:** one track or album named in the prose.
- **delimiter:** prose mentions — each quoted title (and each named album) is one item.
- **fields:**
  - `title:`   the quoted song/album name.
  - `kind:`    `recording` for a single track; `album` for a named LP
               (*John Barleycorn Must Die*).
  - `artist:`  the releasing act named in context — Spencer Davis Group, Traffic,
               Blind Faith, or Steve Winwood (solo). One `artist` credit per item; no
               role decomposition (pop/rock, not classical).
- **alternates:** none.
- **brief:** the playlist-wide constraints collapse to one `Brief:` line —
  `studio` (studio takes only), `no-compilation` (original studio releases, no best-ofs),
  and `performed-by: Steve Winwood` (the career thread that ties the bands together).

The "no live" wish in the prose maps to the `studio` token (studio is the positive form of
no-live); it is not emitted as a separate `no-live` token to avoid redundancy.
