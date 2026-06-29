package catalog

import "testing"

func TestTracklistByReleaseGroupCanonical(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.TracklistByReleaseGroup("rg-jbmd")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 tracks on the canonical release, got %d", len(got))
	}
	if got[0].Position != 1 || got[0].Title != "Glad" {
		t.Errorf("track 1 = %+v, want pos 1 'Glad'", got[0])
	}
	if got[0].MBID != "r-glad" {
		t.Errorf("track 1 recording MBID = %q, want r-glad", got[0].MBID)
	}
	if got[0].DurationS != 419 {
		t.Errorf("track 1 duration_s = %d, want 419 (track.length ms/1000)", got[0].DurationS)
	}
}

func TestTracklistByMasterDiscogs(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.TracklistByMaster(69017)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 Discogs tracks, got %d", len(got))
	}
	if got[0].Position != 1 || got[0].Title != "Glad" {
		t.Errorf("track 1 = %+v, want pos 1 'Glad'", got[0])
	}
	if got[0].DurationS != 392 {
		t.Errorf("track 1 duration_s = %d, want 392 (6:32 parsed)", got[0].DurationS)
	}
}

func TestParseDuration(t *testing.T) {
	cases := map[string]int{"6:32": 392, "0:45": 45, "1:00:00": 3600, "": 0, "garbage": 0}
	for in, want := range cases {
		if got := parseDuration(in); got != want {
			t.Errorf("parseDuration(%q) = %d, want %d", in, got, want)
		}
	}
}
