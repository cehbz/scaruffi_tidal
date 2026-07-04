#!/usr/bin/env python3
"""Render a golden JSON against real Tidal (read-only) and dump structured results.

Writes one JSON line per golden entry to the output path as it goes (crash-safe,
resumable by skipping already-dumped indices), so a long run over hundreds of
albums loses nothing. No playlist is created.

Usage: uv run python scripts/render_dump.py gm.json rendering.jsonl
"""

import json
import sys
import time

from tidalist.config import AppConfig
from tidalist.core.spec import from_golden
from tidalist.core.recording import Recording
from tidalist.core.album import Album
from tidalist.core.edition import EditionPolicy
from tidalist.cli import build_renderer


def entry_ident(e):
    item = e.item
    d = {"kind": "album" if isinstance(item, Album) else "track",
         "title": item.title, "admitted": e.verdict.admitted,
         "note": e.provenance.note}
    if isinstance(item, Album):
        d["artist"] = item.artist
        d["mbid"] = item.ids.mbid
        d["tracks"] = len(item.tracklist)
    else:
        d["artist"] = item.artist
        d["mbid"] = item.mbid
        d["isrc"] = item.isrc
    return d


def main():
    gm_path, out_path = sys.argv[1], sys.argv[2]
    golden = from_golden(json.load(open(gm_path)))

    done = set()
    try:
        with open(out_path) as f:
            for line in f:
                done.add(json.loads(line)["index"])
        print(f"resuming: {len(done)} entries already dumped", flush=True)
    except FileNotFoundError:
        pass

    renderer = build_renderer(AppConfig.load())
    default_pref = EditionPolicy.default()

    out = open(out_path, "a")
    t0 = time.time()
    for i, e in enumerate(golden.entries):
        if i in done:
            continue
        rec = {"index": i, "golden": entry_ident(e)}
        if not e.verdict.admitted:
            rec["skipped"] = "not admitted"
        else:
            try:
                gap_reason = None
                if isinstance(e.item, Recording):
                    pi, comps = renderer.resolve(e.item)
                    items = [pi] if pi is not None else []
                else:
                    pref = e.edition if e.edition is not None else default_pref
                    items, comps, gap_reason = renderer.resolve_album(e.item, pref)
                rec["items"] = [{"ref": it.ref, "title": it.title,
                                 "artists": list(it.artists), "isrc": it.isrc,
                                 "quality": str(it.quality)} for it in items]
                rec["compromises"] = [{"facet": c.facet, "desired": c.desired,
                                       "used": c.used, "note": c.note} for c in comps]
                rec["gap"] = not items
                if rec["gap"]:
                    rec["gap_reason"] = gap_reason
            except Exception as ex:  # keep the sweep alive; the error IS the datum
                rec["error"] = f"{type(ex).__name__}: {ex}"
                rec["gap"] = True
        out.write(json.dumps(rec) + "\n")
        out.flush()
        el = time.time() - t0
        print(f"[{i+1}/{len(golden.entries)}] {rec['golden']['title'][:60]!r} "
              f"items={len(rec.get('items', []))} gap={rec.get('gap')} ({el:.0f}s)",
              flush=True)
    out.close()
    print("done", flush=True)


if __name__ == "__main__":
    main()
