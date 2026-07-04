from tidalist.core.catalog import Track
from tidalist.core.diff import (
    EntryStatus,
    PlaylistDelta,
    diff_playlist,
    delta_to_dict,
    format_delta,
    norm,
    tkey,
)


# --- fixtures ----------------------------------------------------------------
# Three rendering records: fully matched by id, partial with one fuzzy title
# match, and a gap. Mirrors the shape scripts/render_dump.py writes per line.

def _rendering():
    return [
        {
            "index": 0,
            "golden": {"kind": "track", "title": "Glad", "artist": "Traffic",
                       "admitted": True, "note": "signature", "isrc": "GB1"},
            "items": [{"ref": "1", "title": "Glad", "artists": ["Traffic"],
                       "isrc": "GB1", "quality": "isrc"}],
            "compromises": [],
            "gap": False,
        },
        {
            "index": 1,
            "golden": {"kind": "track", "title": "Empty Pages Session", "artist": "Traffic",
                       "admitted": True, "note": "two takes", "isrc": None},
            "items": [
                {"ref": "2", "title": "Empty Pages", "artists": ["Traffic"],
                 "isrc": None, "quality": "strong"},
                {"ref": "3", "title": "Feelin Alright", "artists": ["Traffic"],
                 "isrc": None, "quality": "weak"},
            ],
            "compromises": [],
            "gap": False,
        },
        {
            "index": 2,
            "golden": {"kind": "track", "title": "Obscure B-side", "artist": "Traffic",
                       "admitted": True, "note": "never released", "isrc": None},
            "items": [],
            "compromises": [],
            "gap": True,
        },
    ]


def _existing():
    return [
        Track(id="1", title="Glad", artists=("Traffic",), isrc="GB1", album="Mr Fantasy"),
        Track(id="99", title="Empty Pages (Remastered)", artists=("Traffic",),
              isrc=None, album="John Barleycorn Must Die"),
        Track(id="50", title="Dear Mr Fantasy", artists=("Traffic",), isrc="GB2", album="Mr Fantasy"),
        Track(id="51", title="Paper Sun", artists=("Traffic",), isrc="GB3", album="Traffic"),
    ]


def _report():
    return {"items": [
        {"disposition": "absent", "note": "dropped: bootleg only"},
        {"disposition": "included", "marginal": True, "note": "borderline live take"},
    ]}


# --- norm / tkey (ported verbatim from scripts/compare_playlists.py) --------

def test_norm_folds_accents_and_strips_parentheticals():
    assert norm("Café (Live)") == "cafe"


def test_norm_empty_for_falsy_input():
    assert norm(None) == ""
    assert norm("") == ""


def test_tkey_pairs_normalized_title_and_artist():
    assert tkey("Empty Pages (Remastered)", "Traffic") == ("empty pages", "traffic")


# --- diff_playlist -----------------------------------------------------------

def test_diff_playlist_counts_classify_each_entry():
    delta = diff_playlist(_rendering(), _existing(), _report())
    assert delta.counts["fully_matched"] == 1
    assert delta.counts["partially_matched"] == 1
    assert delta.counts["new_vs_existing"] == 0
    assert delta.counts["gap"] == 1
    assert delta.counts["not_admitted"] == 0
    assert delta.counts["existing_total"] == 4
    assert delta.counts["existing_matched"] == 2


def test_diff_playlist_report_context_counted():
    delta = diff_playlist(_rendering(), _existing(), _report())
    assert delta.counts["report_absent"] == 1
    assert delta.counts["report_marginal"] == 1
    assert delta.report_notes == ("dropped: bootleg only",)


def test_diff_playlist_report_context_absent_when_no_report():
    delta = diff_playlist(_rendering(), _existing(), None)
    assert "report_absent" not in delta.counts
    assert delta.report_notes == ()


def test_diff_playlist_additions_are_unmatched_rendered_refs():
    delta = diff_playlist(_rendering(), _existing(), None)
    assert delta.additions == ("3",)


def test_diff_playlist_unclaimed_is_existing_tracks_no_entry_claimed():
    delta = diff_playlist(_rendering(), _existing(), None)
    assert {t.id for t in delta.unclaimed} == {"50", "51"}
    assert all(isinstance(t, Track) for t in delta.unclaimed)


def test_diff_playlist_entries_carry_per_entry_classification():
    delta = diff_playlist(_rendering(), _existing(), None)
    by_index = {e.index: e for e in delta.entries}
    assert by_index[0].status == "matched"
    assert by_index[0].matched == 1 and by_index[0].fuzzy == 0
    assert by_index[1].status == "partial"
    assert by_index[1].matched == 0 and by_index[1].fuzzy == 1
    assert by_index[2].status == "gap"
    assert by_index[2].title == "Obscure B-side"


def test_diff_playlist_skipped_entry_is_not_admitted():
    rendering = [{
        "index": 0,
        "golden": {"kind": "track", "title": "Cover Version", "artist": "Traffic",
                   "admitted": False, "note": "likely a cover"},
        "skipped": "not admitted",
    }]
    delta = diff_playlist(rendering, [], None)
    assert delta.counts["not_admitted"] == 1
    assert delta.entries[0].status == "not-admitted"


def test_diff_playlist_gap_with_error_is_flagged():
    rendering = [{
        "index": 0,
        "golden": {"kind": "track", "title": "Broken", "artist": "Traffic", "note": ""},
        "items": [],
        "gap": True,
        "error": "TypeError: boom",
    }]
    delta = diff_playlist(rendering, [], None)
    assert delta.entries[0].status == "gap (error)"


def test_diff_playlist_all_new_entry_when_nothing_matches():
    rendering = [{
        "index": 0,
        "golden": {"kind": "track", "title": "Brand New", "artist": "Traffic", "note": ""},
        "items": [{"ref": "999", "title": "Brand New", "artists": ["Traffic"],
                   "isrc": None, "quality": "weak"}],
        "gap": False,
    }]
    delta = diff_playlist(rendering, _existing(), None)
    assert delta.counts["new_vs_existing"] == 1
    assert delta.entries[0].status == "all-new"
    assert delta.additions == ("999",)


# --- format_delta -------------------------------------------------------------

def test_format_delta_has_stable_headings():
    text = format_delta(diff_playlist(_rendering(), _existing(), _report()))
    assert "# Delta report" in text
    assert "## Per-entry status" in text
    assert "## Existing-playlist tracks no rendered entry claimed" in text
    assert "## Curate-report context" in text


def test_format_delta_omits_curate_section_without_report():
    text = format_delta(diff_playlist(_rendering(), _existing(), None))
    assert "## Curate-report context" not in text


def test_format_delta_excludes_matched_entries_from_per_entry_section():
    text = format_delta(diff_playlist(_rendering(), _existing(), None))
    assert "Empty Pages Session" in text  # partial entry listed
    assert "Obscure B-side" in text       # gap entry listed
    lines = text.splitlines()
    per_entry_start = lines.index("## Per-entry status (non-matched only)")
    per_entry_section = "\n".join(lines[per_entry_start:per_entry_start + 6])
    assert "Glad" not in per_entry_section  # fully-matched entry suppressed


def test_format_delta_lists_unclaimed_existing_tracks():
    text = format_delta(diff_playlist(_rendering(), _existing(), None))
    assert "Dear Mr Fantasy" in text
    assert "Paper Sun" in text


# --- delta_to_dict -------------------------------------------------------------

def test_delta_to_dict_round_trips_core_fields():
    delta = diff_playlist(_rendering(), _existing(), _report())
    d = delta_to_dict(delta)
    assert d["counts"]["fully_matched"] == 1
    assert d["additions"] == ["3"]
    assert {u["id"] for u in d["unclaimed"]} == {"50", "51"}
    assert len(d["entries"]) == 3
    assert d["entries"][0]["status"] == "matched"
