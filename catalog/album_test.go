package catalog

import "testing"

func TestFindAlbumMBByArtist(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindAlbum(AlbumQuery{Title: "John Barleycorn Must Die", ArtistName: "Traffic", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 MB candidate, got %d", len(got))
	}
	c := got[0]
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
