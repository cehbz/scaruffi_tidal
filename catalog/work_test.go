package catalog

import "testing"

func TestResolveWork(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.ResolveWork("Missa Papae Marcelli", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 work, got %d", len(got))
	}
	if got[0].MBID != "work-mpm" || got[0].Name != "Missa Papae Marcelli" {
		t.Errorf("work = %+v", got[0])
	}
	if len(got[0].Composers) != 1 || got[0].Composers[0] != "Palestrina" {
		t.Errorf("composers = %v, want [Palestrina]", got[0].Composers)
	}
	if got[0].Match.FTSRank == nil {
		t.Error("expected an fts_rank signal")
	}
}

func TestResolveWorkEmptyOnNoMatch(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.ResolveWork("Nonexistent Symphony", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected no works, got %d", len(got))
	}
}

func TestResolveWorkID(t *testing.T) {
	m := newTestMirror(t)
	id, ok, err := m.resolveWorkID("Missa Papae Marcelli")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || id != 300 {
		t.Errorf("resolveWorkID = (%d,%v), want (300,true)", id, ok)
	}
}
