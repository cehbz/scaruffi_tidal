import json

import pytest

from tidalist.config import AppConfig
from tidalist.core.recording import Candidate, Credit, Recording, Performance
from tidalist.core.criteria import Verdict
from tidalist.core.provenance import Provenance
from tidalist.core.brief import Brief
from tidalist.core.golden import GoldenEntry, GoldenPlaylist
from tidalist.core.realize import Realization, RealizedEntry, PlatformItem, MatchQuality
from tidalist.core.spec import to_golden
from tidalist import cli
from tests.fakes import FakeMetadataProvider, FakeRealizer


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


def _realizer():
    return FakeRealizer({"Glad": PlatformItem(ref="T-glad", title="Glad",
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


def test_format_realization_shows_resolved_and_gaps():
    item = PlatformItem(ref="T-glad", title="Glad", artists=("Traffic",),
                        quality=MatchQuality.ISRC)
    r = Realization("Winwood", (RealizedEntry(_entry("Glad"), items=(item,)),
                                RealizedEntry(_entry("Obscure"))))
    text = cli.format_realization(r)
    assert "Glad" in text and "T-glad" in text and "isrc" in text
    assert "Obscure" in text and "gap" in text.lower()
    assert "1 gap" in text


def test_format_realization_shows_album_track_count():
    from tidalist.core.album import Album
    from tidalist.core.golden import GoldenEntry
    from tidalist.core.provenance import Provenance
    from tidalist.core.criteria import Verdict

    t1 = PlatformItem(ref="t1", title="Glad", artists=("Traffic",))
    t2 = PlatformItem(ref="t2", title="Freedom Rider", artists=("Traffic",))
    album = Album(artist="Traffic", title="John Barleycorn Must Die")
    golden_entry = GoldenEntry(album, Provenance("nl"), Verdict.ok())
    r = Realization("Traffic Albums", (RealizedEntry(golden_entry, items=(t1, t2)),))
    text = cli.format_realization(r)
    assert "2 tracks" in text
    assert "John Barleycorn Must Die" in text


def test_format_realization_shows_compromise_note():
    from tidalist.core.album import Album
    from tidalist.core.golden import GoldenEntry
    from tidalist.core.provenance import Provenance
    from tidalist.core.criteria import Verdict
    from tidalist.core.fidelity import Compromise

    t1 = PlatformItem(ref="t1", title="Glad", artists=("Traffic",))
    album = Album(artist="Traffic", title="John Barleycorn Must Die")
    golden_entry = GoldenEntry(album, Provenance("nl"), Verdict.ok())
    comp = Compromise("edition", "steven wilson", "(none)", "preferred edition unavailable")
    r = Realization("Traffic Albums", (
        RealizedEntry(golden_entry, items=(t1,), compromises=(comp,)),
    ))
    text = cli.format_realization(r)
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


def test_realize_golden_returns_realization_with_gaps():
    r = cli.realize_golden(_golden_dict(), _realizer())
    assert isinstance(r, Realization)
    assert [g.item.title for g in r.gaps()] == ["Obscure"]


def test_publish_golden_emits_resolved_and_returns_reference():
    realizer = _realizer()
    ref = cli.publish_golden(_golden_dict(), realizer)
    name, refs, returned = realizer.emitted[-1]
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


def test_main_realize_prints_resolved_and_gaps(tmp_path, capsys):
    path = tmp_path / "golden.json"
    path.write_text(json.dumps(_golden_dict()))
    rc = cli.main(["realize", str(path)],
                  config_loader=lambda path=None: _cfg(tmp_path),
                  realizer_factory=lambda cfg: _realizer())
    out = capsys.readouterr().out
    assert rc == 0 and "T-glad" in out and "Obscure" in out


def test_main_publish_emits_and_prints_reference(tmp_path, capsys):
    path = tmp_path / "golden.json"
    path.write_text(json.dumps(_golden_dict()))
    realizer = _realizer()
    rc = cli.main(["publish", str(path)],
                  config_loader=lambda path=None: _cfg(tmp_path),
                  realizer_factory=lambda cfg: realizer)
    assert rc == 0 and realizer.emitted


def test_main_run_pipeline_curates_then_publishes(tmp_path, capsys):
    intent_path = tmp_path / "intent.json"
    intent_path.write_text(json.dumps(_intent_dict()))
    realizer = _realizer()
    rc = cli.main(["run", str(intent_path)],
                  config_loader=lambda path=None: _cfg(tmp_path),
                  metadata_factory=lambda cfg: _meta(),
                  realizer_factory=lambda cfg: realizer)
    assert rc == 0 and realizer.emitted


def test_main_unknown_command_exits():
    with pytest.raises(SystemExit):
        cli.main(["frobnicate"])

