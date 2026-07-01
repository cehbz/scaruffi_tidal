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
