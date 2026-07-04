//go:build integration

package catalog

// Integration smoke tests for the catalog package.
//
// These tests open the real local MusicBrainz + Discogs SQLite mirrors and
// exercise one query per catalog tool, verifying that the ported SQL matches
// the real schema.  They are excluded from the normal `go test ./...` run and
// must be invoked explicitly:
//
//	go test -tags integration ./catalog/ -run Integration -v
//
// The test skips itself on any machine that lacks the mirror files.
//
// Anchors (verified live 2026-06-29):
//   - Traffic MB gid:            9fadfba9-ecae-4383-a4d8-47b043cea19a
//   - John Barleycorn Must Die   MB release-group gid: 3770d5ce-e0e1-3389-9acf-cd38f0722baf
//   - John Barleycorn Must Die   Discogs master id: 69017 (main release 583800)
//   - Recording "Glad" (Traffic) MB gid: 53bb54ac-1020-4cd4-83ce-362a58e1ec17, ISRC: GBUM71030667
//   - Work "Dear Mr. Fantasy"    MB gid: fa1ba832-28bd-3b2c-ad70-4f32f0e65b21 (125 recordings, composers: Steve Winwood, Chris Wood)
//   - JBMD canonical tracklist:  6 tracks; position 1 = "Glad"

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cehbz/tidalist/core"
)

const (
	defaultMBPath = "/Volumes/Crucial X10/musicbrainz/musicbrainz.db"
	defaultDCPath = "/Volumes/Crucial X10/discogs/discogs.db"

	trafficGID = "9fadfba9-ecae-4383-a4d8-47b043cea19a"
	jbmdRGGID  = "3770d5ce-e0e1-3389-9acf-cd38f0722baf"
	jbmdDCMID  = int64(69017)

	gladRecordingGID = "53bb54ac-1020-4cd4-83ce-362a58e1ec17"

	dmfWorkGID = "fa1ba832-28bd-3b2c-ad70-4f32f0e65b21"
)

// openRealMirror opens the real mirrors, skipping the test if either file is absent.
func openRealMirror(t *testing.T) *MirrorDB {
	t.Helper()
	mbPath := os.Getenv("TIDALIST_MUSICBRAINZ_DB")
	if mbPath == "" {
		mbPath = defaultMBPath
	}
	dcPath := os.Getenv("TIDALIST_DISCOGS_DB")
	if dcPath == "" {
		dcPath = defaultDCPath
	}
	if _, err := os.Stat(mbPath); err != nil {
		t.Skipf("MusicBrainz mirror not found at %s: %v", mbPath, err)
	}
	if _, err := os.Stat(dcPath); err != nil {
		t.Skipf("Discogs mirror not found at %s: %v", dcPath, err)
	}
	m, err := Open(mbPath, dcPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { m.Close() })
	return m
}

// TestIntegrationResolveArtist checks that "Traffic" resolves to the canonical
// Traffic artist (gid 9fadfba9-…) as one of the top candidates.
func TestIntegrationResolveArtist(t *testing.T) {
	m := openRealMirror(t)
	got, err := m.ResolveArtist("Traffic", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one artist candidate for 'Traffic'")
	}
	// The canonical Traffic band must appear in the results.
	var found bool
	for _, c := range got {
		if c.MBID == core.MBID(trafficGID) {
			found = true
			if c.Name == "" {
				t.Error("Traffic candidate has empty Name")
			}
			if c.Match.FTSRank == nil {
				t.Error("Traffic candidate missing fts_rank signal")
			}
			break
		}
	}
	if !found {
		t.Errorf("Traffic gid %s not found in %d candidates", trafficGID, len(got))
	}
}

// TestIntegrationFindRecordingByTitleArtist checks that searching "Glad" by
// Traffic returns the canonical recording with a known ISRC.
func TestIntegrationFindRecordingByTitleArtist(t *testing.T) {
	m := openRealMirror(t)
	got, err := m.FindRecording(RecordingQuery{Title: "Glad", ArtistName: "Traffic", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) == 0 {
		t.Fatal("expected at least one recording of 'Glad' by Traffic")
	}
	// The canonical JBMD Glad recording must appear.
	var found bool
	for _, c := range got.Candidates {
		if c.MBID == core.MBID(gladRecordingGID) {
			found = true
			if c.ISRC == "" {
				t.Error("Glad recording missing ISRC")
			}
			if c.Match.ArtistConfirmed == nil || !*c.Match.ArtistConfirmed {
				t.Error("artist_confirmed should be true when artist filter applied")
			}
			break
		}
	}
	if !found {
		t.Errorf("Glad recording gid %s not found in %d candidates", gladRecordingGID, len(got.Candidates))
	}
}

// TestIntegrationFindRecordingByWork checks that querying recordings linked to
// the "Dear Mr. Fantasy" work returns ≥1 result.
func TestIntegrationFindRecordingByWork(t *testing.T) {
	m := openRealMirror(t)
	got, err := m.FindRecording(RecordingQuery{Work: "Dear Mr. Fantasy", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) == 0 {
		t.Fatal("expected at least one recording linked to work 'Dear Mr. Fantasy'")
	}
	// Every result must have a gid and a title.
	for i, c := range got.Candidates {
		if c.MBID == "" {
			t.Errorf("result[%d] missing MBID", i)
		}
		if c.Title == "" {
			t.Errorf("result[%d] missing Title", i)
		}
	}
}

// TestIntegrationResolveWork checks that "Dear Mr. Fantasy" resolves to a work
// with the expected gid and composers Steve Winwood and Chris Wood.
func TestIntegrationResolveWork(t *testing.T) {
	m := openRealMirror(t)
	got, err := m.ResolveWork("Dear Mr. Fantasy", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one work for 'Dear Mr. Fantasy'")
	}
	var found bool
	for _, w := range got {
		if w.MBID != core.MBID(dmfWorkGID) {
			continue
		}
		found = true
		if len(w.Composers) == 0 {
			t.Error("Dear Mr. Fantasy work has no composers")
		}
		hasWinwood, hasWood := false, false
		for _, c := range w.Composers {
			if c == "Steve Winwood" {
				hasWinwood = true
			}
			if c == "Chris Wood" {
				hasWood = true
			}
		}
		if !hasWinwood {
			t.Errorf("composers %v missing Steve Winwood", w.Composers)
		}
		if !hasWood {
			t.Errorf("composers %v missing Chris Wood", w.Composers)
		}
		break
	}
	if !found {
		t.Errorf("Dear Mr. Fantasy work gid %s not found in %d results", dmfWorkGID, len(got))
	}
}

// TestIntegrationFindAlbum checks that searching "John Barleycorn Must Die" by
// Traffic returns both a MusicBrainz peer (with the rg gid) and a Discogs peer
// (with master id 69017).
func TestIntegrationFindAlbum(t *testing.T) {
	m := openRealMirror(t)
	got, err := m.FindAlbum(AlbumQuery{Title: "John Barleycorn Must Die", ArtistName: "Traffic", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one album candidate")
	}
	var foundMB, foundDC bool
	for _, c := range got {
		for _, src := range c.Sources {
			if src == core.SourceMusicBrainz && c.MBID == core.MBID(jbmdRGGID) {
				foundMB = true
			}
			if src == core.SourceDiscogs && c.DiscogsMasterID == core.DiscogsMasterID(jbmdDCMID) {
				foundDC = true
			}
		}
	}
	if !foundMB {
		t.Errorf("MB peer with rg gid %s not found among %d candidates", jbmdRGGID, len(got))
	}
	if !foundDC {
		t.Errorf("Discogs peer with master id %d not found among %d candidates", jbmdDCMID, len(got))
	}
}

// TestIntegrationTracklistByReleaseGroup checks that the canonical MB tracklist
// for JBMD has exactly 6 tracks and the first is "Glad".
func TestIntegrationTracklistByReleaseGroup(t *testing.T) {
	m := openRealMirror(t)
	tracks, err := m.TracklistByReleaseGroup(jbmdRGGID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 6 {
		t.Errorf("expected 6 canonical JBMD tracks, got %d", len(tracks))
	}
	if len(tracks) > 0 {
		if tracks[0].Title != "Glad" {
			t.Errorf("track 1 title = %q, want Glad", tracks[0].Title)
		}
		if tracks[0].MBID == "" {
			t.Error("track 1 missing MBID")
		}
	}
	for i, tk := range tracks {
		if tk.Position != i+1 {
			t.Errorf("track[%d].Position = %d, want %d", i, tk.Position, i+1)
		}
	}
}

// TestIntegrationTracklistByMaster checks that the Discogs main-release
// tracklist for JBMD master 69017 has ≥6 tracks.
func TestIntegrationTracklistByMaster(t *testing.T) {
	m := openRealMirror(t)
	tracks, err := m.TracklistByMaster(jbmdDCMID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) < 6 {
		t.Errorf("expected ≥6 Discogs JBMD tracks, got %d", len(tracks))
	}
	if len(tracks) > 0 {
		if tracks[0].Title == "" {
			t.Error("first Discogs track has empty title")
		}
		// The Discogs main release opens with "Glad".
		if tracks[0].Title != "Glad" {
			t.Errorf("Discogs track 1 title = %q, want Glad", tracks[0].Title)
		}
	}
}

// TestIntegrationAlbumEditionsMB checks that JBMD has multiple MB editions with
// key fields populated.
func TestIntegrationAlbumEditionsMB(t *testing.T) {
	m := openRealMirror(t)
	eds, err := m.AlbumEditionsMB(jbmdRGGID)
	if err != nil {
		t.Fatal(err)
	}
	if len(eds) < 5 {
		t.Errorf("expected ≥5 MB editions for JBMD, got %d", len(eds))
	}
	for i, e := range eds {
		if e.MBID == "" {
			t.Errorf("edition[%d] missing MBID", i)
		}
		if e.Title == "" {
			t.Errorf("edition[%d] missing Title", i)
		}
		if e.Source != core.SourceMusicBrainz {
			t.Errorf("edition[%d] source = %q, want musicbrainz", i, e.Source)
		}
	}
}

// TestIntegrationFindRecordingByWorkPerformerCredits checks that a classical
// work query returns candidates with performer Credits (conductor + orchestra)
// and that a conductor filter narrows the result set.
//
// Anchors (verified live 2026-06-30):
//   - Beethoven Symphony no. 5 in C minor, op. 67: I. Allegro con brio
//     work gid: b6f9ecc3-24d1-38ed-b8a0-091f7cd0c6b2 (758 recordings total)
//   - Herbert von Karajan: 24 recordings of this movement; 4 in the top-50
//     most-released recordings.
const (
	beethoven5ImvtWorkName = "Symphony no. 5 in C minor, op. 67: I. Allegro con brio"
	beethoven5ImvtWorkGID  = "b6f9ecc3-24d1-38ed-b8a0-091f7cd0c6b2"
	karajan                = "Herbert von Karajan"
)

func TestIntegrationFindRecordingByWorkPerformerCredits(t *testing.T) {
	m := openRealMirror(t)

	// (a) Unfiltered: top-10 candidates must carry Credits with conductor or
	// orchestra on at least one result.  The work has 758 recordings so the
	// truncation warning must appear.
	unfiltered, err := m.FindRecording(RecordingQuery{
		Work:  beethoven5ImvtWorkName,
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(unfiltered.Candidates) == 0 {
		t.Fatal("expected at least one recording for Beethoven 5th mvt I")
	}
	var foundPerformerCredit bool
	for _, c := range unfiltered.Candidates {
		for _, cr := range c.Credits {
			if cr.Role == core.RoleConductor || cr.Role == core.RoleOrchestra {
				foundPerformerCredit = true
				break
			}
		}
	}
	if !foundPerformerCredit {
		t.Error("expected at least one candidate with a conductor or orchestra credit")
	}
	if len(unfiltered.Warnings) == 0 {
		t.Error("expected a truncation warning for an unfiltered work with 758 recordings")
	}

	// (b) Conductor filter: limit=50 → Karajan appears in 4 of the top-50.
	// All returned candidates must have the Karajan conductor credit.
	filtered, err := m.FindRecording(RecordingQuery{
		Work:    beethoven5ImvtWorkName,
		Credits: core.Credits{{Role: core.RoleConductor, Name: karajan}},
		Limit:   50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Candidates) == 0 {
		t.Fatalf("expected at least one Beethoven 5th I recording by %s", karajan)
	}
	for i, c := range filtered.Candidates {
		if !c.Credits.MatchesRole(core.RoleConductor, karajan) {
			t.Errorf("filtered[%d] %q: missing conductor credit %q", i, c.Title, karajan)
		}
	}
}

// TestIntegrationFindRecordingByWorkConductorUmbrella checks that the
// --credit conductor: umbrella matches a recording whose director is credited
// as chorus_master only (link_type 152), not conductor (link_type 151).
//
// Anchor (verified live 2026-06-30):
//   - Magnus liber organi de graduali… gid: 376c5c49-a7c0-4642-bfe9-7edbc813dbe0
//     11 recordings, all directed by James O'Donnell (chorus_master only, no conductor).
const (
	magnusLiberWorkName   = "Magnus liber organi de graduali et antiphonario pro servitio divino"
	magnusLiberWorkGID    = "376c5c49-a7c0-4642-bfe9-7edbc813dbe0"
	magnusLiberChorusMstr = "James O’Donnell" // stored with RIGHT SINGLE QUOTATION MARK (U+2019)
	// magnusLiberQueryASCII is the same name typed with an ASCII apostrophe; the
	// credit filter must fold it to the stored U+2019 form (proves normalizeName).
	magnusLiberQueryASCII = "James O'Donnell"
)

func TestIntegrationFindRecordingByWorkConductorUmbrella(t *testing.T) {
	m := openRealMirror(t)

	got, err := m.FindRecording(RecordingQuery{
		Work:    magnusLiberWorkName,
		Credits: core.Credits{{Role: core.RoleConductor, Name: magnusLiberQueryASCII}},
		Limit:   20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) == 0 {
		t.Fatalf("conductor umbrella failed: expected recordings matched by %q (chorus_master)", magnusLiberChorusMstr)
	}
	// Every matched recording must carry a chorus_master credit for O'Donnell,
	// not a conductor credit (umbrella matched it, but the credit is chorus_master).
	for i, c := range got.Candidates {
		if c.Credits.Has(core.RoleConductor, magnusLiberChorusMstr) {
			t.Errorf("result[%d] %q: %s credited as conductor (expected chorus_master)", i, c.Title, magnusLiberChorusMstr)
		}
		if !c.Credits.Has(core.RoleChorusMaster, magnusLiberChorusMstr) {
			t.Errorf("result[%d] %q: %s chorus_master credit missing", i, c.Title, magnusLiberChorusMstr)
		}
	}
}

// TestIntegrationAlbumEditionsDiscogs checks that JBMD master 69017 has
// multiple Discogs editions and that exactly one is flagged as the main release.
func TestIntegrationAlbumEditionsDiscogs(t *testing.T) {
	m := openRealMirror(t)
	eds, err := m.AlbumEditionsDiscogs(jbmdDCMID)
	if err != nil {
		t.Fatal(err)
	}
	if len(eds) < 10 {
		t.Errorf("expected ≥10 Discogs editions for JBMD master 69017, got %d", len(eds))
	}
	mainCount := 0
	for i, e := range eds {
		if e.Title == "" {
			t.Errorf("edition[%d] missing Title", i)
		}
		if e.Source != core.SourceDiscogs {
			t.Errorf("edition[%d] source = %q, want discogs", i, e.Source)
		}
		if e.IsMainRelease {
			mainCount++
		}
	}
	if mainCount != 1 {
		t.Errorf("expected exactly 1 main release among %d editions, got %d", len(eds), mainCount)
	}
}

// TestIntegrationFindByAttributes is the live latency gate for the
// find-by-attributes style/genre/year discovery tool (Task 7): a style+year
// query with no title/artist hint must return candidates and complete well
// within an interactive budget, driving off dc.master_style (never a full
// dc.master scan — see the EXPLAIN QUERY PLAN check recorded in the task
// report).
func TestIntegrationFindByAttributes(t *testing.T) {
	m := openRealMirror(t)
	start := time.Now()
	got, err := m.FindByAttributes(AttributeQuery{
		Styles:   []string{"Krautrock"},
		YearFrom: 1970,
		YearTo:   1979,
		Limit:    25,
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one Krautrock candidate (1970-1979)")
	}
	if elapsed > 10*time.Second {
		t.Fatalf("interactive budget blown: %s > 10s", elapsed)
	}
	for i, c := range got {
		if i >= 5 {
			break
		}
		t.Logf("candidate[%d]: %q by %q (%d)", i, c.Title, c.Artist, c.Year)
	}
	t.Logf("Krautrock 1970-1979: %d candidates in %s", len(got), elapsed)
}

// composersContain reports whether any composer name folds to contain want (normalized).
func composersContain(cs []string, want string) bool {
	w := core.NormalizeName(want)
	for _, c := range cs {
		if strings.Contains(core.NormalizeName(c), w) {
			return true
		}
	}
	return false
}

// TestIntegrationResolvePerformance verifies the federated performance resolver against the
// real mirrors.  The design is ARTIST-FIRST: MB resolves the work-group and narrows to the
// performance(s) by the conjunctive performer credit AND; Discogs is corroboration via the
// bridged artist ids.
//
// Anchors (verified live 2026-07-01):
//   - Beethoven "Symphony no. 5 in C minor, op. 67" — op. 67 is unique to Beethoven, so the
//     work-group resolves from the title alone (parent work gid
//     d03bff61-26fc-301b-98ac-4d8e85771cbc, 4 movement sub-works).  Bernstein/NYPhil narrows
//     to 2 earliest-co-release clusters (1963: 5 movement recordings; 1981: 3), each carrying
//     the conductor+orchestra credit AND.
//   - Discogs bridge ids (materialised artist.discogs_artist_id): Beethoven 95544,
//     Bernstein 299702, New York Philharmonic 327356; Kleiber 833155, Wiener Philharmoniker
//     754974.  The canonical Kleiber/VPO DG 2530 516 is Discogs master 287096.
//   - Brahms "Ein deutsches Requiem" and Palestrina "Missa Papae Marcelli" — the motivating
//     classical compounds that came back EMPTY under the retired top-1 resolveWorkID; both now
//     resolve as multi-movement work-groups.
//
// LATENCY / COVERAGE NOTE (updated 2026-07-02): discogsPerformances is performer-driven
// (conductor ∩ orchestra intersection at release level) and interactive — the full
// Beethoven+Kleiber+VPO federated resolve measures ~7s on the real mirror and produces
// cross-source High (the canonical DG 2530 516, master 287096); see
// TestIntegrationResolvePerformanceDiscogsInteractive.  This test keeps the composer
// OMITTED (or composer-only) to exercise the MB-narrowed shapes specifically.
const beethoven5WorkGID = "d03bff61-26fc-301b-98ac-4d8e85771cbc"

func TestIntegrationResolvePerformance(t *testing.T) {
	m := openRealMirror(t)

	// (a) Artist-first MB narrowing.  Performer-constrained (conductor+orchestra) with the
	// composer OMITTED so discogsPerformances fast-exits (see the SCALE FINDING above); the
	// op. 67 title uniquely identifies Beethoven's 5th, so title-only work resolution is safe.
	res, err := m.ResolvePerformance(PerformanceQuery{
		Work: "Symphony no. 5 in C minor, op. 67",
		Credits: core.Credits{
			{Role: core.RoleConductor, Name: "Leonard Bernstein"},
			{Role: core.RoleOrchestra, Name: "New York Philharmonic"},
		},
		Limit: 25,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome == OutcomeAbsent || len(res.Performances) == 0 {
		t.Fatalf("Bernstein/NYPhil Beethoven 5 must resolve; outcome=%q perfs=%d", res.Outcome, len(res.Performances))
	}

	// Every returned performance must satisfy the requested performer credit AND.
	var withRecs *Performance
	for i := range res.Performances {
		p := &res.Performances[i]
		if !p.Credits.MatchesRole(core.RoleConductor, "Leonard Bernstein") {
			t.Errorf("performance[%d] missing conductor credit; matched=%v", i, p.Credits)
		}
		if !p.Credits.MatchesRole(core.RoleOrchestra, "New York Philharmonic") {
			t.Errorf("performance[%d] missing orchestra credit; matched=%v", i, p.Credits)
		}
		if withRecs == nil && len(p.Recordings) > 0 {
			withRecs = p
		}
	}
	if withRecs == nil {
		t.Fatal("a returned performance must carry MB movement recordings (the ISRC substrate)")
	}
	// Composer-less resolution must still land on the Beethoven work-group.
	if withRecs.Work.MBID != core.MBID(beethoven5WorkGID) {
		t.Errorf("resolved work-group gid = %q, want Beethoven 5 %q", withRecs.Work.MBID, beethoven5WorkGID)
	}
	if !composersContain(withRecs.Work.Composers, "Beethoven") {
		t.Errorf("resolved work-group composers %v missing Beethoven", withRecs.Work.Composers)
	}
	// The movement recordings carry MB identity (the ISRC substrate; real data may lack ISRCs).
	if withRecs.Recordings[0].MBID == "" || withRecs.Recordings[0].Title == "" {
		t.Errorf("movement recording missing identity: %+v", withRecs.Recordings[0])
	}
	isrcs := 0
	for _, r := range withRecs.Recordings {
		if r.ISRC != "" {
			isrcs++
		}
	}
	t.Logf("Beethoven5 Bernstein/NYPhil: outcome=%s perfs=%d first-perf year=%d recs=%d isrcs=%d conf=%s sources=%v work=%q composers=%v",
		res.Outcome, len(res.Performances), withRecs.FirstYear, len(withRecs.Recordings), isrcs,
		withRecs.Confidence, withRecs.Sources, withRecs.Work.Name, withRecs.Work.Composers)
	// Discogs cross-source corroboration is NOT asserted here: this MB-narrowed shape omits
	// the composer, so discogsPerformances fast-exits and the performance is MB-only / medium
	// by design.  The composer+performer federated shape is covered by
	// TestIntegrationResolvePerformanceDiscogsInteractive (~7s live).
	if withRecs.Confidence == ConfidenceHigh {
		t.Logf("NOTE: unexpected High confidence (Discogs corroborated) on the MB-narrowed path: master=%d label=%q",
			withRecs.DiscogsMaster, withRecs.Label)
	}

	// (b) The motivating classical compounds must no longer come back empty (they resolved to a
	// single wrong work — or none — under the retired top-1 resolveWorkID).  Composer-only, so
	// discogsPerformances fast-exits and the group resolves via the MB 281-parts spine.
	for _, c := range []struct{ name, work, composer string }{
		{"Brahms — Ein deutsches Requiem", "Ein deutsches Requiem", "Brahms"},
		{"Palestrina — Missa Papae Marcelli", "Missa Papae Marcelli", "Palestrina"},
	} {
		cr, err := m.ResolvePerformance(PerformanceQuery{
			Work:    c.work,
			Credits: core.Credits{{Role: core.RoleComposer, Name: c.composer}},
			Limit:   10,
		})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if cr.Outcome == OutcomeAbsent {
			t.Errorf("%s work-group must resolve (was empty under top-1 resolveWorkID); outcome=absent", c.name)
		}
		if len(cr.Performances) == 0 {
			t.Errorf("%s must surface at least one performance", c.name)
		}
		recs := 0
		if len(cr.Performances) > 0 {
			recs = len(cr.Performances[0].Recordings)
		}
		t.Logf("%s: outcome=%s perfs=%d first-perf recs=%d", c.name, cr.Outcome, len(cr.Performances), recs)
	}
}

// TestIntegrationResolvePerformanceDiscogsInteractive is the latency gate for the
// performer-driven discogsPerformances redesign: a composer+performer query is the shape
// that drives Discogs cross-source discovery/corroboration, and under the retired
// composer-driven discogsPerformances that same Kleiber/VPO query measured ~13 min cold
// on the real mirror (see TestIntegrationResolvePerformance's LATENCY / COVERAGE NOTE
// above). This test asserts the redesign completes within an interactive 60s budget and
// still lands on the canonical cross-source High anchor (DG 2530 516 / Discogs master
// 287096).
//
// If this fails on the missing High assertion, do not adjust thresholds or this test —
// the 2026-07-02 verification produced High on this anchor via the old reconciliation, so
// a regression here is a real defect. Debug with systematic-debugging first.
func TestIntegrationResolvePerformanceDiscogsInteractive(t *testing.T) {
	m := openRealMirror(t)
	q := PerformanceQuery{
		Work: "Symphony no. 5 in C minor, op. 67",
		Credits: core.Credits{
			{Role: core.RoleComposer, Name: "Ludwig van Beethoven"},
			{Role: core.RoleConductor, Name: "Carlos Kleiber"},
			{Role: core.RoleOrchestra, Name: "Wiener Philharmoniker"},
		},
	}
	start := time.Now()
	res, err := m.ResolvePerformance(q)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Kleiber/VPO resolve: %s, outcome=%s, %d performances", elapsed, res.Outcome, len(res.Performances))
	if elapsed > 60*time.Second {
		t.Fatalf("interactive budget blown: %s > 60s (composer-driven was ~13min)", elapsed)
	}
	foundHigh := false
	for _, p := range res.Performances {
		if p.Confidence == ConfidenceHigh && p.DiscogsMaster != 0 {
			foundHigh = true
		}
	}
	if !foundHigh {
		t.Fatal("want at least one cross-source High performance (the canonical DG 2530 516 / master 287096 anchor)")
	}
}

// TestIntegrationResolvePerformanceDiscogsExtreme re-runs the Bernstein/NYPhil worst case
// (5,216 candidates under the retired composer-driven discogsPerformances) under the same
// interactive 60s budget. No canonical cross-source High anchor is asserted here — that
// assertion belongs to the Kleiber/VPO case above — only that the query completes in
// budget and resolves to a non-empty outcome; the confidence distribution is logged for
// the record.
func TestIntegrationResolvePerformanceDiscogsExtreme(t *testing.T) {
	m := openRealMirror(t)
	q := PerformanceQuery{
		Work: "Symphony no. 5",
		Credits: core.Credits{
			{Role: core.RoleComposer, Name: "Ludwig van Beethoven"},
			{Role: core.RoleConductor, Name: "Leonard Bernstein"},
			{Role: core.RoleOrchestra, Name: "New York Philharmonic"},
		},
	}
	start := time.Now()
	res, err := m.ResolvePerformance(q)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Bernstein/NYPhil resolve: %s, outcome=%s, %d performances", elapsed, res.Outcome, len(res.Performances))
	if elapsed > 60*time.Second {
		t.Fatalf("interactive budget blown: %s > 60s", elapsed)
	}
	if res.Outcome == "" {
		t.Fatal("want a non-empty outcome")
	}
	confCounts := map[Confidence]int{}
	for _, p := range res.Performances {
		confCounts[p.Confidence]++
	}
	t.Logf("Bernstein/NYPhil confidence counts: %v", confCounts)
}

// TestIntegrationSingleShotResolution asserts the resolver-refinements gates
// (docs/superpowers/plans/2026-07-03-resolver-refinements.md Task 5): each
// e2e friction case from the 2026-07-02 Scaruffi run
// (docs/superpowers/runs/2026-07-02-scaruffi-e2e/) must now resolve in ONE
// call — no resolve-artist/resolve-work diagnosis step before a curate agent
// can accept a result. The classical ResolvePerformance sub-cases below pass
// MBOnly (per CURATE.md's documented recipe for classical items — Discogs
// discovery for a prolific composer is minutes-scale and orthogonal to what's
// under test here: work-group identity, not cross-source corroboration).
func TestIntegrationSingleShotResolution(t *testing.T) {
	m := openRealMirror(t)

	// (a) Ensemble alias: "Berlin Philharmonic" (English alias) role-resolves to
	// the orchestra whose MB primary name is the German "Berliner Philharmoniker"
	// — not an FTS-shadowing decoy. Pre-fix this required a resolve-artist
	// diagnosis step (TODO.md, 2026-07-02 e2e curate friction).
	t.Run("berlin_philharmonic_role_resolution", func(t *testing.T) {
		start := time.Now()
		id, ok, err := m.resolveArtistIDForRole("Berlin Philharmonic", core.RoleOrchestra)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatal("expected \"Berlin Philharmonic\" to resolve to an artist")
		}
		var name string
		if err := m.DB.QueryRow(`SELECT name FROM artist WHERE id = ?`, id).Scan(&name); err != nil {
			t.Fatalf("fetch resolved artist name: %v", err)
		}
		if name != "Berliner Philharmoniker" {
			t.Errorf("resolved artist name = %q, want %q", name, "Berliner Philharmoniker")
		}
		t.Logf("Berlin Philharmonic -> artist id=%d name=%q (%s)", id, name, elapsed)
	})

	// (b) Cyrillic-primary credit through find-recording: the mirror's Валерий
	// Гергиев is queried by his Latin alias "Valery Gergiev"; the find-recording
	// --credit filter must be alias-aware (Task 3) to match him at all.
	t.Run("gergiev_find_recording_conductor_filter", func(t *testing.T) {
		start := time.Now()
		got, err := m.FindRecording(RecordingQuery{
			Work:    "Le Sacre du printemps",
			Credits: core.Credits{{Role: core.RoleConductor, Name: "Valery Gergiev"}},
			Limit:   20,
		})
		elapsed := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Candidates) == 0 {
			t.Fatal("expected >0 Gergiev-conducted \"Le Sacre du printemps\" recordings via find-recording; got 0")
		}
		t.Logf("Gergiev/Sacre find-recording: %d candidates, work_resolution=%q, warnings=%v (%s)",
			len(got.Candidates), got.WorkResolution, got.Warnings, elapsed)
	})

	// (c) Cross-language work title: "The Rite of Spring" (English) must resolve
	// the Sacre family even though the MB root's canonical title is French and
	// carries no English alias itself — recovery is via a movement-level
	// work_alias + the 281 walk (Task 2).
	t.Run("rite_of_spring_resolve_performance", func(t *testing.T) {
		start := time.Now()
		res, err := m.ResolvePerformance(PerformanceQuery{
			Work:   "The Rite of Spring",
			MBOnly: true,
			Credits: core.Credits{
				{Role: core.RoleComposer, Name: "Igor Stravinsky"},
				{Role: core.RoleConductor, Name: "Valery Gergiev"},
			},
			Limit: 25,
		})
		elapsed := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		if res.Outcome == OutcomeAbsent {
			t.Fatalf("\"The Rite of Spring\"/Stravinsky/Gergiev must resolve in one call; outcome=absent (%s)", elapsed)
		}
		if len(res.Performances) == 0 {
			t.Fatalf("expected at least one performance; outcome=%s (%s)", res.Outcome, elapsed)
		}
		p := res.Performances[0]
		lower := strings.ToLower(p.Work.Name)
		if !strings.Contains(lower, "sacre") && !strings.Contains(lower, "rite") {
			t.Errorf("resolved work name = %q, want it to contain %q or %q", p.Work.Name, "Sacre", "Rite")
		}
		t.Logf("Rite of Spring/Gergiev: outcome=%s perfs=%d work=%q work_resolution=%q warnings=%v (%s)",
			res.Outcome, len(res.Performances), p.Work.Name, p.Work.WorkResolution, res.Warnings, elapsed)
	})

	// (d) Same-composer sibling non-substitution: "St Matthew Passion" (English,
	// no dot — the CURATE.md documented friction case) + Bach + Harnoncourt must
	// never SILENTLY land on the Johannes-Passion sibling. Landing on Johannes
	// while flagged performer-fallback (or with the matching warning) is
	// acceptable — the mandatory cross-check in CURATE.md exists for exactly
	// that case; a silent, unflagged Johannes result is the failure.
	t.Run("st_matthew_passion_never_silently_johannes", func(t *testing.T) {
		start := time.Now()
		res, err := m.ResolvePerformance(PerformanceQuery{
			Work:   "St Matthew Passion",
			MBOnly: true,
			Credits: core.Credits{
				{Role: core.RoleComposer, Name: "Johann Sebastian Bach"},
				{Role: core.RoleConductor, Name: "Nikolaus Harnoncourt"},
			},
			Limit: 25,
		})
		elapsed := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		if res.Outcome == OutcomeAbsent {
			t.Fatalf("St Matthew Passion/Bach/Harnoncourt must resolve in one call; outcome=absent (%s)", elapsed)
		}
		if len(res.Performances) == 0 {
			t.Fatalf("expected at least one performance; outcome=%s (%s)", res.Outcome, elapsed)
		}
		p := res.Performances[0]
		isJohannes := strings.Contains(p.Work.Name, "Johannes")
		flaggedFallback := p.Work.WorkResolution == "performer-fallback"
		for _, w := range res.Warnings {
			if strings.Contains(w, "performer-discography fallback") {
				flaggedFallback = true
			}
		}
		if isJohannes && !flaggedFallback {
			t.Fatalf("SILENT sibling substitution: resolved work %q (Johannes-Passion) with no performer-fallback flag; work_resolution=%q warnings=%v",
				p.Work.Name, p.Work.WorkResolution, res.Warnings)
		}
		t.Logf("St Matthew Passion/Harnoncourt: outcome=%s perfs=%d work=%q work_resolution=%q johannes=%v flaggedFallback=%v warnings=%v (%s)",
			res.Outcome, len(res.Performances), p.Work.Name, p.Work.WorkResolution, isJohannes, flaggedFallback, res.Warnings, elapsed)
	})

	// (e) Regression: the pre-existing Kleiber/VPO federated gate (composer +
	// conductor + orchestra, cross-source High, <60s) must still pass under
	// this task's changes — run it as a subtest here rather than duplicate its
	// assertions.
	t.Run("kleiber_vpo_federated_regression", TestIntegrationResolvePerformanceDiscogsInteractive)
}
