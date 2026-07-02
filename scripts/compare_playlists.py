#!/usr/bin/env python3
"""Diff a realization dump against an existing Tidal playlist snapshot.

Inputs: realization.jsonl (from realize_dump.py), existing-playlist.json (a
track list snapshot), and the curate report (for absent/marginal context).
Output: a markdown delta report on stdout.

Track identity for matching: Tidal track id first, then ISRC, then a normalized
(title, first-artist) key — the last classifies "same music, different
edition/take" rather than missing.
"""

import json
import re
import sys
import unicodedata
from collections import Counter


def norm(s):
    if not s:
        return ""
    s = unicodedata.normalize("NFKD", s)
    s = "".join(c for c in s if not unicodedata.combining(c))
    s = re.sub(r"\(.*?\)|\[.*?\]", " ", s.lower())
    s = re.sub(r"[^a-z0-9]+", " ", s)
    return " ".join(s.split())


def tkey(title, artist):
    return (norm(title), norm(artist))


def main():
    real_path, existing_path = sys.argv[1], sys.argv[2]
    report_path = sys.argv[3] if len(sys.argv) > 3 else None

    entries = [json.loads(l) for l in open(real_path)]
    existing = json.load(open(existing_path))

    ex_by_id = {str(t["id"]): t for t in existing}
    ex_by_isrc = {}
    ex_by_key = {}
    for t in existing:
        if t.get("isrc"):
            ex_by_isrc.setdefault(t["isrc"], t)
        ex_by_key.setdefault(tkey(t["title"], t.get("artist") or ""), t)

    matched_ids = set()
    entry_rows = []
    counts = Counter()
    for e in entries:
        g = e["golden"]
        if e.get("skipped"):
            counts["not_admitted"] += 1
            entry_rows.append((g, "not-admitted", 0, 0))
            continue
        items = e.get("items") or []
        if not items:
            counts["gap"] += 1
            entry_rows.append((g, "gap" + (" (error)" if e.get("error") else ""), 0, 0))
            continue
        by_id = by_fuzzy = 0
        for it in items:
            t = ex_by_id.get(str(it["ref"]))
            if t is None and it.get("isrc"):
                t = ex_by_isrc.get(it["isrc"])
            if t is not None:
                by_id += 1
                matched_ids.add(str(t["id"]))
                continue
            t = ex_by_key.get(tkey(it["title"], (it.get("artists") or [""])[0]))
            if t is not None:
                by_fuzzy += 1
                matched_ids.add(str(t["id"]))
        if by_id + by_fuzzy == 0:
            counts["new_vs_existing"] += 1
            status = "all-new"
        elif by_id + by_fuzzy == len(items):
            counts["fully_matched"] += 1
            status = "matched"
        else:
            counts["partially_matched"] += 1
            status = "partial"
        entry_rows.append((g, status, by_id, by_fuzzy))

    extra = [t for t in existing if str(t["id"]) not in matched_ids]

    print("# Delta report: realized GM vs existing Tidal Scaruffi playlist\n")
    print(f"- GM entries realized: {len(entries)}")
    for k in ("fully_matched", "partially_matched", "all_new_vs_existing" if False else "new_vs_existing", "gap", "not_admitted"):
        print(f"- {k}: {counts[k]}")
    print(f"- existing playlist tracks: {len(existing)}; matched by some entry: {len(matched_ids)}; unmatched (extra in existing): {len(extra)}\n")

    print("## Per-entry status (non-matched only)\n")
    for g, status, by_id, by_fuzzy in entry_rows:
        if status == "matched":
            continue
        print(f"- [{status}] {g.get('artist','?')} — {g['title']}  (id-match {by_id}, fuzzy {by_fuzzy})  · {g.get('note','')[:80]}")

    print("\n## Existing-playlist tracks no realized entry claimed (sample, first 60)\n")
    for t in extra[:60]:
        print(f"- {t.get('artist','?')} — {t['title']}  (album: {t.get('album','?')})")
    if len(extra) > 60:
        print(f"… and {len(extra) - 60} more")

    if report_path:
        rep = json.load(open(report_path))
        absents = [i for i in rep["items"] if i["disposition"] == "absent"]
        marginals = [i for i in rep["items"] if i.get("marginal")]
        print(f"\n## Curate-report context: {len(absents)} absent, {len(marginals)} marginal at curate time\n")
        for i in absents:
            print(f"- absent: {i.get('note','')[:100]}")


if __name__ == "__main__":
    main()
