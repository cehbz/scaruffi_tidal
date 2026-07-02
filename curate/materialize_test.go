package curate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cehbz/tidalist/catalog"
	"github.com/cehbz/tidalist/internal/mirrorfixture"
)

func newTestMirror(t *testing.T) *catalog.MirrorDB {
	t.Helper()
	mb, dc, err := mirrorfixture.Build(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m, err := catalog.Open(mb, dc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	return m
}

const selectionsJSON = `{
  "name": "Test Playlist",
  "brief": {"criteria": [{"type": "no_live"}]},
  "selections": [
    {"kind": "album", "rg_mbid": "rg-jbmd",
     "edition": {"prefer_original": true},
     "provenance": {"source": "test", "note": "the JBMD pick"},
     "alternatives": [{"rg_mbid": "rg-other", "why": "later remaster"}]},
    {"kind": "album", "rg_mbid": "rg-best",
     "criteria": [{"type": "not_compilation"}],
     "provenance": {"source": "test", "note": "comp decoy"}},
    {"kind": "track", "recording_mbid": "r-glad",
     "criteria": [{"type": "studio"}],
     "provenance": {"source": "test", "note": "glad"},
     "marginal": "only one candidate considered"},
    {"kind": "album", "rg_mbid": "rg-nope", "artist": "Nobody", "title": "Nothing",
     "provenance": {"source": "test", "note": "absent"}}
  ]
}`

func materializeFixture(t *testing.T) (GoldenDoc, Report) {
	t.Helper()
	m := newTestMirror(t)
	sel, err := ParseSelections([]byte(selectionsJSON))
	if err != nil {
		t.Fatalf("ParseSelections: %v", err)
	}
	doc, rep, err := Materialize(m, sel)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	return doc, rep
}

func TestMaterializeAlbumEntry(t *testing.T) {
	doc, rep := materializeFixture(t)
	if doc.Name != "Test Playlist" {
		t.Errorf("name = %q", doc.Name)
	}
	if len(doc.Entries) != 4 {
		t.Fatalf("want 4 entries (absent included, reviewable), got %d", len(doc.Entries))
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Entries []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	e := got.Entries[0]
	if e["kind"] != "album" || e["mbid"] != "rg-jbmd" {
		t.Errorf("entry[0] identity: %v %v", e["kind"], e["mbid"])
	}
	if e["artist"] != "Traffic" || e["title"] != "John Barleycorn Must Die" {
		t.Errorf("entry[0] artist/title: %v / %v", e["artist"], e["title"])
	}
	if e["year"] != float64(1970) {
		t.Errorf("entry[0] year: %v", e["year"])
	}
	if e["discogs_master_id"] != float64(69017) {
		t.Errorf("entry[0] discogs_master_id: %v", e["discogs_master_id"])
	}
	tl, ok := e["tracklist"].([]any)
	if !ok || len(tl) != 2 {
		t.Fatalf("entry[0] tracklist: %v", e["tracklist"])
	}
	first := tl[0].(map[string]any)
	if first["position"] != float64(1) || first["title"] != "Glad" {
		t.Errorf("tracklist[0]: %v", first)
	}
	v := e["verdict"].(map[string]any)
	if v["admitted"] != true {
		t.Errorf("entry[0] verdict: %v", v)
	}
	if rep.Items[0].Disposition != "resolved" {
		t.Errorf("report[0] disposition = %q", rep.Items[0].Disposition)
	}
	if len(rep.Items[0].Alternatives) != 1 {
		t.Errorf("report[0] alternatives = %v", rep.Items[0].Alternatives)
	}
}

func TestMaterializeGateRejectsCompilation(t *testing.T) {
	doc, rep := materializeFixture(t)
	b, _ := json.Marshal(doc.Entries[1])
	var e map[string]any
	_ = json.Unmarshal(b, &e)
	v := e["verdict"].(map[string]any)
	if v["admitted"] != false {
		t.Fatalf("compilation must be rejected by not_compilation; verdict=%v", v)
	}
	viol := v["violations"].([]any)
	if len(viol) == 0 || !strings.Contains(viol[0].(string), "compilation") {
		t.Errorf("violations = %v", viol)
	}
	if rep.Items[1].Admitted {
		t.Error("report[1] must show not admitted")
	}
}

func TestMaterializeTrackEntryUnverifiableStudio(t *testing.T) {
	doc, rep := materializeFixture(t)
	b, _ := json.Marshal(doc.Entries[2])
	var e map[string]any
	_ = json.Unmarshal(b, &e)
	if e["kind"] != "track" || e["mbid"] != "r-glad" {
		t.Errorf("entry[2] identity: %v %v", e["kind"], e["mbid"])
	}
	if e["performance"] != "unknown" {
		t.Errorf("performance = %v, want unknown (required by the Python reader)", e["performance"])
	}
	if e["artist"] != "Traffic" || e["album"] != "John Barleycorn Must Die" {
		t.Errorf("artist/album: %v / %v", e["artist"], e["album"])
	}
	v := e["verdict"].(map[string]any)
	if v["admitted"] != true {
		t.Errorf("unknown performance under studio must admit (permissive-flagged); verdict=%v", v)
	}
	var flagged bool
	for _, u := range rep.Items[2].Unverifiable {
		if strings.Contains(u, "studio") {
			flagged = true
		}
	}
	if !flagged {
		t.Errorf("report[2] unverifiable must flag studio; got %v", rep.Items[2].Unverifiable)
	}
	if rep.Items[2].Marginal == "" {
		t.Error("report[2] must carry the marginal passthrough")
	}
}

func TestMaterializeAbsentEntryRejected(t *testing.T) {
	doc, rep := materializeFixture(t)
	b, _ := json.Marshal(doc.Entries[3])
	var e map[string]any
	_ = json.Unmarshal(b, &e)
	if e["artist"] != "Nobody" || e["title"] != "Nothing" {
		t.Errorf("absent stub identity: %v / %v", e["artist"], e["title"])
	}
	v := e["verdict"].(map[string]any)
	if v["admitted"] != false {
		t.Errorf("absent must be rejected; verdict=%v", v)
	}
	if rep.Items[3].Disposition != "absent" {
		t.Errorf("report[3] disposition = %q", rep.Items[3].Disposition)
	}
}

const classicalAlbumSelectionsJSON = `{
  "name": "Classical Credits Test",
  "brief": {"criteria": []},
  "selections": [
    {"kind": "album", "rg_mbid": "rg-a",
     "provenance": {"source": "test", "note": "classical credits"}}
  ]
}`

// TestMaterializeAlbumEntryCredits: an album entry backed by a classical RG (rg-a,
// four movements each carrying a conductor/orchestra performer arc) must carry the
// aggregated role-tagged credits[] — sourced from the mirrors via
// catalog.ReleaseGroupCredits, never authored by the selections JSON.
func TestMaterializeAlbumEntryCredits(t *testing.T) {
	m := newTestMirror(t)
	sel, err := ParseSelections([]byte(classicalAlbumSelectionsJSON))
	if err != nil {
		t.Fatalf("ParseSelections: %v", err)
	}
	doc, _, err := Materialize(m, sel)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	var e struct {
		Credits []struct {
			Artist string `json:"artist"`
			Role   string `json:"role"`
		} `json:"credits"`
	}
	if err := json.Unmarshal(doc.Entries[0], &e); err != nil {
		t.Fatal(err)
	}
	roles := map[string]string{}
	for _, c := range e.Credits {
		roles[c.Role] = c.Artist
	}
	if roles["conductor"] != "Leonard Bernstein" {
		t.Errorf("conductor credit = %q, want Leonard Bernstein; credits=%+v", roles["conductor"], e.Credits)
	}
	if roles["orchestra"] != "New York Philharmonic" {
		t.Errorf("orchestra credit = %q, want New York Philharmonic; credits=%+v", roles["orchestra"], e.Credits)
	}
}

// TestMaterializeAlbumEntryCreditsOmittedWhenEmpty: back-compat — an album with no
// aggregable credits at all must omit the "credits" key entirely, not emit `[]`.
// (rg-jbmd always carries at least its RG artist credit, so this covers the wire
// shape via a stub RG that resolves to no album at all: the absent-entry path,
// which never reaches credit aggregation.)
func TestMaterializeAlbumEntryCreditsOmittedWhenEmpty(t *testing.T) {
	doc, _ := materializeFixture(t)
	if strings.Contains(string(doc.Entries[3]), `"credits"`) {
		t.Errorf("absent album entry must omit credits key entirely; got %s", doc.Entries[3])
	}
}

func TestMaterializeEmitsPythonCriteriaTags(t *testing.T) {
	doc, _ := materializeFixture(t)
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"not_live"`) {
		t.Errorf("brief must emit the Python tag not_live; got %s", s[:200])
	}
	if strings.Contains(s, `"no_live"`) || strings.Contains(s, `"no_compilation"`) {
		t.Error("Go-vocabulary tags must not leak into the golden JSON")
	}
}
