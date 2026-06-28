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
	if (Recording{}).Kind() != KindRecording {
		t.Errorf("Recording.Kind() = %q, want %q", Recording{}.Kind(), KindRecording)
	}
}

func TestGoldenItemDispatch(t *testing.T) {
	items := []GoldenItem{Album{}, Recording{}}
	if items[0].Kind() != KindAlbum || items[1].Kind() != KindRecording {
		t.Error("GoldenItem slice did not dispatch by concrete type")
	}
}

func TestRecordingKindValueIsRecording(t *testing.T) {
	if got := string((Recording{}).Kind()); got != "recording" {
		t.Errorf("Recording.Kind() value = %q, want %q", got, "recording")
	}
}
