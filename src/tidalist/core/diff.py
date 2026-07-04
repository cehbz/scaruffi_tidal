"""Diff a rendering (resolved platform items) against an existing platform playlist.

Track identity for matching: platform track id first, then ISRC, then a normalized
(title, first-artist) key — the last classifies "same music, different edition/take"
rather than missing. Ported from the `scripts/compare_playlists.py` prototype; the
normalizers and classification rules are unchanged.

Pure: no I/O, no platform SDK imports. The CLI layer reads the rendering JSONL and the
existing-playlist snapshot (or fetches it live via the Platform port) and feeds them here.
"""

import re
import unicodedata
from dataclasses import dataclass

from .catalog import Track

_BASE_COUNTS = ("fully_matched", "partially_matched", "new_vs_existing", "gap", "not_admitted")


def norm(s: str | None) -> str:
    """Normalize a string for fuzzy matching: fold accents, drop parenthetical/
    bracketed asides, lowercase, collapse to whitespace-joined alnum tokens."""
    if not s:
        return ""
    s = unicodedata.normalize("NFKD", s)
    s = "".join(c for c in s if not unicodedata.combining(c))
    s = re.sub(r"\(.*?\)|\[.*?\]", " ", s.lower())
    s = re.sub(r"[^a-z0-9]+", " ", s)
    return " ".join(s.split())


def tkey(title: str | None, artist: str | None) -> tuple[str, str]:
    """A normalized (title, first-artist) identity key for fuzzy matching."""
    return (norm(title), norm(artist))


@dataclass(frozen=True, slots=True)
class EntryStatus:
    """One rendering entry's match classification against the existing playlist."""
    index: int
    status: str  # matched | partial | all-new | gap | gap (error) | not-admitted
    title: str
    artist: str
    matched: int   # items matched by platform id or ISRC
    fuzzy: int     # items matched only by normalized (title, artist)
    note: str = ""


@dataclass(frozen=True, slots=True)
class PlaylistDelta:
    """The delta between a rendering and an existing platform playlist.

    `counts` always carries fully_matched/partially_matched/new_vs_existing/gap/
    not_admitted, plus existing_total/existing_matched, and (when a curate report was
    supplied) report_absent/report_marginal.
    """
    counts: dict[str, int]
    entries: tuple[EntryStatus, ...]
    additions: tuple[str, ...]        # platform track refs realized but absent from existing
    unclaimed: tuple[Track, ...]      # existing tracks no rendered entry claimed
    report_notes: tuple[str, ...] = ()  # curate-report "absent" notes, when a report was given


def diff_playlist(rendering: list[dict], existing: list[Track], report: dict | None = None) -> PlaylistDelta:
    """Classify each rendering entry against the existing playlist's current tracks.

    `rendering` is the parsed rendering.jsonl records (see scripts/render_dump.py's
    shape): {index, golden:{title,artist,note,...}, items:[{ref,title,artists,isrc,...}],
    gap, skipped?, error?}. `existing` is the existing playlist's current tracks.
    `report` is the curate-time report ({"items": [{"disposition","marginal","note"}, ...]})
    for additional context, or None to omit that section.
    """
    ex_by_id: dict[str, Track] = {str(t.id): t for t in existing}
    ex_by_isrc: dict[str, Track] = {}
    ex_by_key: dict[tuple[str, str], Track] = {}
    for t in existing:
        if t.isrc:
            ex_by_isrc.setdefault(t.isrc, t)
        ex_by_key.setdefault(tkey(t.title, t.primary_artist), t)

    matched_ids: set[str] = set()
    seen_additions: set[str] = set()
    additions: list[str] = []
    entry_rows: list[EntryStatus] = []
    counts = {k: 0 for k in _BASE_COUNTS}

    for e in rendering:
        g = e["golden"]
        idx = e["index"]
        title, artist, note = g["title"], g.get("artist", "?"), g.get("note") or ""

        if e.get("skipped"):
            counts["not_admitted"] += 1
            entry_rows.append(EntryStatus(idx, "not-admitted", title, artist, 0, 0, note))
            continue

        items = e.get("items") or []
        if not items:
            status = "gap" + (" (error)" if e.get("error") else "")
            counts["gap"] += 1
            entry_rows.append(EntryStatus(idx, status, title, artist, 0, 0, note))
            continue

        by_id = by_fuzzy = 0
        for it in items:
            t = ex_by_id.get(str(it["ref"]))
            if t is None and it.get("isrc"):
                t = ex_by_isrc.get(it["isrc"])
            if t is not None:
                by_id += 1
                matched_ids.add(str(t.id))
                continue
            t = ex_by_key.get(tkey(it["title"], (it.get("artists") or [""])[0]))
            if t is not None:
                by_fuzzy += 1
                matched_ids.add(str(t.id))
                continue
            ref = str(it["ref"])
            if ref not in seen_additions:
                seen_additions.add(ref)
                additions.append(ref)

        if by_id + by_fuzzy == 0:
            counts["new_vs_existing"] += 1
            status = "all-new"
        elif by_id + by_fuzzy == len(items):
            counts["fully_matched"] += 1
            status = "matched"
        else:
            counts["partially_matched"] += 1
            status = "partial"
        entry_rows.append(EntryStatus(idx, status, title, artist, by_id, by_fuzzy, note))

    unclaimed = tuple(t for t in existing if str(t.id) not in matched_ids)
    counts["existing_total"] = len(existing)
    counts["existing_matched"] = len(matched_ids)

    report_notes: tuple[str, ...] = ()
    if report is not None:
        absents = [i for i in report["items"] if i.get("disposition") == "absent"]
        marginals = [i for i in report["items"] if i.get("marginal")]
        counts["report_absent"] = len(absents)
        counts["report_marginal"] = len(marginals)
        report_notes = tuple(i.get("note", "") for i in absents)

    return PlaylistDelta(
        counts=counts,
        entries=tuple(entry_rows),
        additions=tuple(additions),
        unclaimed=unclaimed,
        report_notes=report_notes,
    )


def format_delta(delta: PlaylistDelta) -> str:
    """Render a PlaylistDelta as the markdown delta report (stable heading text)."""
    c = delta.counts
    lines = ["# Delta report", "", f"- GM entries rendered: {len(delta.entries)}"]
    for k in _BASE_COUNTS:
        lines.append(f"- {k}: {c.get(k, 0)}")
    lines.append(
        f"- existing playlist tracks: {c.get('existing_total', 0)}; "
        f"matched by some entry: {c.get('existing_matched', 0)}; "
        f"unmatched (extra in existing): {len(delta.unclaimed)}"
    )

    lines += ["", "## Per-entry status (non-matched only)", ""]
    for e in delta.entries:
        if e.status == "matched":
            continue
        lines.append(f"- [{e.status}] {e.artist} — {e.title}  "
                     f"(id-match {e.matched}, fuzzy {e.fuzzy})  · {e.note[:80]}")

    lines += ["", "## Existing-playlist tracks no rendered entry claimed (sample, first 60)", ""]
    for t in delta.unclaimed[:60]:
        lines.append(f"- {t.primary_artist} — {t.title}  (album: {t.album or '?'})")
    if len(delta.unclaimed) > 60:
        lines.append(f"… and {len(delta.unclaimed) - 60} more")

    if "report_absent" in c:
        lines += ["", f"## Curate-report context: {c['report_absent']} absent, "
                      f"{c.get('report_marginal', 0)} marginal at curate time", ""]
        for note in delta.report_notes:
            lines.append(f"- absent: {note[:100]}")

    return "\n".join(lines)


def delta_to_dict(delta: PlaylistDelta) -> dict:
    """Serialize a PlaylistDelta as the JSON artifact the sync stage consumes."""
    return {
        "counts": dict(delta.counts),
        "entries": [
            {"index": e.index, "status": e.status, "title": e.title, "artist": e.artist,
             "matched": e.matched, "fuzzy": e.fuzzy, "note": e.note}
            for e in delta.entries
        ],
        "additions": list(delta.additions),
        "unclaimed": [
            {"id": str(t.id), "title": t.title, "artist": t.primary_artist,
             "isrc": t.isrc, "album": t.album}
            for t in delta.unclaimed
        ],
    }
