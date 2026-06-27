"""Map MusicBrainz recordings to core Recordings, and serve them through MetadataProvider.

MusicBrainz is recording-centric: it yields the recording MBID, title, performer
relationships, and the earliest release year. `recordings_for` discovers candidates
from a search; it does not select among them (the Curator does) and does not pay for
the per-recording ISRC fetch — ISRC is enriched lazily once a recording is chosen.
Callers must musicbrainzngs.set_useragent(...) before live use.
"""

import musicbrainzngs

from ..core.album import Album, TrackRef, ReleaseTrait
from ..core.identifiers import ISRC, MBID, ExternalIds, Source
from ..core.recording import Candidate, Credit, Recording, Performance


def _traits_from_mb(secondary_list) -> frozenset[ReleaseTrait]:
    """Map MusicBrainz secondary-type-list to ReleaseTrait values, dropping unmapped entries."""
    _map = {"compilation": ReleaseTrait.COMPILATION, "live": ReleaseTrait.LIVE}
    result = set()
    for s in (secondary_list or []):
        trait = _map.get(s.lower())
        if trait is not None:
            result.add(trait)
    return frozenset(result)


def album_from_release_group(rg: dict, tracklist: tuple[TrackRef, ...] = ()) -> Album:
    """Map a MusicBrainz release-group dict (search hit) to an Album."""
    frd = rg.get("first-release-date") or ""
    first_released = int(frd[:4]) if len(frd) >= 4 and frd[:4].isdigit() else None
    artist = rg.get("artist-credit-phrase") or _first_artist(rg)
    return Album(
        artist=artist,
        title=rg["title"],
        ids=ExternalIds(mbid=MBID(rg["id"]) if rg.get("id") else None,
                        sources=frozenset({Source.MUSICBRAINZ})),
        first_released=first_released,
        traits=_traits_from_mb(rg.get("secondary-type-list")),
        tracklist=tracklist,
    )


def recording_from_musicbrainz(rec: dict) -> Recording:
    """Map a MusicBrainz recording dict (search hit or full fetch) to a Recording.

    Tolerant of both shapes: a search hit omits isrc-list/artist-relation-list, so
    those map to None/empty until a full fetch enriches them.
    """
    return Recording(
        artist=rec.get("artist-credit-phrase") or _first_artist(rec),
        title=rec.get("title") or "",
        mbid=MBID(rec["id"]) if rec.get("id") else None,
        isrc=_first_isrc(rec),
        album=_first_album(rec),
        first_released=_first_year(rec),
        duration_s=_duration_s(rec),
        performance=_performance(rec),
        credits=_credits(rec),
    )


def _first_artist(rec: dict) -> str:
    for e in rec.get("artist-credit") or []:
        if isinstance(e, dict) and "artist" in e:
            return e["artist"].get("name", "")
    return ""


def _first_album(rec: dict) -> str | None:
    for r in rec.get("release-list") or []:
        if r.get("title"):
            return r["title"]
    return None


def _duration_s(rec: dict) -> int | None:
    length = rec.get("length")
    return int(length) // 1000 if length is not None and str(length).isdigit() else None


def _first_isrc(rec: dict) -> ISRC | None:
    isrcs = rec.get("isrc-list") or []
    return ISRC(isrcs[0]) if isrcs else None


def _performance(rec: dict) -> Performance:
    return (Performance.LIVE if "live" in (rec.get("disambiguation") or "").lower()
            else Performance.UNKNOWN)


def _credited_to(rec: dict, artist_mbid: str) -> bool:
    return any(isinstance(e, dict) and (e.get("artist") or {}).get("id") == artist_mbid
               for e in rec.get("artist-credit") or [])


def _credits(rec: dict) -> tuple[Credit, ...]:
    credits = [Credit(e["artist"]["name"], "performer")
               for e in rec.get("artist-credit") or []
               if isinstance(e, dict) and "artist" in e]
    for rel in rec.get("artist-relation-list") or []:
        name = (rel.get("artist") or {}).get("name")
        if name:
            credits.append(Credit(name, rel.get("type", "performer")))
    return tuple(credits)


def _first_year(rec: dict) -> int | None:
    years = [int(d[:4]) for r in rec.get("release-list") or []
             if (d := r.get("date")) and d[:4].isdigit()]
    frd = rec.get("first-release-date")
    if frd and frd[:4].isdigit():
        years.append(int(frd[:4]))
    return min(years) if years else None


class MusicBrainzMetadata:
    """MetadataProvider port backed by musicbrainzngs."""

    def __init__(self, mb=musicbrainzngs, *, limit: int = 25, artist_limit: int = 5):
        self._mb = mb
        self._limit = limit
        self._artist_limit = artist_limit

    def _artist_mbid(self, artist: str) -> str | None:
        hits = self._mb.search_artists(artist=artist, limit=self._artist_limit).get("artist-list") or []
        return hits[0]["id"] if hits else None

    def recordings_for(self, candidate: Candidate) -> list[Recording]:
        results = self._mb.search_recordings(
            artist=candidate.artist, recording=candidate.title, limit=self._limit)
        hits = results.get("recording-list") or []
        artist_mbid = (str(candidate.artist_mbid) if candidate.artist_mbid is not None
                       else self._artist_mbid(candidate.artist))
        if artist_mbid is not None:
            hits = [h for h in hits if _credited_to(h, artist_mbid)]
        return [recording_from_musicbrainz(h) for h in hits]

    def _canonical_tracklist(self, rg_id: str) -> tuple[TrackRef, ...]:
        """Fetch and return an ordered canonical tracklist for the given release-group id."""
        try:
            result = self._mb.browse_releases(
                release_group=rg_id, includes=["recordings", "isrcs", "media"], limit=100)
            releases = result.get("release-list") or []

            official = [r for r in releases if r.get("status") == "Official"]
            if not official:
                return ()

            def track_count(release: dict) -> int:
                return sum(int(m.get("track-count", 0)) for m in release.get("medium-list") or [])

            counts = [track_count(r) for r in official]
            modal = max(set(counts), key=counts.count)

            canonical = sorted(official, key=lambda r: (abs(track_count(r) - modal), r.get("date", "")))[0]

            trackrefs = []
            for medium in canonical.get("medium-list") or []:
                for track in medium.get("track-list") or []:
                    recording = track.get("recording") or {}
                    position = int(track["position"])
                    title = recording.get("title")
                    mbid = MBID(recording["id"]) if recording.get("id") else None
                    isrc_list = recording.get("isrc-list") or []
                    isrc = ISRC(isrc_list[0]) if isrc_list else None
                    length = recording.get("length")
                    duration_s = round(int(length) / 1000) if length else None
                    trackrefs.append(TrackRef(
                        position=position,
                        title=title,
                        mbid=mbid,
                        isrc=isrc,
                        duration_s=duration_s,
                    ))
            return tuple(trackrefs)
        except Exception:
            return ()

    def albums_for(self, candidate: Candidate) -> list[Album]:
        results = self._mb.search_release_groups(
            artist=candidate.artist, releasegroup=candidate.title, limit=self._limit)
        rgs = results.get("release-group-list") or []
        artist_mbid = (str(candidate.artist_mbid) if candidate.artist_mbid is not None
                       else self._artist_mbid(candidate.artist))
        if artist_mbid is not None:
            rgs = [rg for rg in rgs if _credited_to(rg, artist_mbid)]
        return [album_from_release_group(rg, tracklist=self._canonical_tracklist(rg["id"]))
                for rg in rgs]
