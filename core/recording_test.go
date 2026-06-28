package core

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestRecordingNoPreferenceStaysSmall(t *testing.T) {
	// A common recording carries no rendering preference; the GM must not fatten.
	r := Recording{Title: "Dear Mr. Fantasy", MBID: "rec-1", ISRC: "GBABC1234567", Performance: PerfStudio}
	b, _ := json.Marshal(r)
	if s := string(b); strings.Contains(s, "rendering") {
		t.Errorf("no-preference recording must omit rendering fields; got %s", s)
	}
}

func TestRecordingWithPreferenceRoundTrip(t *testing.T) {
	in := Recording{
		Title: "In the Court of the Crimson King", MBID: "rec-take", ISRC: "GBXYZ0000001",
		Performance: PerfStudio,
		Rendering:   &RenderingPreference{Markers: []string{"steven-wilson"}},
		Renderings: []RenderingVariant{
			{MBID: "rec-take", ISRC: "GBXYZ0000001", Markers: []string{"original"}},
			{MBID: "rec-remix", ISRC: "GBXYZ0000099", Markers: []string{"steven-wilson", "2009"}},
		},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out Recording
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round-trip mismatch:\n in=%+v\n out=%+v\n json=%s", in, out, b)
	}
}
