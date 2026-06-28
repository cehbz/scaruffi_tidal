package core

import (
	"encoding/json"
	"testing"
)

func TestTrackRefJSONKeysAreSnakeCase(t *testing.T) {
	b, _ := json.Marshal(TrackRef{Position: 1, Title: "Dear Mr. Fantasy", DurationS: 350})
	got := string(b)
	want := `{"position":1,"title":"Dear Mr. Fantasy","duration_s":350}`
	if got != want {
		t.Errorf("TrackRef JSON = %s, want %s", got, want)
	}
}

func TestTrackRefOmitsUnobservedDuration(t *testing.T) {
	b, _ := json.Marshal(TrackRef{Position: 2, Title: "x"})
	if got := string(b); got != `{"position":2,"title":"x"}` {
		t.Errorf("unobserved duration should be omitted; got %s", got)
	}
}
