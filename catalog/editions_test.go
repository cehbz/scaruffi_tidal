package catalog

import "testing"

func TestParseYear(t *testing.T) {
	cases := map[string]int{"1970-07-01": 1970, "1987": 1987, "1970-06-00": 1970, "": 0, "n/a": 0}
	for in, want := range cases {
		if got := parseYear(in); got != want {
			t.Errorf("parseYear(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestAlbumEditionsMB(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.AlbumEditionsMB("rg-jbmd")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 MB editions, got %d", len(got))
	}
	// ordered by year then gid; both 1970, so by gid: rel-jbmd before rel-jbmd-us
	e := got[0]
	if e.MBID != "rel-jbmd" || e.Year != 1970 || e.Country != "United Kingdom" {
		t.Errorf("edition 0 = %+v", e)
	}
	if len(e.Formats) == 0 || e.Formats[0] != "Vinyl" {
		t.Errorf("formats = %v, want [Vinyl]", e.Formats)
	}
	if len(e.Labels) == 0 || e.Labels[0] != "Island Records" {
		t.Errorf("labels = %v", e.Labels)
	}
	if e.TrackCount != 2 {
		t.Errorf("track_count = %d, want 2", e.TrackCount)
	}
	if e.Source != "musicbrainz" {
		t.Errorf("source = %q", e.Source)
	}
}

func TestAlbumEditionsDiscogs(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.AlbumEditionsDiscogs(69017)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 Discogs editions, got %d", len(got))
	}
	// ordered is_main_release DESC, so the main release (583800) first
	e := got[0]
	if e.DiscogsReleaseID != 583800 || !e.IsMainRelease {
		t.Errorf("edition 0 = %+v, want main release 583800", e)
	}
	if e.Year != 1970 || e.Country != "UK" {
		t.Errorf("year/country = %d/%q", e.Year, e.Country)
	}
	if e.Source != "discogs" {
		t.Errorf("source = %q", e.Source)
	}
}
