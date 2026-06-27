"""tidalist CLI: curate a golden playlist, review it, realize and publish to a platform.

Presentation only — the domain use cases (curate, realize, publish) live in core. Verbs
operate on files: `curate` turns an intent JSON into a golden JSON; `review` prints it;
`realize` resolves it onto the platform (no write); `publish` creates the playlist; `run`
chains curate → realize → publish.
"""

import argparse
import json
import sys
from pathlib import Path

from .config import AppConfig
from .core.golden import Curator
from .core.realize import realize, publish, Realization
from .core.spec import to_golden, from_golden, to_intent
from .nl.intent import parse_intent
from .scaruffi.parse import parse_scaruffi


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


def format_realization(realization: Realization) -> str:
    gaps = realization.gaps()
    head = (f"{realization.name} — {len(realization.resolved())} resolved, "
            f"{len(gaps)} gap{'' if len(gaps) == 1 else 's'}")
    lines = [head]
    for e in realization.entries:
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


def scaruffi_intent(html: str, *, name: str = "Scaruffi Classical") -> dict:
    candidates, provenances, brief = parse_scaruffi(html, name=name)
    return to_intent(brief, candidates, provenances)


def realize_golden(golden_data: dict, realizer) -> Realization:
    return realize(from_golden(golden_data), realizer)


def publish_golden(golden_data: dict, realizer) -> str:
    return publish(realize(from_golden(golden_data), realizer), realizer)


# --- adapter construction (composition root; touches real services) ----------

def build_metadata(config: AppConfig):
    from .metadata.mirror import MirrorDB
    from .metadata.mb_mirror import MusicBrainzMetadata
    from .metadata.discogs_mirror import DiscogsMetadata
    from .metadata.composite import Metadata
    db = MirrorDB(config.musicbrainz_db, config.discogs_db)
    return Metadata(MusicBrainzMetadata(db), DiscogsMetadata(db))


def build_realizer(config: AppConfig):
    from .tidal.session import authenticate
    from .tidal.platform import TidalPlatform
    from .realize.tidal import TidalRealizer
    return TidalRealizer(TidalPlatform(authenticate(config.session_file)))


# --- dispatch ----------------------------------------------------------------

def main(argv=None, *, config_loader=AppConfig.load,
         metadata_factory=build_metadata, realizer_factory=build_realizer, out=None) -> int:
    out = out or sys.stdout
    args = _parser().parse_args(argv)

    if args.command == "scaruffi":
        _write_json(scaruffi_intent(_read_text(args.html), name=args.name), args.output, out)
        return 0

    if args.command == "curate":
        golden = curate_golden(_read_json(args.intent), metadata_factory(config_loader(args.config)))
        _write_json(golden, args.output, out)
        return 0

    if args.command == "review":
        print(format_golden(from_golden(_read_json(args.golden))), file=out)
        return 0

    if args.command == "realize":
        realization = realize_golden(_read_json(args.golden),
                                     realizer_factory(config_loader(args.config)))
        print(format_realization(realization), file=out)
        return 0

    if args.command == "publish":
        realizer = realizer_factory(config_loader(args.config))
        realization = realize_golden(_read_json(args.golden), realizer)
        ref = publish(realization, realizer)
        print(format_realization(realization), file=out)
        print(f"published: {ref}", file=out)
        return 0

    if args.command == "run":
        config = config_loader(args.config)
        golden = curate_golden(_read_json(args.intent), metadata_factory(config))
        if args.output:
            _write_json(golden, args.output, out)
        realizer = realizer_factory(config)
        realization = realize_golden(golden, realizer)
        print(format_realization(realization), file=out)
        print(f"published: {publish(realization, realizer)}", file=out)
        return 0

    return 1


def _parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="tidalist",
                                description="Curate a golden playlist, then realize it onto a platform.")
    p.add_argument("--config", default=None, help="path to config.yaml (default: XDG)")
    sub = p.add_subparsers(dest="command", required=True)

    sc = sub.add_parser("scaruffi", help="parse Scaruffi's classical HTML into an intent JSON")
    sc.add_argument("html", help="Scaruffi classical HTML path (or - for stdin)")
    sc.add_argument("-o", "--output", default=None, help="write intent JSON here (default: stdout)")
    sc.add_argument("--name", default="Scaruffi Classical", help="playlist name")

    c = sub.add_parser("curate", help="build a golden playlist from an intent JSON")
    c.add_argument("intent", help="intent JSON path (or - for stdin)")
    c.add_argument("-o", "--output", default=None, help="write golden JSON here (default: stdout)")

    r = sub.add_parser("review", help="print the golden playlist with verdicts")
    r.add_argument("golden", help="golden JSON path")

    rz = sub.add_parser("realize", help="resolve the golden onto the platform (no write)")
    rz.add_argument("golden", help="golden JSON path")

    pb = sub.add_parser("publish", help="resolve and create the platform playlist")
    pb.add_argument("golden", help="golden JSON path")

    run = sub.add_parser("run", help="curate → realize → publish in one go")
    run.add_argument("intent", help="intent JSON path (or - for stdin)")
    run.add_argument("-o", "--output", default=None, help="also write the golden JSON here")
    return p


def _read_text(path: str) -> str:
    return sys.stdin.read() if path == "-" else Path(path).read_text()


def _read_json(path: str) -> dict:
    return json.loads(_read_text(path))


def _write_json(data: dict, output, out) -> None:
    text = json.dumps(data, indent=2, ensure_ascii=False)
    if output:
        Path(output).write_text(text)
    else:
        print(text, file=out)


if __name__ == "__main__":
    sys.exit(main())
