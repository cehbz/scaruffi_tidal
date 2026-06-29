package catalog

import "testing"

func TestFindAlbumMBByArtist(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindAlbum(AlbumQuery{Title: "John Barleycorn Must Die", ArtistName: "Traffic", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	var c *AlbumCandidate
	for i := range got {
		if len(got[i].Sources) > 0 && string(got[i].Sources[0]) == "musicbrainz" {
			c = &got[i]
			break
		}
	}
	if c == nil {
		t.Fatalf("expected MB candidate in results (got %d total): %+v", len(got), got)
	}
	if c.MBID != "rg-jbmd" {
		t.Errorf("MBID = %q, want rg-jbmd", c.MBID)
	}
	if c.DiscogsMasterID != 69017 {
		t.Errorf("DiscogsMasterID = %d, want 69017 (the FK)", c.DiscogsMasterID)
	}
	if c.Year != 1970 {
		t.Errorf("Year = %d, want 1970", c.Year)
	}
	if len(c.Sources) != 1 || c.Sources[0] != "musicbrainz" {
		t.Errorf("Sources = %v, want [musicbrainz]", c.Sources)
	}
	if !c.Credits.Has("artist", "Traffic") {
		t.Errorf("expected an artist credit for Traffic; got %v", c.Credits)
	}
	if c.Match.ArtistConfirmed == nil || !*c.Match.ArtistConfirmed {
		t.Error("artist_confirmed should be true")
	}
}

func TestFindAlbumIncludesDiscogsPeer(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindAlbum(AlbumQuery{Title: "John Barleycorn Must Die", ArtistName: "Traffic", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	var mb, dc *AlbumCandidate
	for i := range got {
		switch got[i].Sources[0] {
		case "musicbrainz":
			mb = &got[i]
		case "discogs":
			dc = &got[i]
		}
	}
	if mb == nil || dc == nil {
		t.Fatalf("expected one MB and one Discogs candidate (peers), got %d: %+v", len(got), got)
	}
	if dc.DiscogsMasterID != 69017 {
		t.Errorf("Discogs master id = %d, want 69017", dc.DiscogsMasterID)
	}
	if dc.Year != 1970 {
		t.Errorf("Discogs year = %d, want 1970", dc.Year)
	}
	if dc.TrackCount != 2 {
		t.Errorf("Discogs track_count = %d, want 2 (main release)", dc.TrackCount)
	}
	if len(dc.Styles) == 0 {
		t.Errorf("expected Discogs styles (genres ∪ styles), got none")
	}
	if dc.MBID != "" {
		t.Errorf("Discogs candidate must not carry an MBID (single-source); got %q", dc.MBID)
	}
}

func TestFindAlbumMBTraits(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindAlbum(AlbumQuery{Title: "Best of Traffic", ArtistName: "Traffic", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(got))
	}
	found := false
	for _, tr := range got[0].Traits {
		if tr == "compilation" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected compilation trait, got %v", got[0].Traits)
	}
}

func TestFindAlbumUnresolvedArtistEmpty(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindAlbum(AlbumQuery{Title: "John Barleycorn Must Die", ArtistName: "Nonexistent", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("a requested-but-unresolved artist must yield no MB candidates; got %d", len(got))
	}
}
