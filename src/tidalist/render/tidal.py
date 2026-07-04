"""TidalRenderer: the Renderer port for Tidal, built on the Platform port.

It composes a Platform (TidalPlatform in production), so all tidalapi specifics stay in
the Platform adapter. resolve() matches a recording to a track ISRC-first, then by
closeness; emit() creates a playlist and adds the resolved tracks.
"""

import re
import unicodedata

from ..core.ports import Platform
from ..core.identifiers import TrackId
from ..core.recording import Recording, Performance, Credit
from ..core.catalog import Track
from ..core.render import PlatformItem, MatchQuality, AlbumResolution
from ..core.fidelity import (
    PlatformCandidate, IdentityFacet, EditionFacet, PerformanceFacet, choose,
    recording_artist_match, Compromise, QualityPreference,
)
from ..core.edition import EditionPreference
from ..core.album import Album, TrackRef

_QUALITY_PREFERENCE = QualityPreference()


class TidalRenderer:
    def __init__(self, platform: Platform):
        self._platform = platform

    def resolve(self, recording: Recording) -> tuple[PlatformItem | None, tuple[Compromise, ...]]:
        if recording.isrc is not None:
            track = self._platform.track_by_isrc(recording.isrc)
            if track is not None:
                return _item(track, MatchQuality.ISRC), ()
        hits = self._platform.search_tracks(_query(recording))
        candidates = [_track_candidate(t) for t in hits]
        if not candidates:
            return None, ()
        chosen, comps = choose(recording, candidates, [IdentityFacet(), PerformanceFacet()],
                               tiebreak=lambda c: (_QUALITY_PREFERENCE.tiebreak(c), c.ref))
        if chosen is None:
            return None, ()
        return _item_from_candidate(chosen, _quality_for(recording, chosen)), comps

    def resolve_album(
        self,
        album: Album,
        preference: EditionPreference,
    ) -> AlbumResolution:
        survivors, tried = self._search_survivors(album)
        if not survivors:
            return self._assemble_from_tracks(album, tried)
        anchor = survivors[0]
        # The discography gives the full edition set; fall back to the search
        # survivors when it's empty (so `editions` is always non-empty here).
        editions = self._platform.album_editions(anchor.id) or survivors
        candidates = [self._candidate(e, with_tracks=bool(album.tracklist)) for e in editions]
        facets = [IdentityFacet(), EditionFacet(preference)]
        chosen, comps = choose(album, candidates, facets)
        if chosen is None:
            return AlbumResolution([], (), "edition-scoring: no candidate chosen")
        tracks = chosen.tracks or tuple(self._platform.album_tracks(TrackId(chosen.ref)))
        items = [_item(t, MatchQuality.STRONG) for t in tracks]
        return AlbumResolution(items, comps, None)

    def _candidate(self, edition, with_tracks: bool) -> PlatformCandidate:
        tracks = tuple(self._platform.album_tracks(edition.id)) if with_tracks else ()
        return PlatformCandidate(ref=str(edition.id), title=edition.title,
                                 artists=edition.artists, year=edition.year, tracks=tracks)

    def _assemble_from_tracks(self, album: Album, tried: int) -> AlbumResolution:
        """When no edition of the release-group is on the platform, assemble the
        canonical tracklist track-by-track from individual catalog tracks.

        `tried` is the number of anchor queries the search survivor pass attempted
        (used to build the no-edition-matched gap reason when there's no tracklist
        to fall back on)."""
        if not album.tracklist:
            return AlbumResolution(
                [], (), f"no-edition-matched: tried {tried} anchor queries, 0 survivors"
            )
        items: list[PlatformItem] = []
        missing: list[int] = []
        for tr in album.tracklist:
            item, _ = self.resolve(_recording_from_trackref(album, tr))
            if item is not None:
                items.append(item)
            else:
                missing.append(tr.position)
        if not items:
            return AlbumResolution(
                [], (), f"track-fallback: 0/{len(album.tracklist)} tracks found"
            )
        comp = _album_source_compromise(album, len(items), len(album.tracklist), missing)
        return AlbumResolution(items, (comp,), None)

    def _search_survivors(self, album: Album) -> tuple[list, int]:
        """Return (survivors, anchor queries tried) — the count is reported in the
        no-edition-matched gap reason when no survivors are ever found."""
        tried = 0
        for query in _anchor_queries(album):
            tried += 1
            hits = self._platform.search_albums(query)
            survivors = [
                c for c in hits
                if _artist_match_album(album, c.artists)
                and _title_match_album(album.title, c.title)
            ]
            if survivors:
                return survivors, tried
        return [], tried

    def emit(self, name: str, items: list[PlatformItem]) -> str:
        playlist = self._platform.create_playlist(name)
        self._platform.add_tracks(playlist, [TrackId(i.ref) for i in items])
        return str(playlist)


def _query(recording: Recording) -> str:
    return f"{recording.artist} {recording.title}".strip()


def _recording_from_trackref(album: Album, tr: TrackRef) -> Recording:
    return Recording(artist=album.artist, title=tr.title, isrc=tr.isrc,
                     mbid=tr.mbid, duration_s=tr.duration_s)


def _album_source_compromise(album: Album, found: int, total: int,
                              missing: list[int]) -> Compromise:
    note = (f"album '{album.title}' unavailable; assembled {found}/{total} tracks "
            f"from individual catalog tracks")
    if missing:
        note += f" (missing positions: {', '.join(str(p) for p in missing)})"
    return Compromise("album-source", "original album",
                      f"assembled {found}/{total} tracks", note)


# Curly quotes/apostrophes and the Unicode hyphen family defeat substring matching:
# the materialized GM carries MB-sourced names/titles containing U+2019, U+2010/U+2011,
# and curly double quotes. Mirrors core.NormalizeName's fold (Go side), extended for
# U+2010 HYPHEN and curly double-quotes the GM actually carries (Go's fold omits U+2010).
_PUNCT_FOLD = str.maketrans({
    "’": "'", "‘": "'", "ʼ": "'",       # U+2019, U+2018, U+02BC -> '
    "”": '"', "“": '"',                        # U+201D, U+201C -> "
    "‐": "-", "‑": "-", "‒": "-", "–": "-",  # U+2010–U+2013 -> -
})


def _fold(s: str | None) -> str:
    """Casefold + NFD-strip combining marks + fold curly punctuation and Unicode hyphens
    to ASCII, so diacritic/curly/hyphen variants match. The single normalizer every
    render string comparator routes through (title match, artist match, query norm)."""
    if not s:
        return ""
    decomposed = unicodedata.normalize("NFD", s)
    stripped = "".join(c for c in decomposed if not unicodedata.combining(c))
    return stripped.translate(_PUNCT_FOLD).casefold().strip()


_ARTICLES = ("the ", "a ", "an ")


def _strip_leading_article(s: str) -> str:
    f = s.casefold()
    for art in _ARTICLES:
        if f.startswith(art):
            return s[len(art):]
    return s


# Compound classical titles differ in their work separator: golden uses "/", Tidal's
# catalog uses ";" or "&". Split on any of them for segment-overlap matching.
_TITLE_SEP = re.compile(r"\s*[/;&]\s*")


def _title_segments(title: str) -> list[str]:
    return [seg for seg in _TITLE_SEP.split(title) if seg.strip()]


# A verbose enumerated title dilutes Tidal's top-N window; its leading segment (before
# the first ':' '/' ';' '(' or ',') is a shorter, higher-ranking additive query. Precision
# is still guarded by the (precise) survivor title match, so a broad short query is safe.
def _short_title(title: str) -> str:
    return re.split(r"[/:;(,]", title, maxsplit=1)[0].strip()


# Anchor-query priority for role-tagged credits: conductor is most selective,
# orchestra/chorus next, plain artist credits least (still ahead of the compound fallbacks).
_ANCHOR_ROLE_PRIORITY = {"conductor": 0, "orchestra": 1, "chorus": 1, "artist": 2}


def _credit_forms(credit: Credit) -> tuple[str, ...]:
    """A credit's own name plus its materialized Latin variants (GM `variants`),
    deduped case-insensitively, order-preserving."""
    seen: set[str] = set()
    out: list[str] = []
    for form in (credit.artist, *credit.variants):
        f = form.strip()
        if f and f.casefold() not in seen:
            seen.add(f.casefold())
            out.append(f)
    return tuple(out)


_COMPOSER_ROLE = "composer"
_MAX_CREDIT_QUERIES = 8


def _anchor_queries(album: Album):
    """Yield de-duplicated search queries for album, most to least specific.

    Per anchor credit (conductor, then orchestra/chorus, then artist) and per Latin
    variant of it: `"{form} {title}"`, a two-credit combo pairing the form with the
    composer (for generic titles that omit the composer), and a short-title variant.
    Credit-derived queries are capped at _MAX_CREDIT_QUERIES; the compound-artist and
    title/short-title fallbacks always follow so the title-only fallback is guaranteed."""
    seen: set[str] = set()
    ranked_credits = sorted(
        (c for c in album.credits if c.role in _ANCHOR_ROLE_PRIORITY),
        key=lambda c: _ANCHOR_ROLE_PRIORITY[c.role],
    )
    composer = next((c for c in album.credits if c.role == _COMPOSER_ROLE), None)
    title = album.title
    stitle = _short_title(title)
    credit_qs: list[str] = []
    for c in ranked_credits:
        for form in _credit_forms(c):
            credit_qs.append(f"{form} {title}")
            if composer is not None and composer.role != c.role:
                for cf in _credit_forms(composer):
                    credit_qs.append(f"{form} {cf} {title}")
            if stitle and stitle != title:
                credit_qs.append(f"{form} {stitle}")
    tail = [
        f"{album.artist} {title}",
        f"{_strip_leading_article(album.artist)} {title}",
        title,
        stitle if stitle and stitle != title else None,
    ]
    for q in credit_qs[:_MAX_CREDIT_QUERIES] + tail:
        if q is None:
            continue
        q = q.strip()
        if q and q not in seen:
            seen.add(q)
            yield q


def _item(track: Track, quality: MatchQuality) -> PlatformItem:
    return PlatformItem(ref=str(track.id), title=track.title, artists=track.artists,
                        isrc=track.isrc, quality=quality)


def _norm(s: str | None) -> str:
    return _fold(s)


def _artist_match_album(album: Album, catalog_artists: tuple[str, ...]) -> bool:
    """A candidate matches when the compound album.artist, any credit name, or any
    materialized credit variant substring-matches a catalog artist (folded, either
    direction) — a candidate credited to just the orchestra/conductor (in any of its
    known name forms) still counts as the same album."""
    names = [album.artist]
    for c in album.credits:
        names.extend(_credit_forms(c))
    return any(_name_in_catalog(name, catalog_artists) for name in names)


def _name_in_catalog(name: str, catalog_artists: tuple[str, ...]) -> bool:
    n = _fold(name)
    if not n:
        return False
    return any(n in _fold(ca) or _fold(ca) in n for ca in catalog_artists)


def _title_match_album(title: str, catalog_title: str) -> bool:
    t = _fold(title)
    ct = _fold(catalog_title)
    if t and (t in ct or ct in t):
        return True
    # Leading article: golden "A Polish Requiem" vs catalog "…: Polish Requiem".
    ta = _fold(_strip_leading_article(title))
    if ta and ta in ct:
        return True
    # Compound title: a shared work segment counts (golden "/" vs Tidal ";"/"&").
    gsegs = [_fold(s) for s in _title_segments(title)]
    csegs = [_fold(s) for s in _title_segments(catalog_title)]
    if len(gsegs) > 1 or len(csegs) > 1:
        for g in gsegs:
            if g and any(g in c or c in g for c in csegs):
                return True
    return False


_LIVE_MARKERS = ("(live", "[live", " live at ", " - live", "live in ", "live from", "unplugged")


def _observe_performance(title: str) -> Performance:
    t = title.casefold()
    return Performance.LIVE if any(m in t for m in _LIVE_MARKERS) else Performance.UNKNOWN


def _track_candidate(track: Track) -> PlatformCandidate:
    return PlatformCandidate(
        ref=str(track.id), title=track.title, artists=track.artists,
        isrc=track.isrc, duration_s=track.duration_s,
        performance=_observe_performance(track.title),
        audio_quality=track.audio_quality, popularity=track.popularity,
    )


def _item_from_candidate(cand: PlatformCandidate, quality: MatchQuality) -> PlatformItem:
    return PlatformItem(ref=cand.ref, title=cand.title, artists=cand.artists,
                        isrc=cand.isrc, quality=quality)


def _quality_for(recording: Recording, cand: PlatformCandidate) -> MatchQuality:
    title_ok = _norm(recording.title) == _norm(cand.title)
    artist_ok = recording_artist_match(recording, cand.artists)
    return MatchQuality.STRONG if title_ok and artist_ok else MatchQuality.WEAK
