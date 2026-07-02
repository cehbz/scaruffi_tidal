package catalog

import (
	"testing"

	"github.com/cehbz/tidalist/core"
)

func TestRecordingCreditsRoleMapping(t *testing.T) {
	m := newTestMirror(t)
	cs, err := m.recordingCredits(30)
	if err != nil {
		t.Fatal(err)
	}
	role := map[string]string{}
	for _, c := range cs {
		role[string(c.Role)] = c.Name
	}
	if role["conductor"] != "Peter Phillips" {
		t.Errorf("conductor = %q", role["conductor"])
	}
	if role["orchestra"] != "The Tallis Scholars" {
		t.Errorf("orchestra = %q", role["orchestra"])
	}
	if role["chorus"] != "Oxford Choir" {
		t.Errorf("chorus = %q (vocal + choir-vocals attr must map to chorus)", role["chorus"])
	}
	if role["chorus_master"] != "Some Chorusmaster" {
		t.Errorf("chorus_master = %q (152 must surface as its own role, alongside the conductor)", role["chorus_master"])
	}
	foundSoloist := false
	for _, c := range cs {
		if c.Role == "soloist" && c.Name == "Emma Kirkby" {
			foundSoloist = true
			if c.Attrs["instrument"] != "piano" {
				t.Errorf("soloist instrument = %q, want piano", c.Attrs["instrument"])
			}
		}
	}
	if !foundSoloist {
		t.Error("expected soloist Emma Kirkby (instrument)")
	}
}

func TestRecordingCreditsStandaloneChorusMaster(t *testing.T) {
	m := newTestMirror(t)
	cs, err := m.recordingCredits(32)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range cs {
		if c.Role == "chorus_master" && c.Name == "Barnaby Smith" {
			found = true
		}
	}
	if !found {
		t.Errorf("standalone chorus-master must surface as a chorus_master credit; got %+v", cs)
	}
}

// TestReleaseGroupCreditsClassical: rg-a's canonical tracklist (recordings 40-43)
// each carry a conductor(Bernstein)/orchestra(NYPhil) arc; the RG's own
// artist_credit_name (artist 1, "Traffic" — shared fixture placeholder credit) is
// the album's RoleArtist credit. Aggregating over all four movements must dedupe
// the repeated conductor/orchestra arcs down to one credit each.
func TestReleaseGroupCreditsClassical(t *testing.T) {
	m := newTestMirror(t)
	cs, err := m.ReleaseGroupCredits(core.MBID("rg-a"))
	if err != nil {
		t.Fatal(err)
	}
	want := core.Credits{
		{Role: core.RoleArtist, Name: "Traffic"},
		{Role: core.RoleConductor, Name: "Leonard Bernstein"},
		{Role: core.RoleOrchestra, Name: "New York Philharmonic"},
	}
	if len(cs) != len(want) {
		t.Fatalf("ReleaseGroupCredits(rg-a) = %+v, want %+v (deduped across 4 movements)", cs, want)
	}
	for i := range want {
		if cs[i].Role != want[i].Role || cs[i].Name != want[i].Name {
			t.Errorf("credit[%d] = %+v, want %+v", i, cs[i], want[i])
		}
	}
}

// TestReleaseGroupCreditsArtistOnly: rg-jbmd's recordings (Glad, Freedom Rider)
// have no performer arcs, so only the RG's own artist credit is emitted — and the
// per-recording RoleArtist rows recordingCredits also returns must NOT leak in as
// duplicate/extra credits (only performer-arc roles are aggregated from part b).
func TestReleaseGroupCreditsArtistOnly(t *testing.T) {
	m := newTestMirror(t)
	cs, err := m.ReleaseGroupCredits(core.MBID("rg-jbmd"))
	if err != nil {
		t.Fatal(err)
	}
	want := core.Credits{{Role: core.RoleArtist, Name: "Traffic"}}
	if len(cs) != len(want) || cs[0].Role != want[0].Role || cs[0].Name != want[0].Name {
		t.Errorf("ReleaseGroupCredits(rg-jbmd) = %+v, want %+v", cs, want)
	}
}

// TestReleaseGroupCreditsUnknownRG: an unresolvable gid yields an empty (not nil-
// error) result, consistent with albumArtistCredits/TracklistByReleaseGroup's
// empty-result-set-not-error behavior for unknown gids.
func TestReleaseGroupCreditsUnknownRG(t *testing.T) {
	m := newTestMirror(t)
	cs, err := m.ReleaseGroupCredits(core.MBID("rg-does-not-exist"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 0 {
		t.Errorf("ReleaseGroupCredits(unknown) = %+v, want empty", cs)
	}
}
