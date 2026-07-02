package catalog

import (
	"testing"

	"github.com/cehbz/tidalist/core"
)

func beethovenGroup(t *testing.T, m *MirrorDB) WorkGroup {
	t.Helper()
	g, ok, err := m.resolveWorkGroup("Symphony no. 5 in C minor, op. 67", "Beethoven")
	if err != nil || !ok {
		t.Fatalf("resolveWorkGroup: ok=%v err=%v", ok, err)
	}
	return g
}

func TestMBPerformancesClusterByCoRelease(t *testing.T) {
	m := newTestMirror(t)
	g := beethovenGroup(t, m)
	q := PerformanceQuery{
		Work: "Symphony no. 5 in C minor, op. 67",
		Credits: core.Credits{
			{Role: core.RoleConductor, Name: "Leonard Bernstein"},
			{Role: core.RoleOrchestra, Name: "New York Philharmonic"},
		},
		Limit: 10,
	}
	perfs, err := m.mbPerformances(g, q)
	if err != nil {
		t.Fatal(err)
	}
	// SAME forces, TWO co-release clusters → two distinct performances, not one merged.
	if len(perfs) != 2 {
		t.Fatalf("want 2 performances (1963, 1985), got %d: %+v", len(perfs), perfs)
	}
	byYear := map[int]Performance{}
	for _, p := range perfs {
		byYear[p.FirstYear] = p
	}
	a, ok := byYear[1963]
	if !ok {
		t.Fatalf("missing the 1963 performance; years=%v", perfsYears(perfs))
	}
	if len(a.Recordings) != 4 {
		t.Errorf("1963 performance should have 4 movement recordings, got %d", len(a.Recordings))
	}
	if _, ok := byYear[1985]; !ok {
		t.Errorf("missing the 1985 performance; years=%v", perfsYears(perfs))
	}
	if a.Confidence != ConfidenceMedium {
		t.Errorf("MB-only performance confidence = %q, want medium", a.Confidence)
	}
}

func TestMBPerformancesCreditANDExcludesWrongForces(t *testing.T) {
	m := newTestMirror(t)
	g := beethovenGroup(t, m)
	// A conductor who did not perform this work-group → zero performances.
	perfs, err := m.mbPerformances(g, PerformanceQuery{
		Credits: core.Credits{{Role: core.RoleConductor, Name: "Herbert von Karajan"}},
		Limit:   10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(perfs) != 0 {
		t.Errorf("unmatched conductor must yield no performances, got %d", len(perfs))
	}
}

func perfsYears(ps []Performance) []int {
	var out []int
	for _, p := range ps {
		out = append(out, p.FirstYear)
	}
	return out
}

func beethovenPerfQuery() PerformanceQuery {
	return PerformanceQuery{
		// FORMAL work title (divergent from the album-style Discogs title). Artist-first
		// discovery must still find the albums via the bridged ids, not the title.
		Work: "Symphony no. 5 in C minor, op. 67",
		Credits: core.Credits{
			{Role: core.RoleComposer, Name: "Beethoven"},
			{Role: core.RoleConductor, Name: "Leonard Bernstein"},
			{Role: core.RoleOrchestra, Name: "New York Philharmonic"},
		},
		Limit: 10,
	}
}

func TestDiscogsPerformancesPerformerDriven(t *testing.T) {
	m := newTestMirror(t)
	q := PerformanceQuery{
		Work: "Symphony No. 5",
		Credits: core.Credits{
			{Role: core.RoleComposer, Name: "Ludwig van Beethoven"},
			{Role: core.RoleConductor, Name: "Leonard Bernstein"},
			{Role: core.RoleOrchestra, Name: "New York Philharmonic"},
		},
	}
	got, err := m.discogsPerformances(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].MasterID != 70000 {
		t.Fatalf("want exactly master 70000 (Beethoven 5, full force), got %+v", got)
	}
	if got[0].Year != 1963 || got[0].Label != "CBS" {
		t.Fatalf("want 1963/CBS from the main release, got %+v", got[0])
	}
}

func TestDiscogsPerformancesMultiComposerTrap(t *testing.T) {
	// Master 70002 satisfies Bernstein ∩ NYPhil and has "Symphony No. 5" tokens
	// (Mahler's) plus a release-level Beethoven filler credit. The work-group's
	// composer is Mahler -> Beethoven must NOT claim it.
	m := newTestMirror(t)
	q := PerformanceQuery{
		Work: "Symphony No. 5",
		Credits: core.Credits{
			{Role: core.RoleComposer, Name: "Ludwig van Beethoven"},
			{Role: core.RoleConductor, Name: "Leonard Bernstein"},
			{Role: core.RoleOrchestra, Name: "New York Philharmonic"},
		},
	}
	got, err := m.discogsPerformances(q)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range got {
		if p.MasterID == 70002 {
			t.Fatal("multi-composer trap: master 70002's Symphony-5 group is Mahler's")
		}
	}
}

func TestDiscogsPerformancesMahlerQueryTakesDecoy(t *testing.T) {
	// The same album IS a valid Mahler 5 candidate for a Mahler query.
	m := newTestMirror(t)
	q := PerformanceQuery{
		Work: "Symphony No. 5",
		Credits: core.Credits{
			{Role: core.RoleComposer, Name: "Gustav Mahler"},
			{Role: core.RoleConductor, Name: "Leonard Bernstein"},
			{Role: core.RoleOrchestra, Name: "New York Philharmonic"},
		},
	}
	got, err := m.discogsPerformances(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].MasterID != 70002 {
		t.Fatalf("want master 70002 for the Mahler query, got %+v", got)
	}
}

func TestDiscogsPerformancesNoComposerNoClaim(t *testing.T) {
	m := newTestMirror(t)
	q := beethovenPerfQuery()
	q.Credits = core.Credits{ // performers but NO composer
		{Role: core.RoleConductor, Name: "Leonard Bernstein"},
		{Role: core.RoleOrchestra, Name: "New York Philharmonic"},
	}
	dps, err := m.discogsPerformances(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(dps) != 0 {
		t.Errorf("no composer → no Discogs claim (never a token-only match); got %d", len(dps))
	}
}

func TestReconcileGradesByConstraintCompleteness(t *testing.T) {
	m := newTestMirror(t)
	g := beethovenGroup(t, m)
	q := beethovenPerfQuery()
	mb, err := m.mbPerformances(g, q)
	if err != nil {
		t.Fatal(err)
	}
	dc, err := m.discogsPerformances(q)
	if err != nil {
		t.Fatal(err)
	}
	perfs, _, err := m.reconcile(mb, dc, q)
	if err != nil {
		t.Fatal(err)
	}
	byYear := map[int]Performance{}
	for _, p := range perfs {
		byYear[p.FirstYear] = p
	}
	// 1963 MB take reconciles with MASTER A (Beethoven+Bernstein+NYPhil = FULL) → High.
	a := byYear[1963]
	if a.Confidence != ConfidenceHigh || a.DiscogsMaster != 70000 {
		t.Errorf("1963 full-constraint match → High/master 70000; got %q/%d", a.Confidence, a.DiscogsMaster)
	}
	// 1985 MB take has no Discogs candidate: master 70001 is Bernstein-only (no NYPhil
	// credit), and the performer-driven intersection requires every performer arm →
	// stays MB-only Medium, no Discogs identity.
	b := byYear[1985]
	if b.Confidence != ConfidenceMedium || b.DiscogsMaster != 0 {
		t.Errorf("1985 unmatched (70001 excluded by the performer intersection) → MB-only Medium, no master; got %q/%d", b.Confidence, b.DiscogsMaster)
	}
}

func TestAlbumMatchesWorkDisambiguatesNumber(t *testing.T) {
	work := significantWorkTokens("Symphony no. 5 in C minor, op. 67")
	if !albumMatchesWork(work, "Beethoven: Symphony No. 5") {
		t.Error("the 5th album must match the 5th work")
	}
	if albumMatchesWork(work, "Beethoven: Symphony No. 7 in A major") {
		t.Error("a 7th album must NOT match the 5th work (number disambiguates)")
	}
}

func TestResolvePerformanceCandidatesWhenAmbiguous(t *testing.T) {
	m := newTestMirror(t)
	q := beethovenPerfQuery() // FORMAL title; no year selector.
	res, err := m.ResolvePerformance(q)
	if err != nil {
		t.Fatal(err)
	}
	// Two takes, no year/label selector → surface candidates, never substitute one.
	if res.Outcome != OutcomeCandidates {
		t.Fatalf("two unseparated takes → candidates, got %q (%d perfs)", res.Outcome, len(res.Performances))
	}
	if len(res.Performances) != 2 {
		t.Fatalf("want both takes surfaced, got %d", len(res.Performances))
	}
	byYear := map[int]Performance{}
	for _, p := range res.Performances {
		byYear[p.FirstYear] = p
	}
	if p, ok := byYear[1963]; !ok || p.Confidence != ConfidenceHigh {
		t.Errorf("1963 take (full-constraint reconciliation) should be high, got %+v", byYear[1963])
	}
	if p, ok := byYear[1985]; !ok || p.Confidence != ConfidenceMedium {
		t.Errorf("1985 take (partial-constraint reconciliation) should be medium, got %+v", byYear[1985])
	}
}

func TestResolvePerformanceCapturesWithYearSelector(t *testing.T) {
	m := newTestMirror(t)
	q := beethovenPerfQuery() // FORMAL title
	q.Year = 1963
	res, err := m.ResolvePerformance(q)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != OutcomeCaptured {
		t.Fatalf("year 1963 selects one take → captured, got %q", res.Outcome)
	}
	if len(res.Performances) != 1 || res.Performances[0].FirstYear != 1963 {
		t.Fatalf("captured perf = %+v", res.Performances)
	}
	if res.Performances[0].Confidence != ConfidenceHigh {
		t.Errorf("the 1963 take reconciles with Discogs → high, got %q", res.Performances[0].Confidence)
	}
}

func TestResolvePerformanceAbsentWorkGroup(t *testing.T) {
	m := newTestMirror(t)
	res, err := m.ResolvePerformance(PerformanceQuery{
		Work:    "Concerto for Nonexistent Instrument",
		Credits: core.Credits{{Role: core.RoleComposer, Name: "Nobody"}},
		Limit:   10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != OutcomeAbsent {
		t.Errorf("unresolvable work-group → absent, got %q", res.Outcome)
	}
	if len(res.Performances) != 0 {
		t.Errorf("absent must carry no performances, got %d", len(res.Performances))
	}
}

func TestResolvePerformanceYearMissWarnsNotSubstitutes(t *testing.T) {
	m := newTestMirror(t)
	q := beethovenPerfQuery() // FORMAL title
	q.Year = 1972             // matches neither 1963 nor 1985 within ±2
	res, err := m.ResolvePerformance(q)
	if err != nil {
		t.Fatal(err)
	}
	// The performance exists; the requested vintage doesn't → surface candidates + warn,
	// never fabricate a match.
	if res.Outcome != OutcomeCandidates {
		t.Errorf("year miss → candidates (never substitute), got %q", res.Outcome)
	}
	if len(res.Warnings) == 0 {
		t.Error("a year selector that matches nothing must warn")
	}
}

func TestResolvePerformanceMBOnlySkipsDiscogs(t *testing.T) {
	m := newTestMirror(t)

	// Without MBOnly the fixture reconciles: at least one performance touches Discogs
	// (a Sources entry or a Discogs-only Low candidate) — proves the flag isn't vacuous.
	full, err := m.ResolvePerformance(beethovenPerfQuery())
	if err != nil {
		t.Fatal(err)
	}
	var touchesDiscogs bool
	for _, p := range full.Performances {
		for _, s := range p.Sources {
			if s == core.SourceDiscogs {
				touchesDiscogs = true
			}
		}
	}
	if !touchesDiscogs {
		t.Fatal("fixture must produce a Discogs-touched performance without MBOnly")
	}

	q := beethovenPerfQuery()
	q.MBOnly = true
	res, err := m.ResolvePerformance(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Performances) == 0 {
		t.Fatal("MBOnly resolution must still surface the MB performances")
	}
	for i, p := range res.Performances {
		for _, s := range p.Sources {
			if s == core.SourceDiscogs {
				t.Errorf("perf[%d]: MBOnly must not touch Discogs; sources=%v", i, p.Sources)
			}
		}
		if p.DiscogsMaster != 0 {
			t.Errorf("perf[%d]: MBOnly must not carry a Discogs master; got %d", i, p.DiscogsMaster)
		}
		if p.Confidence == ConfidenceHigh {
			t.Errorf("perf[%d]: MBOnly cannot grade High (no cross-source agreement)", i)
		}
	}
}

func TestMBPerformancesExportReleaseGroupIdentity(t *testing.T) {
	m := newTestMirror(t)
	g := beethovenGroup(t, m)
	perfs, err := m.mbPerformances(g, PerformanceQuery{
		Credits: core.Credits{
			{Role: core.RoleConductor, Name: "Leonard Bernstein"},
			{Role: core.RoleOrchestra, Name: "New York Philharmonic"},
		},
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	byYear := map[int]Performance{}
	for _, p := range perfs {
		byYear[p.FirstYear] = p
	}
	a, ok := byYear[1963]
	if !ok {
		t.Fatalf("missing the 1963 performance; years=%v", perfsYears(perfs))
	}
	if a.RGMBID != "rg-a" {
		t.Errorf("1963 RGMBID = %q, want rg-a", a.RGMBID)
	}
	if a.RGTitle != "Beethoven: Symphony no. 5" {
		t.Errorf("1963 RGTitle = %q", a.RGTitle)
	}
	if a.RGArtistCredit == "" {
		t.Error("1963 RGArtistCredit must be populated")
	}
	b, ok := byYear[1985]
	if !ok {
		t.Fatalf("missing the 1985 performance")
	}
	if b.RGMBID != "rg-b" {
		t.Errorf("1985 RGMBID = %q, want rg-b", b.RGMBID)
	}
}

// TestResolvePerformanceFallsBackToPerformerWorkGroup exercises the title-twin
// work-family trap (the live Goldberg case): the English-named family matched by
// title FTS holds only other performers' recordings, while the performer's actual
// recordings hang off a German-named twin family the English phrase can never
// match. Resolution must fall back to performer-driven work-group discovery.
func TestResolvePerformanceFallsBackToPerformerWorkGroup(t *testing.T) {
	m := newTestMirror(t)
	res, err := m.ResolvePerformance(PerformanceQuery{
		Work: "Goldberg Variations",
		Credits: core.Credits{
			{Role: core.RoleComposer, Name: "Johann Sebastian Bach"},
			{Role: core.RoleSoloist, Name: "Glenn Gould"},
		},
		MBOnly: true,
		Limit:  10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome == OutcomeAbsent || len(res.Performances) == 0 {
		t.Fatalf("performer fallback must resolve the twin family; outcome=%q perfs=%d warnings=%v",
			res.Outcome, len(res.Performances), res.Warnings)
	}
	p := res.Performances[0]
	if p.Work.MBID != "w-gv-de" {
		t.Errorf("resolved work-group = %q, want the German twin w-gv-de", p.Work.MBID)
	}
	if len(p.Recordings) != 2 {
		t.Errorf("want Gould's 2 variation recordings, got %d", len(p.Recordings))
	}
	if p.FirstYear != 1955 {
		t.Errorf("first year = %d, want 1955", p.FirstYear)
	}
	if !p.Credits.MatchesRole(core.RoleSoloist, "Glenn Gould") {
		t.Errorf("matched credits %v missing the Gould soloist credit", p.Credits)
	}
}

// TestCreditsSatisfyEnsembleUmbrella: an intent-side orchestra/soloist constraint
// must match MB's actual credit typing for the same name — a vocal ensemble is
// credited chorus (or only as the credited artist), a soloist may carry only the
// artist credit. Identity stays name-exact; only the role is umbrella'd.
func TestCreditsSatisfyEnsembleUmbrella(t *testing.T) {
	cases := []struct {
		name string
		have core.Credits
		want core.Credits
		ok   bool
	}{
		{"orchestra matches chorus credit",
			core.Credits{{Role: core.RoleChorus, Name: "The Tallis Scholars"}},
			core.Credits{{Role: core.RoleOrchestra, Name: "The Tallis Scholars"}}, true},
		{"orchestra matches bare artist credit",
			core.Credits{{Role: core.RoleArtist, Name: "The Tallis Scholars"}},
			core.Credits{{Role: core.RoleOrchestra, Name: "The Tallis Scholars"}}, true},
		{"soloist matches bare artist credit",
			core.Credits{{Role: core.RoleArtist, Name: "Glenn Gould"}},
			core.Credits{{Role: core.RoleSoloist, Name: "Glenn Gould"}}, true},
		{"chorus matches bare artist credit",
			core.Credits{{Role: core.RoleArtist, Name: "Monteverdi Choir"}},
			core.Credits{{Role: core.RoleChorus, Name: "Monteverdi Choir"}}, true},
		{"orchestra does NOT match a different name",
			core.Credits{{Role: core.RoleChorus, Name: "Some Other Choir"}},
			core.Credits{{Role: core.RoleOrchestra, Name: "The Tallis Scholars"}}, false},
		{"conductor still umbrellas chorus_master",
			core.Credits{{Role: core.RoleChorusMaster, Name: "Peter Phillips"}},
			core.Credits{{Role: core.RoleConductor, Name: "Peter Phillips"}}, true},
		{"conductor does NOT match bare artist credit",
			core.Credits{{Role: core.RoleArtist, Name: "Herbert von Karajan"}},
			core.Credits{{Role: core.RoleConductor, Name: "Herbert von Karajan"}}, false},
	}
	for _, c := range cases {
		if got := creditsSatisfy(c.have, wantsOf(c.want)); got != c.ok {
			t.Errorf("%s: creditsSatisfy = %v, want %v", c.name, got, c.ok)
		}
	}
}

// TestResolvePerformanceAliasNamedArtists: MB stores Russian artists under
// Cyrillic primary names with Latin forms in artist_alias. A query using the
// Latin names (composer, conductor, orchestra) must still resolve: artist
// resolution falls back to the alias table, and credit matching accepts any
// alias of the requested name.
func TestResolvePerformanceAliasNamedArtists(t *testing.T) {
	m := newTestMirror(t)
	res, err := m.ResolvePerformance(PerformanceQuery{
		Work: "Le Sacre du printemps",
		Credits: core.Credits{
			{Role: core.RoleComposer, Name: "Igor Stravinsky"},
			{Role: core.RoleConductor, Name: "Valery Gergiev"},
			{Role: core.RoleOrchestra, Name: "Kirov Orchestra"},
		},
		MBOnly: true,
		Limit:  10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome == OutcomeAbsent || len(res.Performances) == 0 {
		t.Fatalf("alias-named query must resolve; outcome=%q warnings=%v", res.Outcome, res.Warnings)
	}
	p := res.Performances[0]
	if p.RGMBID != "rg-sacre-gergiev" {
		t.Errorf("rg = %q, want rg-sacre-gergiev", p.RGMBID)
	}
	if p.FirstYear != 1999 {
		t.Errorf("first year = %d, want 1999", p.FirstYear)
	}
}

func TestWorkGroupTracks(t *testing.T) {
	work := significantWorkTokens("Symphony No. 5")
	tracks := []dcTrack{
		{ID: 1, Title: "Symphony No. 5: I. Allegro con brio"},
		{ID: 2, Title: "Symphony No. 5: II. Andante con moto"},
		{ID: 3, Title: "Fidelio Overture"},
	}
	got := workGroupTracks(work, "Beethoven / Mahler: Orchestral Works", tracks)
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 2 {
		t.Fatalf("want tracks 1,2 (the Symphony 5 group), got %+v", got)
	}
}

func TestWorkGroupTracksReleaseTitleFallback(t *testing.T) {
	// Movement-titled tracks share no token with the work; the release title carries it.
	work := significantWorkTokens("Symphony No. 5")
	tracks := []dcTrack{
		{ID: 1, Title: "I. Allegro con brio"},
		{ID: 2, Title: "II. Andante con moto"},
	}
	got := workGroupTracks(work, "Beethoven: Symphony No. 5", tracks)
	if len(got) != 2 {
		t.Fatalf("release-title fallback: want all 2 tracks, got %+v", got)
	}
}

func TestWorkGroupTracksNoMatch(t *testing.T) {
	work := significantWorkTokens("Symphony No. 5")
	tracks := []dcTrack{{ID: 1, Title: "Ein Heldenleben"}}
	if got := workGroupTracks(work, "Strauss: Ein Heldenleben", tracks); got != nil {
		t.Fatalf("want nil (album is not this work), got %+v", got)
	}
}

func TestGroupComposerConfirmedTrackLevel(t *testing.T) {
	group := []dcTrack{{ID: 1, Title: "Symphony No. 5: I."}}
	trackCredits := map[int64][]dcCredit{1: {{ArtistID: 953, Role: "Composed By"}}}
	if !groupComposerConfirmed(group, trackCredits, nil, 953) {
		t.Fatal("track-level composer credit on the group must confirm")
	}
}

func TestGroupComposerConfirmedTrapRejected(t *testing.T) {
	// The matched group is Mahler's (track-credited); a Beethoven release-level
	// filler credit must NOT confirm Beethoven for this group.
	group := []dcTrack{{ID: 1, Title: "Symphony No. 5: I."}}
	trackCredits := map[int64][]dcCredit{1: {{ArtistID: 953, Role: "Composed By"}}}
	releaseCredits := []dcCredit{{ArtistID: 952, Role: "Composed By"}}
	if groupComposerConfirmed(group, trackCredits, releaseCredits, 952) {
		t.Fatal("multi-composer trap: group composer is Mahler; Beethoven must be rejected")
	}
}

func TestGroupComposerConfirmedReleaseLevelFallback(t *testing.T) {
	// No track-level composer credits anywhere in the group -> release-level confirms.
	group := []dcTrack{{ID: 1, Title: "Symphony No. 5: I."}}
	trackCredits := map[int64][]dcCredit{1: {{ArtistID: 299702, Role: "Conductor"}}}
	releaseCredits := []dcCredit{{ArtistID: 952, Role: "Composed By"}}
	if !groupComposerConfirmed(group, trackCredits, releaseCredits, 952) {
		t.Fatal("release-level composer credit must confirm when the group has no track-level composer")
	}
}

func TestGroupComposerConfirmedCombinedRole(t *testing.T) {
	group := []dcTrack{{ID: 1, Title: "Symphony No. 5: I."}}
	trackCredits := map[int64][]dcCredit{1: {{ArtistID: 952, Role: "Composed By, Conductor"}}}
	if !groupComposerConfirmed(group, trackCredits, nil, 952) {
		t.Fatal("combined role string containing Composed By must confirm")
	}
}

func TestPerformerIntersectionCandidates(t *testing.T) {
	m := newTestMirror(t)
	// Bernstein(299702) ∩ NYPhil(950): releases 60000 (Beethoven 5) and 60002 (Mahler
	// decoy) are credited with both; 60001 is Bernstein-only and must be absent.
	got, err := m.performerIntersectionCandidates([]int64{299702, 950})
	if err != nil {
		t.Fatal(err)
	}
	byMaster := map[int64]dcCandidate{}
	for _, c := range got {
		byMaster[c.MasterID] = c
	}
	if _, ok := byMaster[70001]; ok {
		t.Fatal("master 70001 is Bernstein-only; the intersection must exclude it")
	}
	if c, ok := byMaster[70000]; !ok || c.ReleaseID != 60000 || c.Year != 1963 {
		t.Fatalf("master 70000: want credited release 60000 year 1963, got %+v", c)
	}
	if _, ok := byMaster[70002]; !ok {
		t.Fatal("master 70002 (Mahler decoy) satisfies the performer intersection; work-group confirmation rejects it later, not here")
	}
}

func TestTracksForBatch(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.tracksFor([]int64{583800, 382820})
	if err != nil {
		t.Fatal(err)
	}
	if len(got[583800]) != 2 || got[583800][0].Title != "Glad" {
		t.Fatalf("release 583800: want [Glad, Freedom Rider], got %+v", got[583800])
	}
	// 382820: the sub-track (parent_track_id=110) is excluded.
	if len(got[382820]) != 1 || got[382820][0].Title != "John Barleycorn Suite" {
		t.Fatalf("release 382820: want the single top-level track, got %+v", got[382820])
	}
}

func TestReleaseArtistsForBatch(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.releaseArtistsFor([]int64{60000, 60001})
	if err != nil {
		t.Fatal(err)
	}
	if len(got[60000]) != 3 { // Composed By + Conductor + Orchestra
		t.Fatalf("release 60000: want 3 credits, got %+v", got[60000])
	}
	if len(got[60001]) != 2 {
		t.Fatalf("release 60001: want 2 credits, got %+v", got[60001])
	}
}

func TestTrackArtistsForBatchEmpty(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.trackArtistsFor([]int64{583800})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 { // fixture has no track_artist rows for 583800
		t.Fatalf("want empty map, got %+v", got)
	}
}
