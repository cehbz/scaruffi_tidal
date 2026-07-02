"""Map tidalapi objects to core value objects, and serve them through the Platform port."""

import time
from datetime import datetime

import tidalapi
from requests.exceptions import HTTPError
from tidalapi.exceptions import ObjectNotFound

from ..core.identifiers import ISRC, TrackId, PlaylistId
from ..core.catalog import Track, PlatformAlbum


# Tidal /v1/search returns 500 for queries over 256 characters (measured 2026-07-02:
# 256 OK, 257 -> 500; the bound is characters, not UTF-8 bytes).
_MAX_QUERY_LEN = 256

# Measured 2026-07-02: a 6-minute live realize run lost 25/64 entries to transient
# Tidal-side HTTP 500s on /v1/search (short, valid queries that succeed minutes later).
# Retry 5xx a bounded number of times so unattended multi-hour runs survive these windows.
_RETRY_SLEEPS_S = (1, 4)

# Patchable seam for tests.
_SLEEP = time.sleep


def _clamp_query(query: str) -> str:
    if len(query) <= _MAX_QUERY_LEN:
        return query
    cut = query[:_MAX_QUERY_LEN]
    head, _, _ = cut.rpartition(" ")
    return head if head else cut


def _retry_5xx(fn):
    """Call fn(), retrying with backoff on HTTPError with a 5xx status.

    Non-HTTP errors, 4xx responses, and a final exhausted 5xx propagate unchanged.
    """
    attempts = len(_RETRY_SLEEPS_S) + 1
    for attempt in range(attempts):
        try:
            return fn()
        except HTTPError as e:
            status = getattr(e.response, "status_code", None)
            is_5xx = status is not None and 500 <= status < 600
            if not is_5xx or attempt == attempts - 1:
                raise
            _SLEEP(_RETRY_SLEEPS_S[attempt])


class TidalPlatform:
    """Platform port backed by an authenticated tidalapi Session."""

    def __init__(self, session: tidalapi.Session):
        self._session = session

    def search_tracks(self, query: str, limit: int = 25) -> list[Track]:
        clamped = _clamp_query(query)
        results = _retry_5xx(lambda: self._session.search(clamped, models=[tidalapi.media.Track], limit=limit))
        return [track_from_tidal(t) for t in results["tracks"][:limit]]

    def track_by_isrc(self, isrc: ISRC) -> Track | None:
        try:
            hits = self._session.get_tracks_by_isrc(isrc)
        except ObjectNotFound:
            # tidalapi raises ObjectNotFound when the ISRC is not in the catalog.
            return None
        return track_from_tidal(hits[0]) if hits else None

    def create_playlist(self, name: str, description: str = "") -> PlaylistId:
        playlist = self._session.user.create_playlist(name, description)
        return PlaylistId(str(playlist.id))

    def add_tracks(self, playlist: PlaylistId, tracks: list[TrackId]) -> None:
        self._session.playlist(playlist).add([str(t) for t in tracks])

    def search_albums(self, query: str, limit: int = 25) -> list[PlatformAlbum]:
        clamped = _clamp_query(query)
        results = _retry_5xx(lambda: self._session.search(clamped, models=[tidalapi.album.Album], limit=limit))
        return [_album_from_tidal(a) for a in results["albums"][:limit]]

    def album_tracks(self, album_id: TrackId) -> list[Track]:
        return [track_from_tidal(t) for t in self._session.album(album_id).tracks()]

    def album_editions(self, album_id: TrackId) -> list[PlatformAlbum]:
        try:
            anchor = self._session.album(album_id)
            discography = anchor.artist.get_albums()
            return [_album_from_tidal(x) for x in discography if _same_album_title(anchor.name, x.name)]
        except Exception:
            return []


def track_from_tidal(t) -> Track:
    return Track(
        id=TrackId(str(t.id)),
        title=t.name,
        artists=tuple(_artist_names(t)),
        isrc=ISRC(t.isrc) if getattr(t, "isrc", None) else None,
        album=t.album.name if getattr(t, "album", None) else None,
        year=_year(t),
        duration_s=getattr(t, "duration", None),
        audio_quality=getattr(t, "audio_quality", None),
        popularity=getattr(t, "popularity", None),
    )


def _artist_names(t) -> list[str]:
    artists = getattr(t, "artists", None)
    if artists:
        return [a.name for a in artists]
    artist = getattr(t, "artist", None)
    return [artist.name] if artist else ["Unknown"]


def _album_from_tidal(a) -> PlatformAlbum:
    artists = tuple(ar.name for ar in getattr(a, "artists", []))
    return PlatformAlbum(
        id=TrackId(str(a.id)),
        title=a.name,
        artists=artists,
        year=getattr(a, "year", None),
        num_tracks=getattr(a, "num_tracks", None),
    )


def _same_album_title(a: str, b: str) -> bool:
    return a.casefold() in b.casefold() or b.casefold() in a.casefold()


def _year(t) -> int | None:
    # tidalapi exposes release dates as datetime; Album.year is already an int.
    # Normalize to int here so the core Track never receives a datetime.
    album = getattr(t, "album", None)
    if album is not None and getattr(album, "year", None):
        return album.year
    released = getattr(t, "tidal_release_date", None)
    if isinstance(released, datetime):
        return released.year
    return None
