"""Reconcile a rendering against an existing platform playlist: additive by default.

Pure planning (`plan_sync`, `format_plan`): no I/O, no platform SDK imports. `apply_sync`
is the one function in this module that touches the platform, and it only executes what
the plan enumerates — a `SyncPlan` built without `prune=True` carries no removals, so
`apply_sync` never calls `remove_tracks` for it. This mirrors the CLI's dry-run-by-default
posture: build the plan, print it, and only call `apply_sync` when the caller explicitly
opts in (`--apply`), with removals gated a second time behind `--prune`.
"""

from dataclasses import dataclass

from .diff import PlaylistDelta
from .ports import Platform

_SAMPLE = 60


@dataclass(frozen=True, slots=True)
class SyncPlan:
    """A concrete plan to reconcile a rendered playlist against an existing platform playlist."""
    playlist: str
    additions: tuple[str, ...]   # track refs to add (from PlaylistDelta.additions)
    removals: tuple[str, ...]    # only populated when prune=True: unclaimed existing track ids


def plan_sync(delta: PlaylistDelta, playlist: str, prune: bool = False) -> SyncPlan:
    """Turn a PlaylistDelta into a SyncPlan.

    Additive by default: `removals` stays empty unless `prune` is explicitly True, in
    which case the delta's unclaimed existing tracks become removal candidates.
    """
    removals = tuple(str(t.id) for t in delta.unclaimed) if prune else ()
    return SyncPlan(playlist=playlist, additions=delta.additions, removals=removals)


def apply_sync(plan: SyncPlan, platform: Platform) -> None:
    """Execute a SyncPlan: add the additions, then remove the removals — if any.

    A plan built without `prune` carries no removals, so `remove_tracks` is never called
    for it; this function does not itself re-check any "prune" flag, it only acts on what
    the plan already enumerates.
    """
    if plan.additions:
        platform.add_tracks(plan.playlist, list(plan.additions))
    if plan.removals:
        platform.remove_tracks(plan.playlist, list(plan.removals))


def format_plan(plan: SyncPlan, applied: bool = False) -> str:
    """Render a SyncPlan as a stable-heading markdown summary.

    The plan body must be unambiguous about execution state when read in isolation:
    unless `applied` is True, it opens with a DRY RUN banner stating that no platform
    changes were made.
    """
    lines = []
    if not applied:
        lines += ["DRY RUN — no platform changes; re-run with --apply to execute", ""]
    lines += [
        f"# Sync plan: {plan.playlist}",
        "",
        f"- additions: {len(plan.additions)}",
        f"- removals: {len(plan.removals)}",
    ]
    if plan.additions:
        lines += ["", "## Additions (track refs to add)", ""]
        for ref in plan.additions[:_SAMPLE]:
            lines.append(f"- {ref}")
        if len(plan.additions) > _SAMPLE:
            lines.append(f"… and {len(plan.additions) - _SAMPLE} more")
    if plan.removals:
        lines += ["", "## Removals (existing track ids — applied only with --prune --apply)", ""]
        for ref in plan.removals[:_SAMPLE]:
            lines.append(f"- {ref}")
        if len(plan.removals) > _SAMPLE:
            lines.append(f"… and {len(plan.removals) - _SAMPLE} more")
    return "\n".join(lines)
