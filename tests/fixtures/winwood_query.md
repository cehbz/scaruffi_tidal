# Steve Winwood — natural-language test query

The canonical end-to-end NL brief. An `nl` front-end agent turns this into an
intent JSON (`winwood_intent.json` was hand-built from it); `curate` then produces
the golden, and `render` maps it onto Tidal. Deliberately packs many requirement
types into one spec (whole albums, group membership, individual tracks, performer
vs cover, edition preference).

> I want a playlist of Steve Winwood's greatest music. His best albums, including
> groups where he was a significant member not just his solo work, as well as
> individual tracks where he either made a significant contribution, is known for
> that track, or it's a fan or artist favorite. Include only performances where
> he's performing (except when including entire albums), not covers. Prefer
> original recordings to live, compilation, or re-releases.
