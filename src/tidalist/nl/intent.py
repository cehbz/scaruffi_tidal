"""The intent contract: the JSON a curation agent emits, parsed into (candidates, brief).

Intent JSON:

    {
      "name": "<playlist name>",
      "brief": {
        "criteria": [{"type": "performed_by", "artist": "..."}, {"type": "studio"}]
      },
      "candidates": [
        {
          "artist": "...",
          "title": "...",
          "album": "...?",
          "year": 1970,
          "isrc": "...?",
          "kind": "track|album",
          "note": "<why it belongs>",
          "criteria": [{"type": "performed_by", "artist": "..."}],
          "edition": {"markers": ["steven wilson"], "prefer_original": true},
          "artist_mbid": "...?"
        }
      ]
    }

`kind` defaults to "track"; omit for single tracks. Per-candidate `criteria` are
combined with the brief's at judging time. `edition` overrides the global render-time
edition policy for album candidates. `artist_mbid` is an identity hint that bypasses
the artist-search call in the MusicBrainz provider. Criteria are a closed tag union —
validated by tag, never eval'd. `note` becomes the entry's provenance rationale.
"""

from ..core.recording import Candidate
from ..core.provenance import Provenance
from ..core.brief import Brief
from ..core.spec import from_intent


def parse_intent(data: dict, source: str = "nl") -> tuple[list[Candidate], list[Provenance], Brief]:
    """Validate the agent's intent JSON and parse it. Raises ValueError on a bad shape."""
    if not data.get("name"):
        raise ValueError("intent requires a non-empty 'name'")
    if not data.get("candidates"):
        raise ValueError("intent requires at least one candidate")
    return from_intent(data, source=source)
