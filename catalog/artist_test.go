package catalog

import (
	"testing"

	"github.com/cehbz/tidalist/core"
)

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

// TestResolveArtistIDAliasBeatsFTSDecoy: Berliner Philharmoniker (id 68) carries
// the artist_alias "Berlin Philharmonic"; a decoy "Berlin Philharmonic Wind
// Quintet" (id 69) is the FTS top hit for the literal query text (its name
// contains the phrase; the orchestra's German primary name shares no tokens with
// the query at all, so FTS never even sees it). Today's resolveArtistID only
// consults artist_alias on an FTS MISS, so the quintet wins — RED. Alias-exact
// must outrank any FTS rank.
func TestResolveArtistIDAliasBeatsFTSDecoy(t *testing.T) {
	m := newTestMirror(t)
	id, ok, err := m.resolveArtistID("Berlin Philharmonic")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || id != 68 {
		t.Errorf("resolveArtistID(Berlin Philharmonic) = (%d,%v), want (68,true) — exact alias (Berliner Philharmoniker) beats the Wind Quintet's FTS rank", id, ok)
	}
}

// TestResolveArtistIDForRoleOrchestraPrefersOrchestraType: "Leningrad
// Philharmonic Trio" (id 70, type 2) and "Leningrad Philharmonic Orchestra" (id
// 71, type 5) tie on FTS bm25 (same query-term frequency, same document length);
// the trio's lower rowid wins the tie-break, so plain FTS-rank resolution returns
// the trio. For an orchestra credit role, type preference must promote the actual
// orchestra past the FTS-favored trio.
func TestResolveArtistIDForRoleOrchestraPrefersOrchestraType(t *testing.T) {
	m := newTestMirror(t)
	id, ok, err := m.resolveArtistIDForRole("Leningrad Philharmonic", core.RoleOrchestra)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || id != 71 {
		t.Errorf("resolveArtistIDForRole(Leningrad Philharmonic, orchestra) = (%d,%v), want (71,true) — the orchestra, not the FTS-favored trio", id, ok)
	}
}

// TestResolveArtistIDForRoleSoloistKeepsFTSTop: same candidate pair as above, but
// for a soloist credit role — the soloist type preference ({person, group}) gives
// the orchestra (type 5) no advantage, so the FTS-top candidate (the trio) must
// still win. Type preference is role-specific, not a blanket re-rank.
func TestResolveArtistIDForRoleSoloistKeepsFTSTop(t *testing.T) {
	m := newTestMirror(t)
	id, ok, err := m.resolveArtistIDForRole("Leningrad Philharmonic", core.RoleSoloist)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || id != 70 {
		t.Errorf("resolveArtistIDForRole(Leningrad Philharmonic, soloist) = (%d,%v), want (70,true) — soloist type preference gives the orchestra no advantage; the FTS-top trio still wins", id, ok)
	}
}
