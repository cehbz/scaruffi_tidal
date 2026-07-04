"""The golden stage: candidates + metadata + a brief -> an ordered, platform-agnostic
playlist of recordings.

Providers discover recordings; the Curator discriminates. For each candidate it judges
the discovered recordings against the brief's criteria and chooses one by the
recording-ranking, preferring admissible takes. A candidate that finds nothing, or whose
takes all violate the brief, still yields an entry — with a rejected verdict — so the
result is reviewable; rendering later acts only on admitted entries.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import TYPE_CHECKING

from .ports import MetadataProvider
from .album import Album
from .recording import Candidate, Recording, Kind
from .brief import Brief
from .criteria import Verdict
from .ranking import RecordingRanking, PreferStudioEarliest
from .provenance import Provenance

if TYPE_CHECKING:
    from .edition import EditionPreference


@dataclass(frozen=True, slots=True)
class GoldenEntry:
    item: Album | Recording
    provenance: Provenance
    verdict: Verdict
    edition: EditionPreference | None = None


@dataclass(frozen=True, slots=True)
class GoldenPlaylist:
    name: str
    brief: Brief
    entries: tuple[GoldenEntry, ...]


class Curator:
    def __init__(self, metadata: MetadataProvider,
                 ranking: RecordingRanking = PreferStudioEarliest()):
        self._metadata = metadata
        self._ranking = ranking

    def curate(self, brief: Brief, candidates: list[Candidate],
               provenances: list[Provenance] | None = None) -> GoldenPlaylist:
        if provenances is None:
            provenances = [Provenance("nl")] * len(candidates)
        elif len(provenances) != len(candidates):
            raise ValueError("provenances must be one per candidate")
        entries = tuple(self._entry(brief, c, p) for c, p in zip(candidates, provenances))
        return GoldenPlaylist(brief.name, brief, entries)

    def _entry(self, brief: Brief, candidate: Candidate, provenance: Provenance) -> GoldenEntry:
        combined_brief = self._combine(brief, candidate)
        if candidate.kind is Kind.ALBUM:
            return self._album_entry(candidate, provenance, combined_brief)
        recordings = self._metadata.recordings_for(candidate)
        if not recordings:
            miss = Recording(artist=candidate.artist, title=candidate.title)
            return GoldenEntry(miss, provenance, Verdict.rejected("no recording found"),
                               edition=candidate.edition)
        chosen = self._choose(combined_brief, recordings)
        return GoldenEntry(chosen, provenance, combined_brief.judge(chosen),
                           edition=candidate.edition)

    def _album_entry(self, candidate: Candidate, provenance: Provenance,
                    brief: Brief) -> GoldenEntry:
        albums = self._metadata.albums_for(candidate)
        if not albums:
            miss = Album(artist=candidate.artist, title=candidate.title)
            return GoldenEntry(miss, provenance, Verdict.rejected("no album found"),
                               edition=candidate.edition)
        return GoldenEntry(albums[0], provenance, brief.judge(albums[0]),
                           edition=candidate.edition)

    @staticmethod
    def _combine(brief: Brief, candidate: Candidate) -> Brief:
        """Return a brief whose criteria are the brief's + the candidate's combined."""
        if not candidate.criteria:
            return brief
        return Brief(brief.name, brief.criteria + candidate.criteria)

    def _choose(self, brief: Brief, recordings: list[Recording]) -> Recording:
        admissible = [r for r in recordings if brief.judge(r).admitted]
        return min(admissible or recordings, key=self._ranking.key)
