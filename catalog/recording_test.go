package catalog

import (
	"testing"

	"github.com/cehbz/tidalist/core"
)

func TestFindRecordingByTitleAndArtist(t *testing.T) {
	m := newTestMirror(t)
	res, err := m.FindRecording(RecordingQuery{Title: "Dear Mr. Fantasy", ArtistName: "Traffic", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 1 {
		t.Fatalf("artist filter should yield exactly the Traffic recording; got %d", len(res.Candidates))
	}
	c := res.Candidates[0]
	if c.MBID != "r-dmf" {
		t.Errorf("MBID = %q, want r-dmf", c.MBID)
	}
	if c.ISRC != "GBABC1234567" {
		t.Errorf("ISRC = %q, want GBABC1234567", c.ISRC)
	}
	if c.DurationS != 300 {
		t.Errorf("DurationS = %d, want 300 (ms/1000)", c.DurationS)
	}
	if c.Match.ArtistConfirmed == nil || !*c.Match.ArtistConfirmed {
		t.Error("artist_confirmed should be true when the artist filter applied")
	}
	if c.Match.TitleDistance == nil || *c.Match.TitleDistance != 0 {
		t.Errorf("title_distance should be 0 for an exact title; got %v", c.Match.TitleDistance)
	}
}

func TestFindRecordingTitleOnlyWhenNoArtist(t *testing.T) {
	m := newTestMirror(t)
	res, err := m.FindRecording(RecordingQuery{Title: "Dear Mr. Fantasy", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 2 {
		t.Fatalf("title-only should return both recordings; got %d", len(res.Candidates))
	}
	if res.Candidates[0].Match.ArtistConfirmed != nil {
		t.Error("artist_confirmed must be omitted when no artist filter applied")
	}
}

func TestFindRecordingISRCExact(t *testing.T) {
	m := newTestMirror(t)
	res, err := m.FindRecording(RecordingQuery{Title: "Dear Mr. Fantasy", ArtistName: "Traffic", ISRC: "GBABC1234567", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if res.Candidates[0].Match.ISRCExact == nil || !*res.Candidates[0].Match.ISRCExact {
		t.Error("isrc_exact should be true when the requested ISRC matches")
	}
}

func TestFindRecordingUnresolvedArtistReturnsEmpty(t *testing.T) {
	m := newTestMirror(t)

	res, err := m.FindRecording(RecordingQuery{Title: "Dear Mr. Fantasy", ArtistName: "Nonexistent Band", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 0 {
		t.Errorf("unresolved ArtistName should yield 0 candidates; got %d", len(res.Candidates))
	}

	res, err = m.FindRecording(RecordingQuery{Title: "Dear Mr. Fantasy", ArtistMBID: "bogus-gid", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 0 {
		t.Errorf("unresolved ArtistMBID should yield 0 candidates; got %d", len(res.Candidates))
	}
}

func TestFindRecordingByArtistMBID(t *testing.T) {
	m := newTestMirror(t)
	res, err := m.FindRecording(RecordingQuery{Title: "Dear Mr. Fantasy", ArtistMBID: "a-traffic", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 1 {
		t.Fatalf("ArtistMBID filter should yield exactly 1 candidate; got %d", len(res.Candidates))
	}
	c := res.Candidates[0]
	if c.MBID != "r-dmf" {
		t.Errorf("MBID = %q, want r-dmf", c.MBID)
	}
	if c.Match.ArtistConfirmed == nil || !*c.Match.ArtistConfirmed {
		t.Error("ArtistConfirmed should be non-nil and true when ArtistMBID resolved")
	}
}

func TestFindRecordingByWork(t *testing.T) {
	m := newTestMirror(t)
	res, err := m.FindRecording(RecordingQuery{Work: "Missa Papae Marcelli", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 3 {
		t.Fatalf("expected 3 recordings of the work, got %d", len(res.Candidates))
	}
	if res.Candidates[0].MBID != "r-kyrie" {
		t.Errorf("MBID = %q, want r-kyrie", res.Candidates[0].MBID)
	}
	if res.Candidates[0].ISRC != "GBCLASSICAL01" {
		t.Errorf("ISRC = %q, want GBCLASSICAL01", res.Candidates[0].ISRC)
	}
	if res.Candidates[0].DurationS != 360 {
		t.Errorf("DurationS = %d, want 360", res.Candidates[0].DurationS)
	}
}

func TestFindRecordingByWorkUnresolvedEmpty(t *testing.T) {
	m := newTestMirror(t)
	res, err := m.FindRecording(RecordingQuery{Work: "Nonexistent Work", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 0 {
		t.Errorf("an unresolved work must yield no recordings; got %d", len(res.Candidates))
	}
}

func TestFindRecordingByWorkCreditsAndUmbrella(t *testing.T) {
	m := newTestMirror(t)

	// Unfiltered: all three recordings of the work, each carrying performer credits.
	res, err := m.FindRecording(RecordingQuery{Work: "Missa Papae Marcelli", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 3 {
		t.Fatalf("expected 3 recordings of the work, got %d", len(res.Candidates))
	}

	// Filter by orchestra -> only r-kyrie (rec 31 has a different orchestra; rec 32 none).
	res, err = m.FindRecording(RecordingQuery{
		Work: "Missa Papae Marcelli", Limit: 10,
		Credits: core.Credits{{Role: core.RoleOrchestra, Name: "Tallis Scholars"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 1 || res.Candidates[0].MBID != "r-kyrie" {
		t.Fatalf("orchestra filter should select only r-kyrie; got %+v", res.Candidates)
	}

	// Conductor umbrella: a real conductor name selects rec 30.
	res, _ = m.FindRecording(RecordingQuery{
		Work: "Missa Papae Marcelli", Limit: 10,
		Credits: core.Credits{{Role: core.RoleConductor, Name: "Peter Phillips"}},
	})
	if len(res.Candidates) != 1 || res.Candidates[0].MBID != "r-kyrie" {
		t.Fatalf("conductor:Phillips should select r-kyrie; got %+v", res.Candidates)
	}

	// Conductor umbrella: a STANDALONE chorus-master name (rec 32) is reachable via --credit conductor:.
	res, _ = m.FindRecording(RecordingQuery{
		Work: "Missa Papae Marcelli", Limit: 10,
		Credits: core.Credits{{Role: core.RoleConductor, Name: "Barnaby Smith"}},
	})
	if len(res.Candidates) != 1 || res.Candidates[0].MBID != "r-mpm-acap" {
		t.Fatalf("conductor umbrella must match a standalone chorus_master; got %+v", res.Candidates)
	}
}

func TestFindRecordingByTitleFiltersByCredits(t *testing.T) {
	m := newTestMirror(t)

	// Recording r-kyrie ("Kyrie") carries conductor Peter Phillips and a
	// standalone chorus_master "Some Chorusmaster". The title path must apply
	// --credit uniformly, including the conductor→chorus_master umbrella.

	// Matching conductor → kept.
	res, err := m.FindRecording(RecordingQuery{
		Title:   "Kyrie",
		Credits: core.Credits{{Role: core.RoleConductor, Name: "Peter Phillips"}},
		Limit:   10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 1 || res.Candidates[0].MBID != "r-kyrie" {
		t.Fatalf("title-path conductor filter should keep only r-kyrie; got %+v", res.Candidates)
	}

	// Non-matching conductor → filtered out.
	res, err = m.FindRecording(RecordingQuery{
		Title:   "Kyrie",
		Credits: core.Credits{{Role: core.RoleConductor, Name: "Nonexistent Conductor"}},
		Limit:   10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 0 {
		t.Fatalf("title-path non-matching credit must filter out the candidate; got %+v", res.Candidates)
	}

	// Conductor umbrella reaches a standalone chorus_master on the title path too.
	res, err = m.FindRecording(RecordingQuery{
		Title:   "Kyrie",
		Credits: core.Credits{{Role: core.RoleConductor, Name: "Some Chorusmaster"}},
		Limit:   10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 1 || res.Candidates[0].MBID != "r-kyrie" {
		t.Fatalf("title-path conductor umbrella must match a chorus_master credit; got %+v", res.Candidates)
	}
}

// TestFindRecordingByWorkAliasCreditConductor checks that find-recording's
// --credit filter is alias-aware: the fixture's Gergiev/Sacre recording (63,
// r-sacre-gergiev) carries a Cyrillic-primary conductor credit (Валерий
// Гергиев, artist 66); a query for the Latin alias "Valery Gergiev" must
// still select it, the same variant-aware matching ResolvePerformance already
// gets via expandWants/creditsSatisfy.
func TestFindRecordingByWorkAliasCreditConductor(t *testing.T) {
	m := newTestMirror(t)
	res, err := m.FindRecording(RecordingQuery{
		Work: "Le Sacre du printemps", Limit: 10,
		Credits: core.Credits{{Role: core.RoleConductor, Name: "Valery Gergiev"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 1 || res.Candidates[0].MBID != "r-sacre-gergiev" {
		t.Fatalf("alias-aware conductor filter should select r-sacre-gergiev; got %+v", res.Candidates)
	}
}

// TestFindRecordingByWorkAliasCreditWrongNameFiltersOut guards against a
// recall regression from alias expansion: a conductor name that resolves to
// no known artist (so it expands to just itself) must still filter the
// Gergiev recording out, not accidentally match everything.
func TestFindRecordingByWorkAliasCreditWrongNameFiltersOut(t *testing.T) {
	m := newTestMirror(t)
	res, err := m.FindRecording(RecordingQuery{
		Work: "Le Sacre du printemps", Limit: 10,
		Credits: core.Credits{{Role: core.RoleConductor, Name: "Wrong Conductor"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 0 {
		t.Fatalf("a non-matching conductor name must filter out the recording; got %+v", res.Candidates)
	}
}

func TestFindRecordingByWorkUsesWorkGroupNotTop1(t *testing.T) {
	m := newTestMirror(t)
	// The existing Palestrina case still works (single work, no movements).
	res, err := m.FindRecording(RecordingQuery{Work: "Missa Papae Marcelli", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) == 0 {
		t.Fatal("Missa Papae Marcelli must still list its recordings via the work-group path")
	}
}

func TestFindRecordingByWorkWarnsWhenTruncated(t *testing.T) {
	m := newTestMirror(t)
	res, err := m.FindRecording(RecordingQuery{Work: "Missa Papae Marcelli", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) == 0 {
		t.Error("unfiltered --work truncated by limit must emit a warning")
	}
}

func TestRecordingByGID(t *testing.T) {
	m := newTestMirror(t)
	info, ok, err := m.RecordingByGID("r-glad")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("r-glad must resolve")
	}
	if info.MBID != "r-glad" || info.Title != "Glad" {
		t.Errorf("identity = %q %q", info.MBID, info.Title)
	}
	if info.ArtistCredit != "Traffic" {
		t.Errorf("artist credit = %q, want Traffic", info.ArtistCredit)
	}
	if info.Album != "John Barleycorn Must Die" {
		t.Errorf("album = %q", info.Album)
	}
	if info.Year != 1970 {
		t.Errorf("year = %d, want 1970", info.Year)
	}
	if info.DurationS != 419 {
		t.Errorf("duration = %d, want 419", info.DurationS)
	}

	if _, ok, err := m.RecordingByGID("r-nope"); err != nil || ok {
		t.Errorf("unknown gid: ok=%v err=%v, want ok=false err=nil", ok, err)
	}
}
