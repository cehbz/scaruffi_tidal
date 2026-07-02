"""In-memory implementations of the Platform and MetadataProvider ports for tests.

These are real (deterministic) implementations, not mocks — tests specify behavior
against them, not against call expectations.
"""

from tidalist.core.album import Album
from tidalist.core.identifiers import PlaylistId, TrackId
from tidalist.core.catalog import Track, PlatformAlbum
from tidalist.core.recording import Candidate, Recording


class FakePlatform:
    def __init__(self, tracks, albums=(), album_track_map=None, album_editions_map=None):
        self._tracks = list(tracks)
        self._albums = list(albums)
        self._album_track_map: dict[str, list[Track]] = dict(album_track_map or {})
        self._album_editions_map: dict[str, list[PlatformAlbum]] = dict(album_editions_map or {})
        self.playlists: dict[str, list] = {}
        self._n = 0

    @staticmethod
    def _haystack(t: Track) -> str:
        return f"{t.title} {' '.join(t.artists)} {t.album or ''}".casefold()

    def search_tracks(self, query: str, limit: int = 25) -> list[Track]:
        words = query.casefold().split()
        return [t for t in self._tracks
                if all(w in self._haystack(t) for w in words)][:limit]

    def track_by_isrc(self, isrc):
        return next((t for t in self._tracks if t.isrc == isrc), None)

    def create_playlist(self, name: str, description: str = "") -> PlaylistId:
        self._n += 1
        pid = PlaylistId(f"pl-{self._n}")
        self.playlists[pid] = []
        return pid

    def add_tracks(self, playlist, tracks) -> None:
        self.playlists[playlist].extend(tracks)

    @staticmethod
    def _album_haystack(a: PlatformAlbum) -> str:
        return f"{a.title} {' '.join(a.artists)}".casefold()

    def search_albums(self, query: str, limit: int = 25) -> list[PlatformAlbum]:
        words = query.casefold().split()
        return [a for a in self._albums
                if all(w in self._album_haystack(a) for w in words)][:limit]

    def album_tracks(self, album_id: TrackId) -> list[Track]:
        return list(self._album_track_map.get(str(album_id), []))

    def album_editions(self, album_id: TrackId) -> list[PlatformAlbum]:
        return list(self._album_editions_map.get(str(album_id), []))


class FakeMetadataProvider:
    def __init__(self, recordings: dict[str, Recording | list[Recording]] | None = None,
                 albums: dict[str, Album | list[Album]] | None = None):
        # keyed by candidate title (case-insensitive); value is a Recording/Album or a list.
        recordings = recordings or {}
        albums = albums or {}
        self._recordings_by_title = {k.casefold(): (list(v) if isinstance(v, list) else [v])
                                     for k, v in recordings.items()}
        self._albums_by_title = {k.casefold(): (list(v) if isinstance(v, list) else [v])
                                 for k, v in albums.items()}

    def recordings_for(self, candidate: Candidate) -> list[Recording]:
        return list(self._recordings_by_title.get(candidate.title.casefold(), []))

    def albums_for(self, candidate: Candidate) -> list[Album]:
        return list(self._albums_by_title.get(candidate.title.casefold(), []))


class FakeRealizer:
    """Realizer port fake: resolves by recording title (missing => gap); records emits.

    `albums` maps album title → ([PlatformItem, ...], tuple[Compromise, ...]);
    resolve_album returns the AlbumResolution 3-tuple (items, compromises, gap_reason=None).
    """

    def __init__(self, items: dict, albums: dict | None = None):
        self._by_title = {k.casefold(): v for k, v in items.items()}
        self._albums = {k.casefold(): v for k, v in (albums or {}).items()}
        self.emitted: list = []

    def resolve(self, recording):
        return self._by_title.get(recording.title.casefold()), ()

    def resolve_album(self, album, preference):
        key = album.title.casefold()
        if key in self._albums:
            items, comps = self._albums[key]
            return items, comps, None
        return [], (), None

    def emit(self, name: str, items: list) -> str:
        ref = f"playlist-{len(self.emitted) + 1}"
        self.emitted.append((name, [i.ref for i in items], ref))
        return ref
