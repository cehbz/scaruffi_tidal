package catalog

import (
	"encoding/json"
	"testing"
)

func TestTitleDistance(t *testing.T) {
	if d := titleDistance("Dear Mr. Fantasy", "Dear Mr. Fantasy"); d != 0 {
		t.Errorf("identical titles distance = %v, want 0", d)
	}
	if d := titleDistance("Dear Mr Fantasy", "Heaven Is in Your Mind"); d != 1 {
		t.Errorf("disjoint titles distance = %v, want 1", d)
	}
	// "mr fantasy" vs "dear mr fantasy": inter {mr,fantasy}=2, union {dear,mr,fantasy}=3
	if d := titleDistance("Mr Fantasy", "Dear Mr Fantasy"); d <= 0 || d >= 1 {
		t.Errorf("partial-overlap distance = %v, want strictly between 0 and 1", d)
	}
}

func TestMatchOmitsUnsetSignals(t *testing.T) {
	b, _ := json.Marshal(Match{FTSRank: floatPtr(0.5), ArtistConfirmed: boolPtr(true)})
	got := string(b)
	want := `{"fts_rank":0.5,"artist_confirmed":true}`
	if got != want {
		t.Errorf("Match JSON = %s, want %s", got, want)
	}
}
