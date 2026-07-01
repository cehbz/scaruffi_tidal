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

func TestResolveWorkGroupNoComposerFilterResolvesSingleWork(t *testing.T) {
	m := newTestMirror(t)
	g, ok, err := m.resolveWorkGroup("Missa Papae Marcelli", "")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || g.RootID != 300 {
		t.Errorf("resolveWorkGroup = (%+v,%v), want RootID 300, true", g, ok)
	}
}

func TestResolveWorkGroupExpandsMovementsAndDisambiguatesComposer(t *testing.T) {
	m := newTestMirror(t)

	// FTS on the group title lands on a movement/stub; composer pins Beethoven and
	// the group expands to parent + all four movements (arrangement 322 excluded).
	g, ok, err := m.resolveWorkGroup("Symphony no. 5 in C minor, op. 67", "Beethoven")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected the Beethoven work-group to resolve")
	}
	if g.RootMBID != "w-sym5" {
		t.Errorf("root = %q, want w-sym5 (the parent, not a movement/stub)", g.RootMBID)
	}
	want := map[int64]bool{310: true, 311: true, 312: true, 313: true, 314: true}
	if len(g.WorkIDs) != len(want) {
		t.Fatalf("work-group size = %d %v, want 5 (parent + 4 movements)", len(g.WorkIDs), g.WorkIDs)
	}
	for _, id := range g.WorkIDs {
		if !want[id] {
			t.Errorf("unexpected work %d in group (322 arrangement must be excluded)", id)
		}
	}
	if len(g.Composers) == 0 || g.Composers[0] != "Ludwig van Beethoven" {
		t.Errorf("composers = %v, want Beethoven", g.Composers)
	}
}

func TestResolveWorkGroupFromMovementWalksToParent(t *testing.T) {
	m := newTestMirror(t)
	// FTS matches ONLY a movement title (work 313). Root selection must walk to the
	// parent (310) and expand to the full group, never stop at a movement or the
	// arc-less title-twin stub (321).
	g, ok, err := m.resolveWorkGroup("Symphony no. 5 in C minor, op. 67: III. Scherzo", "Beethoven")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("a movement-only FTS match must still resolve the parent work-group")
	}
	if g.RootMBID != "w-sym5" {
		t.Errorf("root = %q, want w-sym5 (walked from the movement to the parent)", g.RootMBID)
	}
	if len(g.WorkIDs) != 5 {
		t.Errorf("group size = %d %v, want 5 (parent + 4 movements)", len(g.WorkIDs), g.WorkIDs)
	}
}

func TestResolveWorkGroupComposerMismatchIsAbsent(t *testing.T) {
	m := newTestMirror(t)
	// The bare title "Symphony no. 5 in C minor" is ambiguous across composers; a
	// composer that matches no candidate work must NOT resolve (no silent top-1).
	_, ok, err := m.resolveWorkGroup("Symphony no. 5 in C minor", "Sibelius")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("a composer with no matching work must not resolve a group")
	}
}
