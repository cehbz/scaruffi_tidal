package core

import (
	"encoding/json"
	"testing"
)

func TestExternalIDsEmpty(t *testing.T) {
	if !(ExternalIDs{}).Empty() {
		t.Error("zero ExternalIDs should be Empty")
	}
	if (ExternalIDs{MBID: "abc"}).Empty() {
		t.Error("ExternalIDs with an MBID is not Empty")
	}
	if (ExternalIDs{DiscogsMasterID: 7}).Empty() {
		t.Error("ExternalIDs with a Discogs master id is not Empty (Discogs-only is a full identity)")
	}
}

func TestExternalIDsJSONRoundTrip(t *testing.T) {
	in := ExternalIDs{MBID: "rg-1", DiscogsMasterID: 42, Sources: []Source{SourceMusicBrainz, SourceDiscogs}}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out ExternalIDs
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.MBID != in.MBID || out.DiscogsMasterID != in.DiscogsMasterID || len(out.Sources) != 2 {
		t.Errorf("round-trip mismatch: %+v -> %s -> %+v", in, b, out)
	}
}

func TestExternalIDsOmitsAbsent(t *testing.T) {
	b, _ := json.Marshal(ExternalIDs{MBID: "x"})
	if got := string(b); got != `{"mbid":"x"}` {
		t.Errorf("absent ids should be omitted; got %s", got)
	}
}
