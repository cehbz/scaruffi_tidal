package catalog

import "testing"

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
