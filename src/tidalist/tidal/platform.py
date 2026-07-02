"""Map tidalapi objects to core value objects, and serve them through the Platform port."""

from datetime import datetime

import tidalapi
from tidalapi.exceptions import ObjectNotFound

from ..core.identifiers import ISRC, TrackId, PlaylistId
from ..core.catalog import Track, PlatformAlbum


# Tidal /v1/search returns 500 for queries over 256 characters (measured 2026-07-02:
# 256 OK, 257 -> 500; the bound is characters, not UTF-8 bytes).
_MAX_QUERY_LEN = 256


def _clamp_query(query: str) -> str:
    if len(query) <= _MAX_QUERY_LEN:
        return query
    cut = query[:_MAX_QUERY_LEN]
    head, sep, _ = cut.rpartition(" ")
    return head if sep else cut


class TidalPlatform:
    """Platform port backed by an authenticated tidalapi Session."""

    def __init__(self, session: tidalapi.Session):
        self._session = session

    def search_tracks(self, query: str, limit: int = 25) -> list[Track]:
        results = self._session.search(_clamp_query(query), models=[tidalapi.media.Track], limit=limit)
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
        results = self._session.search(_clamp_query(query), models=[tidalapi.album.Album], limit=limit)
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
