from tidalist.core.catalog import Track
from tidalist.core.diff import PlaylistDelta, EntryStatus
from tidalist.core.sync import SyncPlan, plan_sync, apply_sync, format_plan
from tests.fakes import FakePlatform


# --- fixtures ----------------------------------------------------------------

def _unclaimed():
    return (
        Track(id="50", title="Dear Mr Fantasy", artists=("Traffic",), isrc="GB2", album="Mr Fantasy"),
        Track(id="51", title="Paper Sun", artists=("Traffic",), isrc="GB3", album="Traffic"),
    )


def _delta(additions=("3",), unclaimed=None):
    return PlaylistDelta(
        counts={"fully_matched": 1, "partially_matched": 0, "new_vs_existing": 0,
                "gap": 0, "not_admitted": 0, "existing_total": 3, "existing_matched": 1},
        entries=(EntryStatus(0, "matched", "Glad", "Traffic", 1, 0),),
        additions=additions,
        unclaimed=_unclaimed() if unclaimed is None else unclaimed,
    )


# --- plan_sync -----------------------------------------------------------------

def test_plan_sync_carries_additions_from_delta():
    plan = plan_sync(_delta(), "PL1")
    assert plan.playlist == "PL1"
    assert plan.additions == ("3",)


def test_plan_sync_removals_empty_without_prune():
    plan = plan_sync(_delta(), "PL1", prune=False)
    assert plan.removals == ()


def test_plan_sync_removals_default_is_no_prune():
    plan = plan_sync(_delta(), "PL1")
    assert plan.removals == ()


def test_plan_sync_removals_are_unclaimed_ids_when_pruning():
    plan = plan_sync(_delta(), "PL1", prune=True)
    assert plan.removals == ("50", "51")


def test_plan_sync_removals_empty_when_pruning_but_nothing_unclaimed():
    plan = plan_sync(_delta(unclaimed=()), "PL1", prune=True)
    assert plan.removals == ()


# --- apply_sync ------------------------------------------------------------------

def test_apply_sync_calls_add_tracks_with_plan_additions():
    platform = FakePlatform([])
    pid = platform.create_playlist("Scaruffi")
    plan = SyncPlan(playlist=str(pid), additions=("3", "4"), removals=())
    apply_sync(plan, platform)
    assert platform.playlists[pid] == ["3", "4"]


def test_apply_sync_never_calls_remove_tracks_without_removals():
    platform = FakePlatform([])
    pid = platform.create_playlist("Scaruffi")
    plan = SyncPlan(playlist=str(pid), additions=("3",), removals=())
    apply_sync(plan, platform)
    assert not any(name == "remove_tracks" for name, _ in platform.calls)


def test_apply_sync_calls_remove_tracks_when_plan_has_removals():
    platform = FakePlatform([])
    pid = platform.create_playlist("Scaruffi")
    plan = SyncPlan(playlist=str(pid), additions=(), removals=("50", "51"))
    apply_sync(plan, platform)
    assert ("remove_tracks", (str(pid), ["50", "51"])) in platform.calls


def test_apply_sync_skips_add_tracks_call_when_no_additions():
    platform = FakePlatform([])
    pid = platform.create_playlist("Scaruffi")
    plan = SyncPlan(playlist=str(pid), additions=(), removals=("50",))
    apply_sync(plan, platform)
    assert not any(name == "add_tracks" for name, _ in platform.calls)


# --- format_plan -----------------------------------------------------------------

def test_format_plan_has_stable_headings_and_counts():
    plan = plan_sync(_delta(), "PL1", prune=True)
    text = format_plan(plan)
    assert "# Sync plan: PL1" in text
    assert "- additions: 1" in text
    assert "- removals: 2" in text
    assert "## Additions" in text
    assert "## Removals" in text
    assert "3" in text
    assert "50" in text and "51" in text


def test_format_plan_omits_removals_section_without_prune():
    plan = plan_sync(_delta(), "PL1")
    text = format_plan(plan)
    assert "## Removals" not in text
    assert "- removals: 0" in text


def test_format_plan_carries_dry_run_banner_by_default():
    """The plan body read in isolation must be unambiguous about execution state."""
    text = format_plan(plan_sync(_delta(), "PL1"))
    assert text.splitlines()[0] == "DRY RUN — no platform changes; re-run with --apply to execute"


def test_format_plan_omits_dry_run_banner_when_applied():
    text = format_plan(plan_sync(_delta(), "PL1"), applied=True)
    assert "DRY RUN" not in text
    assert text.splitlines()[0] == "# Sync plan: PL1"
