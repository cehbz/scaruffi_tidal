package catalog

import "testing"

func TestDiscogsArtistIDBridge(t *testing.T) {
	m := newTestMirror(t)
	// MB Bernstein (artist 60) → discogs_artist_id 299702, which resolves in dc.
	id, ok, err := m.discogsArtistID(60)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || id != 299702 {
		t.Fatalf("bridge(60) = %d,%v, want 299702,true", id, ok)
	}
	// A MB artist with a NULL bridge → not ok.
	if _, ok, _ := m.discogsArtistID(50); ok {
		t.Error("artist 50 (Beethoven) has no discogs_artist_id in the fixture; want ok=false")
	}
}

func TestDCArtistIDByNameFallback(t *testing.T) {
	m := newTestMirror(t)
	id, ok, err := m.dcArtistIDByName("Leonard Bernstein")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || id != 299702 {
		t.Fatalf("dcArtistIDByName = %d,%v, want 299702,true", id, ok)
	}
}
