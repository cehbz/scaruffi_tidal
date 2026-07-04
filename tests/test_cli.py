import json

import pytest

from tidalist.config import AppConfig
from tidalist.core.catalog import Track
from tidalist.core.diff import PlaylistDelta
from tidalist.core.recording import Candidate, Credit, Recording, Performance
from tidalist.core.criteria import Verdict
from tidalist.core.provenance import Provenance
from tidalist.core.brief import Brief
from tidalist.core.golden import GoldenEntry, GoldenPlaylist
from tidalist.core.render import Rendering, RenderedEntry, PlatformItem, MatchQuality
from tidalist.core.spec import to_golden
from tidalist import cli
from tests.fakes import FakeMetadataProvider, FakePlatform, FakeRenderer


# --- fixtures ----------------------------------------------------------------

def _entry(title, admitted=True, reasons=("cover",), artist="Traffic"):
    rec = Recording(artist=artist, title=title, mbid="mb-1",
                    performance=Performance.STUDIO, first_released=1970,
                    credits=(Credit("Steve Winwood", "performer"),))
    verdict = Verdict.ok() if admitted else Verdict.rejected(*reasons)
    return GoldenEntry(rec, Provenance("nl", "note"), verdict)


def _golden(*entries, name="Winwood"):
    return GoldenPlaylist(name, Brief(name, ()), tuple(entries))


def _golden_dict():
    return to_golden(_golden(_entry("Glad"), _entry("Obscure")))


def _intent_dict():
    return {
        "name": "Winwood",
        "brief": {"criteria": [{"type": "performed_by", "artist": "Steve Winwood"}]},
        "candidates": [{"artist": "Traffic", "title": "Glad", "note": "signature"}],
    }


def _meta():
    rec = Recording(artist="Traffic", title="Glad", mbid="mb-1",
                    credits=(Credit("Steve Winwood", "performer"),))
    return FakeMetadataProvider({"Glad": rec})


def _renderer():
    return FakeRenderer({"Glad": PlatformItem(ref="T-glad", title="Glad",
                                              artists=("Traffic",), quality=MatchQuality.ISRC)})


def _cfg(tmp_path):
    return AppConfig(config_dir=tmp_path)


# --- formatters --------------------------------------------------------------

def test_format_golden_lists_entries_and_rejection_reasons():
    text = cli.format_golden(_golden(_entry("Glad"),
                                     _entry("Feelin Alright", admitted=False,
                                            reasons=("likely a cover",))))
    assert "Winwood" in text
    assert "Glad" in text and "Feelin Alright" in text
    assert "likely a cover" in text


def test_format_golden_header_counts_admitted():
    text = cli.format_golden(_golden(_entry("A"), _entry("B", admitted=False, reasons=("x",))))
    assert "1 admitted" in text


def test_format_rendering_shows_resolved_and_gaps():
    item = PlatformItem(ref="T-glad", title="Glad", artists=("Traffic",),
                        quality=MatchQuality.ISRC)
    r = Rendering("Winwood", (RenderedEntry(_entry("Glad"), items=(item,)),
                              RenderedEntry(_entry("Obscure"))))
    text = cli.format_rendering(r)
    assert "Glad" in text and "T-glad" in text and "isrc" in text
    assert "Obscure" in text and "gap" in text.lower()
    assert "1 gap" in text


def test_format_rendering_shows_album_track_count():
    from tidalist.core.album import Album
    from tidalist.core.golden import GoldenEntry
    from tidalist.core.provenance import Provenance
    from tidalist.core.criteria import Verdict

    t1 = PlatformItem(ref="t1", title="Glad", artists=("Traffic",))
    t2 = PlatformItem(ref="t2", title="Freedom Rider", artists=("Traffic",))
    album = Album(artist="Traffic", title="John Barleycorn Must Die")
    golden_entry = GoldenEntry(album, Provenance("nl"), Verdict.ok())
    r = Rendering("Traffic Albums", (RenderedEntry(golden_entry, items=(t1, t2)),))
    text = cli.format_rendering(r)
    assert "2 tracks" in text
    assert "John Barleycorn Must Die" in text


def test_format_rendering_shows_compromise_note():
    from tidalist.core.album import Album
    from tidalist.core.golden import GoldenEntry
    from tidalist.core.provenance import Provenance
    from tidalist.core.criteria import Verdict
    from tidalist.core.fidelity import Compromise

    t1 = PlatformItem(ref="t1", title="Glad", artists=("Traffic",))
    album = Album(artist="Traffic", title="John Barleycorn Must Die")
    golden_entry = GoldenEntry(album, Provenance("nl"), Verdict.ok())
    comp = Compromise("edition", "steven wilson", "(none)", "preferred edition unavailable")
    r = Rendering("Traffic Albums", (
        RenderedEntry(golden_entry, items=(t1,), compromises=(comp,)),
    ))
    text = cli.format_rendering(r)
    assert "compromise" in text
    assert "preferred edition unavailable" in text


# --- verb use cases ----------------------------------------------------------

def test_curate_golden_builds_golden_from_intent_with_notes():
    golden = cli.curate_golden(_intent_dict(), _meta())
    assert golden["name"] == "Winwood"
    entry = golden["entries"][0]
    assert entry["title"] == "Glad"
    assert entry["provenance"]["note"] == "signature"
    assert entry["verdict"]["admitted"] is True


def test_render_golden_returns_rendering_with_gaps():
    r = cli.render_golden(_golden_dict(), _renderer())
    assert isinstance(r, Rendering)
    assert [g.item.title for g in r.gaps()] == ["Obscure"]


def test_publish_golden_emits_resolved_and_returns_reference():
    renderer = _renderer()
    ref = cli.publish_golden(_golden_dict(), renderer)
    name, refs, returned = renderer.emitted[-1]
    assert refs == ["T-glad"] and ref == returned


# --- main dispatch -----------------------------------------------------------

def test_main_curate_writes_golden_file(tmp_path):
    intent_path = tmp_path / "intent.json"
    intent_path.write_text(json.dumps(_intent_dict()))
    out_path = tmp_path / "golden.json"
    rc = cli.main(["curate", str(intent_path), "-o", str(out_path)],
                  config_loader=lambda path=None: _cfg(tmp_path),
                  metadata_factory=lambda cfg: _meta())
    assert rc == 0
    golden = json.loads(out_path.read_text())
    assert golden["entries"][0]["title"] == "Glad"


def test_main_review_prints_entries(tmp_path, capsys):
    path = tmp_path / "golden.json"
    path.write_text(json.dumps(_golden_dict()))
    rc = cli.main(["review", str(path)])
    assert rc == 0 and "Glad" in capsys.readouterr().out


def test_main_render_prints_resolved_and_gaps(tmp_path, capsys):
    path = tmp_path / "golden.json"
    path.write_text(json.dumps(_golden_dict()))
    rc = cli.main(["render", str(path)],
                  config_loader=lambda path=None: _cfg(tmp_path),
                  renderer_factory=lambda cfg: _renderer())
    out = capsys.readouterr().out
    assert rc == 0 and "T-glad" in out and "Obscure" in out


def test_main_publish_emits_and_prints_reference(tmp_path, capsys):
    path = tmp_path / "golden.json"
    path.write_text(json.dumps(_golden_dict()))
    renderer = _renderer()
    rc = cli.main(["publish", str(path)],
                  config_loader=lambda path=None: _cfg(tmp_path),
                  renderer_factory=lambda cfg: renderer)
    assert rc == 0 and renderer.emitted


def test_main_run_pipeline_curates_then_publishes(tmp_path, capsys):
    intent_path = tmp_path / "intent.json"
    intent_path.write_text(json.dumps(_intent_dict()))
    renderer = _renderer()
    rc = cli.main(["run", str(intent_path)],
                  config_loader=lambda path=None: _cfg(tmp_path),
                  metadata_factory=lambda cfg: _meta(),
                  renderer_factory=lambda cfg: renderer)
    assert rc == 0 and renderer.emitted


def test_main_unknown_command_exits():
    with pytest.raises(SystemExit):
        cli.main(["frobnicate"])


# --- diff verb -----------------------------------------------------------------

def _diff_rendering_records():
    return [
        {"index": 0, "golden": {"title": "Glad", "artist": "Traffic", "note": ""},
         "items": [{"ref": "1", "title": "Glad", "artists": ["Traffic"], "isrc": None, "quality": "isrc"}],
         "gap": False},
        {"index": 1, "golden": {"title": "Obscure", "artist": "Traffic", "note": ""},
         "items": [], "gap": True},
    ]


def _write_rendering_jsonl(tmp_path):
    path = tmp_path / "rendering.jsonl"
    lines = [json.dumps(r) for r in _diff_rendering_records()]
    path.write_text("\n".join(lines) + "\n")
    return path


def _existing_snapshot():
    return [
        {"id": "1", "title": "Glad", "isrc": None, "artist": "Traffic", "album": "Mr Fantasy"},
        {"id": "2", "title": "Paper Sun", "isrc": None, "artist": "Traffic", "album": "Traffic"},
    ]


def test_diff_rendering_uses_existing_override_without_touching_platform():
    existing = [Track(id="1", title="Glad", artists=("Traffic",))]
    delta = cli.diff_rendering(_diff_rendering_records(), None, "PL1", existing, None)
    assert isinstance(delta, PlaylistDelta)
    assert delta.counts["fully_matched"] == 1
    assert delta.counts["gap"] == 1


def test_diff_rendering_fetches_live_playlist_when_no_existing_override():
    live_track = Track(id="1", title="Glad", artists=("Traffic",))
    platform = FakePlatform([], playlist_tracks_map={"PL1": [live_track]})
    delta = cli.diff_rendering(_diff_rendering_records(), platform, "PL1", None, None)
    assert delta.counts["fully_matched"] == 1


def test_main_diff_with_existing_snapshot_prints_markdown_report(tmp_path, capsys):
    rendering_path = _write_rendering_jsonl(tmp_path)
    existing_path = tmp_path / "existing.json"
    existing_path.write_text(json.dumps(_existing_snapshot()))
    rc = cli.main(["diff", str(rendering_path), "--playlist-id", "PL1",
                  "--existing", str(existing_path)])
    out = capsys.readouterr().out
    assert rc == 0
    assert "# Delta report" in out
    assert "Paper Sun" in out  # unclaimed existing track


def test_main_diff_without_existing_uses_platform_factory(tmp_path, capsys):
    rendering_path = _write_rendering_jsonl(tmp_path)
    live_track = Track(id="1", title="Glad", artists=("Traffic",))
    platform = FakePlatform([], playlist_tracks_map={"PL1": [live_track]})
    rc = cli.main(["diff", str(rendering_path), "--playlist-id", "PL1"],
                  config_loader=lambda path=None: _cfg(tmp_path),
                  platform_factory=lambda cfg: platform)
    out = capsys.readouterr().out
    assert rc == 0 and "# Delta report" in out


def test_main_diff_writes_json_artifact(tmp_path):
    rendering_path = _write_rendering_jsonl(tmp_path)
    existing_path = tmp_path / "existing.json"
    existing_path.write_text(json.dumps(_existing_snapshot()))
    json_out = tmp_path / "delta.json"
    rc = cli.main(["diff", str(rendering_path), "--playlist-id", "PL1",
                  "--existing", str(existing_path), "--json", str(json_out)])
    assert rc == 0
    data = json.loads(json_out.read_text())
    assert data["counts"]["fully_matched"] == 1
    assert data["additions"] == []


def test_main_diff_with_report_includes_curate_context(tmp_path, capsys):
    rendering_path = _write_rendering_jsonl(tmp_path)
    existing_path = tmp_path / "existing.json"
    existing_path.write_text(json.dumps(_existing_snapshot()))
    report_path = tmp_path / "report.json"
    report_path.write_text(json.dumps({"items": [{"disposition": "absent", "note": "bootleg"}]}))
    rc = cli.main(["diff", str(rendering_path), "--playlist-id", "PL1",
                  "--existing", str(existing_path), "--report", str(report_path)])
    out = capsys.readouterr().out
    assert rc == 0 and "## Curate-report context" in out
