"""JSON-pure (de)serialization of the durable golden artifact and the intent hand-off.

Criteria are a closed discriminated union (a `type` tag), so a front-end emits only
known rule types, we validate by tag, and we never eval model output.
"""

from .identifiers import ISRC, MBID, ExternalIds, Source, DiscogsMasterId, DiscogsReleaseId
from .recording import Candidate, Credit, Recording, Performance, Kind
from .album import Album, TrackRef, ReleaseTrait
from .criteria import PerformedBy, Studio, NotCompilation, NotLive, Criterion, Verdict
from .brief import Brief
from .edition import EditionPreference
from .provenance import Provenance
from .golden import GoldenPlaylist, GoldenEntry


# --- criteria (discriminated union) ------------------------------------------

def _criterion_to_dict(c: Criterion) -> dict:
    if isinstance(c, PerformedBy):
        return {"type": "performed_by", "artist": c.artist}
    if isinstance(c, Studio):
        return {"type": "studio"}
    if isinstance(c, NotCompilation):
        return {"type": "not_compilation"}
    if isinstance(c, NotLive):
        return {"type": "not_live"}
    raise ValueError(f"unserializable criterion: {type(c).__name__}")


def _criterion_from_dict(d: dict) -> Criterion:
    kind = d["type"]
    if kind == "performed_by":
        return PerformedBy(d["artist"])
    if kind == "studio":
        return Studio()
    if kind == "not_compilation":
        return NotCompilation()
    if kind == "not_live":
        return NotLive()
    raise ValueError(f"unknown criterion type: {kind!r}")


# --- edition (de)serialization -----------------------------------------------

def _edition_to_dict(e: EditionPreference) -> dict:
    return {"markers": list(e.markers), "prefer_original": e.prefer_original}


def _edition_from_dict(d: dict) -> EditionPreference:
    # `prefer_original` defaults to True to mirror the EditionPreference VO default,
    # so an edition block omitting the key deserializes the same as EditionPreference().
    return EditionPreference(markers=tuple(d.get("markers", ())),
                             prefer_original=bool(d.get("prefer_original", True)))


# --- value objects -----------------------------------------------------------

def _candidate_to_dict(c: Candidate) -> dict:
    d: dict = {"artist": c.artist, "title": c.title, "album": c.album,
               "year": c.year, "isrc": c.isrc, "kind": c.kind.value}
    if c.criteria:
        d["criteria"] = [_criterion_to_dict(cr) for cr in c.criteria]
    if c.edition is not None:
        d["edition"] = _edition_to_dict(c.edition)
    if c.artist_mbid is not None:
        d["artist_mbid"] = str(c.artist_mbid)
    return d


def _candidate_from_dict(d: dict) -> Candidate:
    criteria = tuple(_criterion_from_dict(cr) for cr in d.get("criteria", []))
    edition_raw = d.get("edition")
    edition = _edition_from_dict(edition_raw) if edition_raw is not None else None
    artist_mbid_raw = d.get("artist_mbid")
    artist_mbid = MBID(artist_mbid_raw) if artist_mbid_raw is not None else None
    return Candidate(d["artist"], d["title"], d.get("album"), d.get("year"),
                     _isrc(d.get("isrc")), Kind(d.get("kind", "track")),
                     criteria=criteria, edition=edition, artist_mbid=artist_mbid)


def _brief_to_dict(b: Brief) -> dict:
    return {"criteria": [_criterion_to_dict(c) for c in b.criteria]}


def _brief_from_dict(name: str, d: dict) -> Brief:
    return Brief(name, tuple(_criterion_from_dict(c) for c in d.get("criteria", [])))


def _provenance_to_dict(p: Provenance) -> dict:
    return {"source": p.source, "note": p.note}


def _verdict_to_dict(v: Verdict) -> dict:
    return {"admitted": v.admitted, "violations": list(v.violations)}


def _isrc(value):
    return ISRC(value) if value is not None else None


def _mbid(value):
    return MBID(value) if value is not None else None


def _discogs_master_id(value):
    return DiscogsMasterId(value) if value is not None else None


def _discogs_release_id(value):
    return DiscogsReleaseId(value) if value is not None else None


def _trackref_to_dict(t: TrackRef) -> dict:
    return {"position": t.position, "title": t.title, "isrc": t.isrc,
            "mbid": t.mbid, "duration_s": t.duration_s}


def _credit_to_dict(c: Credit) -> dict:
    d = {"artist": c.artist, "role": c.role}
    if c.variants:
        d["variants"] = list(c.variants)
    return d


def _credit_from_dict(d: dict) -> Credit:
    return Credit(d["artist"], d["role"], tuple(d.get("variants", ())))


def _trackref_from_dict(d: dict) -> TrackRef:
    return TrackRef(
        position=d["position"],
        title=d["title"],
        isrc=ISRC(d["isrc"]) if d.get("isrc") else None,
        mbid=MBID(d["mbid"]) if d.get("mbid") else None,
        duration_s=d.get("duration_s"),
    )


# --- golden artifact (the durable, portable product) -------------------------

def _golden_entry_to_dict(e: GoldenEntry) -> dict:
    prov_verdict = {
        "provenance": _provenance_to_dict(e.provenance),
        "verdict": _verdict_to_dict(e.verdict),
    }
    if e.edition is not None:
        prov_verdict["edition"] = _edition_to_dict(e.edition)
    if isinstance(e.item, Album):
        a = e.item
        d = {"kind": "album", "mbid": a.ids.mbid, "artist": a.artist,
             "title": a.title, "year": a.first_released,
             "traits": sorted(t.value for t in a.traits),
             "tracklist": [_trackref_to_dict(t) for t in a.tracklist]}
        if a.ids.discogs_master_id is not None:
            d["discogs_master_id"] = a.ids.discogs_master_id
        if a.ids.discogs_release_id is not None:
            d["discogs_release_id"] = a.ids.discogs_release_id
        if a.ids.sources:
            d["sources"] = sorted(s.value for s in a.ids.sources)
        if a.styles:
            d["styles"] = sorted(a.styles)
        if a.credits:
            d["credits"] = [_credit_to_dict(c) for c in a.credits]
        return {**d, **prov_verdict}
    r = e.item
    return {
        "kind": "track",
        "mbid": r.mbid, "isrc": r.isrc, "artist": r.artist, "title": r.title,
        "album": r.album, "year": r.first_released, "duration_s": r.duration_s,
        "performance": r.performance.value,
        "credits": [_credit_to_dict(c) for c in r.credits],
        **prov_verdict,
    }


def _golden_entry_from_dict(d: dict) -> GoldenEntry:
    prov, v = d["provenance"], d["verdict"]
    provenance = Provenance(prov["source"], prov.get("note", ""))
    verdict = Verdict(v["admitted"], tuple(v.get("violations", [])))
    edition_raw = d.get("edition")
    edition = _edition_from_dict(edition_raw) if edition_raw is not None else None
    if d.get("kind", "track") == "album":
        ids = ExternalIds(
            mbid=_mbid(d.get("mbid")),
            discogs_master_id=_discogs_master_id(d.get("discogs_master_id")),
            discogs_release_id=_discogs_release_id(d.get("discogs_release_id")),
            sources=frozenset(Source(s) for s in d.get("sources", [])),
        )
        item = Album(artist=d["artist"], title=d["title"], ids=ids,
                     first_released=d.get("year"),
                     traits=frozenset(ReleaseTrait(t) for t in d.get("traits", [])),
                     styles=frozenset(d.get("styles", [])),
                     tracklist=tuple(_trackref_from_dict(t) for t in d.get("tracklist", [])),
                     credits=tuple(_credit_from_dict(c) for c in d.get("credits", [])))
    else:
        item = Recording(
            artist=d["artist"], title=d["title"], mbid=_mbid(d.get("mbid")),
            isrc=_isrc(d.get("isrc")), album=d.get("album"),
            first_released=d.get("year"), duration_s=d.get("duration_s"),
            performance=Performance(d["performance"]),
            credits=tuple(_credit_from_dict(c) for c in d.get("credits", [])))
    return GoldenEntry(item, provenance, verdict, edition=edition)


def to_golden(golden: GoldenPlaylist) -> dict:
    return {
        "name": golden.name,
        "brief": _brief_to_dict(golden.brief),
        "entries": [_golden_entry_to_dict(e) for e in golden.entries],
    }


def from_golden(data: dict) -> GoldenPlaylist:
    brief = _brief_from_dict(data["name"], data.get("brief", {}))
    entries = tuple(_golden_entry_from_dict(e) for e in data.get("entries", []))
    return GoldenPlaylist(data["name"], brief, entries)


# --- intent artifact (front-end hand-off: candidates + per-line notes + brief) -

def to_intent(brief: Brief, candidates: list[Candidate],
              provenances: list[Provenance]) -> dict:
    return {
        "name": brief.name,
        "brief": _brief_to_dict(brief),
        "candidates": [{**_candidate_to_dict(c), "note": p.note}
                       for c, p in zip(candidates, provenances)],
    }


def from_intent(data: dict, source: str = "nl") -> tuple[list[Candidate], list[Provenance], Brief]:
    brief = _brief_from_dict(data["name"], data.get("brief", {}))
    candidates, provenances = [], []
    for cd in data.get("candidates", []):
        candidates.append(_candidate_from_dict(cd))
        provenances.append(Provenance(source, cd.get("note", "")))
    return candidates, provenances, brief
