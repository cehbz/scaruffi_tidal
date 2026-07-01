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

func TestDiscogsPerformancesArtistFirstComposerRequired(t *testing.T) {
	m := newTestMirror(t)
	dps, err := m.discogsPerformances(beethovenPerfQuery())
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]bool{}
	for _, d := range dps {
		got[d.MasterID] = true
	}
	// MASTER A (full) and MASTER B (partial) are Beethoven 5th by Bernstein → found via
	// the bridge despite the formal query not phrase-matching the album title.
	if !got[70000] || !got[70001] {
		t.Fatalf("expected MASTER A(70000)+B(70001) via artist-first bridge; got %v", got)
	}
	// DECOY (70002) is a Mahler 5th by the SAME performers — the composer requirement
	// must exclude it (a bare "Symphony No. 5" is composer-ambiguous).
	if got[70002] {
		t.Error("wrong-composer decoy (Mahler 5th) must be excluded by the composer requirement")
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

func TestAlbumMatchesWorkDisambiguatesNumber(t *testing.T) {
	work := significantWorkTokens("Symphony no. 5 in C minor, op. 67")
	if !albumMatchesWork(work, "Beethoven: Symphony No. 5") {
		t.Error("the 5th album must match the 5th work")
	}
	if albumMatchesWork(work, "Beethoven: Symphony No. 7 in A major") {
		t.Error("a 7th album must NOT match the 5th work (number disambiguates)")
	}
}
