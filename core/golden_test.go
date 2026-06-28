package core

import "testing"

// Compile-time proof both grains are first-class golden units.
var (
	_ GoldenItem = Album{}
	_ GoldenItem = Recording{}
)

func TestKindDiscriminates(t *testing.T) {
	if (Album{}).Kind() != KindAlbum {
		t.Errorf("Album.Kind() = %q, want %q", Album{}.Kind(), KindAlbum)
	}
	if (Recording{}).Kind() != KindTrack {
		t.Errorf("Recording.Kind() = %q, want %q", Recording{}.Kind(), KindTrack)
	}
}

func TestGoldenItemDispatch(t *testing.T) {
	items := []GoldenItem{Album{}, Recording{}}
	if items[0].Kind() != KindAlbum || items[1].Kind() != KindTrack {
		t.Error("GoldenItem slice did not dispatch by concrete type")
	}
}
