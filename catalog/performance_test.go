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

func TestDiscogsPerformancesViaBridge(t *testing.T) {
	m := newTestMirror(t)
	q := PerformanceQuery{
		Work: "Beethoven: Symphony No. 5",
		Credits: core.Credits{
			{Role: core.RoleConductor, Name: "Leonard Bernstein"},
			{Role: core.RoleOrchestra, Name: "New York Philharmonic"},
		},
		Limit: 10,
	}
	dps, err := m.discogsPerformances(q)
	if err != nil {
		t.Fatal(err)
	}
	// Only MASTER A (70000) carries BOTH bridged forces (Bernstein 299702 + NYPhil 950).
	var found *dcPerf
	for i := range dps {
		if dps[i].MasterID == 70000 {
			found = &dps[i]
		}
	}
	if found == nil {
		t.Fatalf("MASTER A (Bernstein+NYPhil) must resolve via the bridge; got %+v", dps)
	}
	if found.Year != 1963 || found.Label != "CBS" || found.Catno != "MS 6468" {
		t.Errorf("master A attrs = year %d label %q catno %q", found.Year, found.Label, found.Catno)
	}
}

func TestReconcileCrossSourceAgreementIsHigh(t *testing.T) {
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
	mb, err := m.mbPerformances(g, q)
	if err != nil {
		t.Fatal(err)
	}
	// Discogs query uses the album-level title; both takes share it, but only MASTER A
	// carries NYPhil, so only the 1963 MB performance reconciles.
	dq := q
	dq.Work = "Beethoven: Symphony No. 5"
	dc, err := m.discogsPerformances(dq)
	if err != nil {
		t.Fatal(err)
	}
	perfs, warns, err := m.reconcile(mb, dc, q)
	if err != nil {
		t.Fatal(err)
	}
	_ = warns
	var a1963 *Performance
	for i := range perfs {
		if perfs[i].FirstYear == 1963 {
			a1963 = &perfs[i]
		}
	}
	if a1963 == nil {
		t.Fatal("the 1963 MB performance must survive reconciliation")
	}
	if a1963.Confidence != ConfidenceHigh {
		t.Errorf("cross-source agreement must be high, got %q", a1963.Confidence)
	}
	if a1963.DiscogsMaster != 70000 || a1963.Label != "CBS" || a1963.Catno != "MS 6468" {
		t.Errorf("1963 dual identity = master %d label %q catno %q", a1963.DiscogsMaster, a1963.Label, a1963.Catno)
	}
	// The 1985 MB performance (Bernstein+NYPhil label in MB, but Discogs MASTER B is
	// Wiener) has no Discogs agreement → stays medium, no master.
	for i := range perfs {
		if perfs[i].FirstYear == 1985 {
			if perfs[i].Confidence != ConfidenceMedium || perfs[i].DiscogsMaster != 0 {
				t.Errorf("1985 must stay MB-only/medium; got conf %q master %d", perfs[i].Confidence, perfs[i].DiscogsMaster)
			}
		}
	}
}
