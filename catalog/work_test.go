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

func TestResolveWorkGroupSurvivesGenericTitleFlood(t *testing.T) {
	m := newTestMirror(t)
	// The fixture seeds 30 decoy works titled EXACTLY like the real Beethoven
	// Symphony No. 5 (work 310), composed by an unrelated decoy composer, ranked
	// ahead of the real work in title-FTS order (see mirrorfixture's
	// mbGenericTitleFloodStmts). A candidate query that takes the top-25 FTS hits
	// BEFORE filtering by composer never sees the real work at all — the
	// production symptom for generic titles like "Symphony No. 5". The composer
	// filter must be joined into candidate generation itself so a composer id
	// that resolves conditions the FTS query, not just the post-filter.
	g, ok, err := m.resolveWorkGroup("Symphony no. 5 in C minor, op. 67", "Beethoven")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected the Beethoven work-group to resolve despite the 30-decoy title flood")
	}
	if g.RootMBID != "w-sym5" {
		t.Errorf("root = %q, want w-sym5 (the Beethoven parent, not a decoy)", g.RootMBID)
	}
	if len(g.Composers) == 0 || g.Composers[0] != "Ludwig van Beethoven" {
		t.Errorf("composers = %v, want Beethoven", g.Composers)
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

// TestResolveWorkGroupRecoversViaWorkAlias: "The Rite of Spring" is an English
// title; work_fts only holds the French/Cyrillic-adjacent "Le Sacre du
// printemps" forms, so title FTS misses entirely. The root (350) carries no
// English alias — only the movement (351) does — so recovery depends on the
// work_alias candidate union plus the existing 281-parent walk (step c).
func TestResolveWorkGroupRecoversViaWorkAlias(t *testing.T) {
	m := newTestMirror(t)
	g, ok, err := m.resolveWorkGroup("The Rite of Spring", "Igor Stravinsky")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected the Sacre work-group to resolve via work_alias")
	}
	if g.RootID != 350 {
		t.Errorf("root = %d, want 350 (Le Sacre du printemps)", g.RootID)
	}
}

// TestResolveWorkGroupWorkAliasNeverSubstitutesSibling: the punctuation-free
// query "St Matthew Passion" must still recover the Matthäus-Passion family
// via its movement's work_alias row, "St. Matthew Passion, BWV 244: Part I" —
// real mirror aliases carry punctuation queries lack, and the period after
// "St" breaks a bare core.NormalizeName prefix match (see foldWorkTitle in
// catalog/work.go). The query must never resolve the same-composer sibling
// Johannes-Passion family (which carries no alias at all) — the same-
// composer-sibling case arcs alone cannot discriminate.
func TestResolveWorkGroupWorkAliasNeverSubstitutesSibling(t *testing.T) {
	m := newTestMirror(t)
	g, ok, err := m.resolveWorkGroup("St Matthew Passion", "Johann Sebastian Bach")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected the Matthäus work-group to resolve via work_alias")
	}
	if g.RootID != 353 {
		t.Errorf("root = %d, want 353 (Matthäus-Passion, BWV 244), never the Johannes sibling", g.RootID)
	}
}
