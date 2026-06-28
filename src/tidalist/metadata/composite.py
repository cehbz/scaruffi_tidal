"""Metadata composite provider: reconciles MusicBrainz and Discogs album results."""

from __future__ import annotations

from ..core.album import Album
from ..core.ports import MetadataProvider
from ..core.recording import Candidate, Recording
from .reconcile import reconcile_albums


class Metadata:
    """Composite MetadataProvider over two peer providers; reconciles album results."""

    def __init__(self, mb: MetadataProvider, discogs: MetadataProvider) -> None:
        self._mb = mb
        self._discogs = discogs

    def albums_for(self, candidate: Candidate) -> list[Album]:
        return reconcile_albums(
            self._mb.albums_for(candidate),
            self._discogs.albums_for(candidate),
        )

    def recordings_for(self, candidate: Candidate) -> list[Recording]:
        # Discogs contributes no recordings yet
        return self._mb.recordings_for(candidate)
