"""tidalist CLI: curate a golden playlist, review it, render and publish to a platform.

Presentation only — the domain use cases (curate, render, publish, diff) live in core.
Verbs operate on files: `curate` turns an intent JSON into a golden JSON; `review` prints
it; `render` resolves it onto the platform (no write); `publish` creates the playlist;
`diff` compares a rendering against an existing platform playlist; `run` chains
curate → render → publish.
"""

import argparse
import json
import sys
from pathlib import Path

from .config import AppConfig
from .core.catalog import Track
from .core.diff import PlaylistDelta, delta_to_dict, diff_playlist, format_delta
from .core.golden import Curator
from .core.render import render, publish, Rendering
from .core.spec import to_golden, from_golden
from .nl.intent import parse_intent


# --- presentation ------------------------------------------------------------

def format_golden(golden) -> str:
    admitted = sum(1 for e in golden.entries if e.verdict.admitted)
    lines = [f"{golden.name} — {len(golden.entries)} entries, {admitted} admitted"]
    for e in golden.entries:
        r = e.item
        mark = "✓" if e.verdict.admitted else "✗"
        line = f"  {mark} {r.artist} — {r.title}{_recmeta(r)}"
        if not e.verdict.admitted:
            line += "  — " + "; ".join(e.verdict.violations)
        lines.append(line)
    return "\n".join(lines)


def format_rendering(rendering: Rendering) -> str:
    gaps = rendering.gaps()
    head = (f"{rendering.name} — {len(rendering.resolved())} resolved, "
            f"{len(gaps)} gap{'' if len(gaps) == 1 else 's'}")
    lines = [head]
    for e in rendering.entries:
        r = e.golden.item
        if e.is_gap:
            lines.append(f"  ✗ {r.artist} — {r.title}  — gap (no platform match)")
        elif len(e.items) == 1:
            item = e.items[0]
            line = f"  ✓ {r.artist} — {r.title} → {item.ref}  [{item.quality.value}]"
            if e.compromises:
                line += "  [compromise: " + "; ".join(c.note for c in e.compromises) + "]"
            lines.append(line)
        else:
            line = f"  ✓ {r.artist} — {r.title} → {len(e.items)} tracks"
            if e.compromises:
                line += "  [compromise: " + "; ".join(c.note for c in e.compromises) + "]"
            lines.append(line)
    return "\n".join(lines)


def _recmeta(r) -> str:
    performance = getattr(r, "performance", None)
    perf_str = performance.value if performance is not None and performance.value != "unknown" else None
    bits = [b for b in (perf_str, str(r.first_released) if r.first_released else None) if b]
    return f"  [{', '.join(bits)}]" if bits else ""


# --- verb use cases (dependency-injected; no I/O) ----------------------------

def curate_golden(intent: dict, metadata) -> dict:
    candidates, provenances, brief = parse_intent(intent)
    return to_golden(Curator(metadata).curate(brief, candidates, provenances))


def render_golden(golden_data: dict, renderer) -> Rendering:
    return render(from_golden(golden_data), renderer)


def publish_golden(golden_data: dict, renderer) -> str:
    return publish(render(from_golden(golden_data), renderer), renderer)


def diff_rendering(rendering_records: list[dict], platform, playlist_id: str,
                    existing: list[Track] | None, report: dict | None) -> PlaylistDelta:
    """Diff a parsed rendering against an existing platform playlist.

    `existing`, when given, bypasses the live fetch (an offline snapshot); otherwise
    the current tracks are fetched from `platform`.
    """
    tracks = existing if existing is not None else platform.playlist_tracks(playlist_id)
    return diff_playlist(rendering_records, tracks, report)


# --- adapter construction (composition root; touches real services) ----------

def build_metadata(config: AppConfig):
    from .metadata.mirror import MirrorDB
    from .metadata.mb_mirror import MusicBrainzMetadata
    from .metadata.discogs_mirror import DiscogsMetadata
    from .metadata.composite import Metadata
    db = MirrorDB(config.musicbrainz_db, config.discogs_db)
    return Metadata(MusicBrainzMetadata(db), DiscogsMetadata(db))


def build_renderer(config: AppConfig):
    from .tidal.session import authenticate
    from .tidal.platform import TidalPlatform
    from .render.tidal import TidalRenderer
    return TidalRenderer(TidalPlatform(authenticate(config.session_file)))


def build_platform(config: AppConfig):
    from .tidal.session import authenticate
    from .tidal.platform import TidalPlatform
    return TidalPlatform(authenticate(config.session_file))


# --- dispatch ----------------------------------------------------------------

def main(argv=None, *, config_loader=AppConfig.load,
         metadata_factory=build_metadata, renderer_factory=build_renderer,
         platform_factory=build_platform, out=None) -> int:
    out = out or sys.stdout
    args = _parser().parse_args(argv)

    if args.command == "curate":
        golden = curate_golden(_read_json(args.intent), metadata_factory(config_loader(args.config)))
        _write_json(golden, args.output, out)
        return 0

    if args.command == "review":
        print(format_golden(from_golden(_read_json(args.golden))), file=out)
        return 0

    if args.command == "render":
        rendering = render_golden(_read_json(args.golden),
                                  renderer_factory(config_loader(args.config)))
        print(format_rendering(rendering), file=out)
        return 0

    if args.command == "publish":
        renderer = renderer_factory(config_loader(args.config))
        rendering = render_golden(_read_json(args.golden), renderer)
        ref = publish(rendering, renderer)
        print(format_rendering(rendering), file=out)
        print(f"published: {ref}", file=out)
        return 0

    if args.command == "diff":
        rendering_records = _read_jsonl(args.rendering)
        report = _read_json(args.report) if args.report else None
        if args.existing:
            existing = _tracks_from_snapshot(_read_json(args.existing))
            platform = None
        else:
            existing = None
            platform = platform_factory(config_loader(args.config))
        delta = diff_rendering(rendering_records, platform, args.playlist_id, existing, report)
        print(format_delta(delta), file=out)
        if args.json_out:
            Path(args.json_out).write_text(
                json.dumps(delta_to_dict(delta), indent=2, ensure_ascii=False))
        return 0

    if args.command == "run":
        config = config_loader(args.config)
        golden = curate_golden(_read_json(args.intent), metadata_factory(config))
        if args.output:
            _write_json(golden, args.output, out)
        renderer = renderer_factory(config)
        rendering = render_golden(golden, renderer)
        print(format_rendering(rendering), file=out)
        print(f"published: {publish(rendering, renderer)}", file=out)
        return 0

    return 1


def _parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="tidalist",
                                description="Curate a golden playlist, then render it onto a platform.")
    p.add_argument("--config", default=None, help="path to config.yaml (default: XDG)")
    sub = p.add_subparsers(dest="command", required=True)

    c = sub.add_parser("curate", help="build a golden playlist from an intent JSON")
    c.add_argument("intent", help="intent JSON path (or - for stdin)")
    c.add_argument("-o", "--output", default=None, help="write golden JSON here (default: stdout)")

    r = sub.add_parser("review", help="print the golden playlist with verdicts")
    r.add_argument("golden", help="golden JSON path")

    rz = sub.add_parser("render", help="resolve the golden onto the platform (no write)")
    rz.add_argument("golden", help="golden JSON path")

    pb = sub.add_parser("publish", help="resolve and create the platform playlist")
    pb.add_argument("golden", help="golden JSON path")

    df = sub.add_parser("diff", help="diff a rendering against an existing platform playlist")
    df.add_argument("rendering", help="rendering JSONL path (from `render_dump.py`)")
    df.add_argument("--playlist-id", required=True, help="existing platform playlist id to diff against")
    df.add_argument("--existing", default=None,
                    help="existing-playlist snapshot JSON (bypasses the live fetch)")
    df.add_argument("--report", default=None, help="curate-report JSON for additional context")
    df.add_argument("--json", dest="json_out", default=None,
                    help="write the delta JSON artifact here")

    run = sub.add_parser("run", help="curate → render → publish in one go")
    run.add_argument("intent", help="intent JSON path (or - for stdin)")
    run.add_argument("-o", "--output", default=None, help="also write the golden JSON here")
    return p


def _read_text(path: str) -> str:
    return sys.stdin.read() if path == "-" else Path(path).read_text()


def _read_json(path: str) -> dict:
    return json.loads(_read_text(path))


def _read_jsonl(path: str) -> list[dict]:
    return [json.loads(line) for line in _read_text(path).splitlines() if line.strip()]


def _tracks_from_snapshot(data: list[dict]) -> list[Track]:
    """Convert an existing-playlist snapshot ([{id,title,isrc,artist,album}, ...]) to Tracks."""
    return [Track(id=str(t["id"]), title=t["title"], artists=(t.get("artist") or "Unknown",),
                  isrc=t.get("isrc"), album=t.get("album"))
            for t in data]


def _write_json(data: dict, output, out) -> None:
    text = json.dumps(data, indent=2, ensure_ascii=False)
    if output:
        Path(output).write_text(text)
    else:
        print(text, file=out)


if __name__ == "__main__":
    sys.exit(main())
