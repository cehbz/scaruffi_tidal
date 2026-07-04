package curate

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/cehbz/tidalist/catalog"
	"github.com/cehbz/tidalist/core"
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

// TestMaterializeAlbumEntryMBNoUnverifiable pins the MB semantics: MB's
// release_group_secondary_type_join model is complete, so an RG with zero
// secondary-type rows is a POSITIVE observation ("neither live nor compilation"
// — the album analog of PerfStudio), not an unobservable fact. rg-jbmd carries
// no traits and the brief carries no_live; the report must NOT flag unverifiable.
func TestMaterializeAlbumEntryMBNoUnverifiable(t *testing.T) {
	_, rep := materializeFixture(t)
	if len(rep.Items[0].Unverifiable) != 0 {
		t.Errorf("MB-resolved album with zero secondary types must carry no unverifiable flags; got %v",
			rep.Items[0].Unverifiable)
	}
}

const discogsOnlyAlbumSelectionsJSON = `{
  "name": "Discogs-only Test",
  "brief": {"criteria": [{"type": "not_live"}]},
  "selections": [
    {"kind": "album", "discogs_master_id": 69017,
     "provenance": {"source": "test", "note": "discogs-only pick"}}
  ]
}`

// TestMaterializeAlbumEntryDiscogsOnly: a selection with no rg_mbid but a valid
// discogs_master_id must resolve via catalog.AlbumByMaster, not fall through to
// the absent stub (the ~5/267-item loss this task fixes). Discogs carries no
// secondary-type facts, so the brief's not_live criterion admits (permissive)
// and the report flags it unverifiable rather than a violation.
func TestMaterializeAlbumEntryDiscogsOnly(t *testing.T) {
	m := newTestMirror(t)
	sel, err := ParseSelections([]byte(discogsOnlyAlbumSelectionsJSON))
	if err != nil {
		t.Fatalf("ParseSelections: %v", err)
	}
	doc, rep, err := Materialize(m, sel)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if rep.Items[0].Disposition != "resolved" {
		t.Fatalf("disposition = %q, want resolved (Discogs-only must not fall to absent)", rep.Items[0].Disposition)
	}
	var e map[string]any
	if err := json.Unmarshal(doc.Entries[0], &e); err != nil {
		t.Fatal(err)
	}
	if e["discogs_master_id"] != float64(69017) {
		t.Errorf("discogs_master_id = %v, want 69017", e["discogs_master_id"])
	}
	if _, hasMBID := e["mbid"]; hasMBID {
		t.Errorf("Discogs-only entry must omit mbid; got %v", e["mbid"])
	}
	if e["artist"] != "Traffic" || e["title"] != "John Barleycorn Must Die" {
		t.Errorf("artist/title = %v / %v", e["artist"], e["title"])
	}
	if e["year"] != float64(1970) {
		t.Errorf("year = %v, want 1970", e["year"])
	}
	tl, ok := e["tracklist"].([]any)
	if !ok || len(tl) == 0 {
		t.Fatalf("tracklist must be non-empty, got %v", e["tracklist"])
	}
	if srcs, ok := e["sources"].([]any); !ok || len(srcs) != 1 || srcs[0] != "discogs" {
		t.Errorf("sources = %v, want [discogs]", e["sources"])
	}
	v := e["verdict"].(map[string]any)
	if v["admitted"] != true {
		t.Errorf("no traits observed must admit (permissive-flagged); verdict = %v", v)
	}
	var flagged bool
	for _, u := range rep.Items[0].Unverifiable {
		if strings.Contains(u, "not_live") {
			flagged = true
		}
	}
	if !flagged {
		t.Errorf("report unverifiable must flag not_live; got %v", rep.Items[0].Unverifiable)
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

func TestMaterializeAlbumCreditsCarryLatinVariants(t *testing.T) {
	m := newTestMirror(t)
	sel := Selections{
		Name:  "variant-test",
		Items: []Selection{{Kind: "album", RGMBID: "rg-sacre-gergiev"}},
	}
	doc, _, err := Materialize(m, sel)
	if err != nil {
		t.Fatal(err)
	}
	var entry struct {
		Credits []struct {
			Artist   string   `json:"artist"`
			Role     string   `json:"role"`
			Variants []string `json:"variants"`
		} `json:"credits"`
	}
	if err := json.Unmarshal(doc.Entries[0], &entry); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, c := range entry.Credits {
		// The Gergiev credit here arrives with role "artist" (the release-group's
		// artist-credit arm of the fixture), not "conductor" — this assertion
		// checks only the artist string and variants; don't expect a conductor
		// role from this fixture entry.
		if c.Artist == "Валерий Гергиев" {
			found = true
			if len(c.Variants) != 1 || c.Variants[0] != "Valery Gergiev" {
				t.Errorf("Gergiev credit variants = %v, want [Valery Gergiev]", c.Variants)
			}
		}
	}
	if !found {
		t.Fatalf("no Gergiev credit in %d credits", len(entry.Credits))
	}
}

// TestVariantMemoGetMatchesDirect: variantMemo.get must return exactly what
// m.LatinAliasVariants returns directly, for both a hit (Gergiev's Cyrillic
// primary name resolves to a Latin alias) and a miss (a Latin-primary name
// resolves but carries no distinct Latin alias) — and repeated calls must stay
// consistent (proves the cache never diverges from the source of truth).
func TestVariantMemoGetMatchesDirect(t *testing.T) {
	m := newTestMirror(t)
	vm := newVariantMemo(m)

	wantHit, err := m.LatinAliasVariants("Валерий Гергиев", core.RoleConductor)
	if err != nil {
		t.Fatal(err)
	}
	gotHit, err := vm.get("Валерий Гергиев", core.RoleConductor)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotHit, wantHit) {
		t.Errorf("vm.get(hit) = %v, want %v", gotHit, wantHit)
	}

	wantMiss, err := m.LatinAliasVariants("Peter Phillips", core.RoleConductor)
	if err != nil {
		t.Fatal(err)
	}
	gotMiss, err := vm.get("Peter Phillips", core.RoleConductor)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotMiss, wantMiss) {
		t.Errorf("vm.get(miss) = %v, want %v", gotMiss, wantMiss)
	}

	gotHitAgain, err := vm.get("Валерий Гергиев", core.RoleConductor)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotHitAgain, gotHit) {
		t.Errorf("vm.get repeated = %v, want %v (consistent with first call)", gotHitAgain, gotHit)
	}
}

// TestVariantMemoGetCachesPerKey inspects the unexported memo map directly
// (same package): two calls for the same (role, name) must leave exactly one
// entry, and a negative result (no variants) must be cached too — the whole
// point of memoizing is that the common no-alias case doesn't re-run FTS.
func TestVariantMemoGetCachesPerKey(t *testing.T) {
	m := newTestMirror(t)
	vm := newVariantMemo(m)

	if _, err := vm.get("Валерий Гергиев", core.RoleConductor); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.get("Валерий Гергиев", core.RoleConductor); err != nil {
		t.Fatal(err)
	}
	if len(vm.memo) != 1 {
		t.Errorf("memo has %d entries after two calls for the same key, want 1 (%v)", len(vm.memo), vm.memo)
	}

	if _, err := vm.get("Peter Phillips", core.RoleConductor); err != nil {
		t.Fatal(err)
	}
	key := string(core.RoleConductor) + "\x00" + "Peter Phillips"
	v, ok := vm.memo[key]
	if !ok {
		t.Fatalf("negative result for %q must be cached; memo=%v", key, vm.memo)
	}
	if len(v) != 0 {
		t.Errorf("Peter Phillips variants = %v, want empty", v)
	}
	if len(vm.memo) != 2 {
		t.Errorf("memo has %d entries, want 2 (Gergiev + Phillips); %v", len(vm.memo), vm.memo)
	}
}
