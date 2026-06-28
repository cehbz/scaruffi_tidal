package catalog

import "testing"

func TestResolveArtistRanksByFTS(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.ResolveArtist("Traffic", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one artist candidate")
	}
	if got[0].Name != "Traffic" {
		t.Errorf("top candidate name = %q, want Traffic (exact match outranks Traffic Sound)", got[0].Name)
	}
	if got[0].MBID == "" {
		t.Error("candidate must carry an MBID (gid)")
	}
	if got[0].Match.FTSRank == nil {
		t.Error("candidate must carry an fts_rank signal")
	}
}

func TestResolveArtistEmptyOnNoMatch(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.ResolveArtist("Nonexistent Ensemble", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected no candidates, got %d", len(got))
	}
}

func TestResolveArtistID(t *testing.T) {
	m := newTestMirror(t)
	id, ok, err := m.resolveArtistID("Traffic")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || id != 1 {
		t.Errorf("resolveArtistID(Traffic) = (%d,%v), want (1,true) — exact-match FTS top hit", id, ok)
	}
}
