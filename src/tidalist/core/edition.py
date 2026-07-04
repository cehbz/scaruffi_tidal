"""Edition selection policy — a backend-agnostic preference over album editions.

`markers` is an ordered provenance preference (e.g. Steven Wilson, then MoFi); a renderer
matches them, best-effort, against whatever an edition's representation exposes on its
platform. `prefer_original` favors the canonical/earliest edition when no marker matches.
A particular backend's blindness never changes this preference — it surfaces as a reported
rendering compromise, not a downgrade of the golden.
"""

from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class EditionPreference:
    markers: tuple[str, ...] = ()
    prefer_original: bool = True

    def __post_init__(self):
        object.__setattr__(self, "markers", tuple(m.lower() for m in self.markers))


class EditionPolicy:
    """The standing default edition preference; a per-entry preference overrides it."""

    @staticmethod
    def default() -> EditionPreference:
        return EditionPreference(markers=("steven wilson", "mobile fidelity"), prefer_original=True)
