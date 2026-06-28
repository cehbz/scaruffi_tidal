package core

import (
	"encoding/json"
	"reflect"
	"testing"
)

func sampleAlbum() Album {
	return Album{
		Credits:       Credits{{Role: RoleComposer, Name: "Palestrina"}, {Role: RoleOrchestra, Name: "The Tallis Scholars"}},
		Title:         "Missa Papae Marcelli",
		IDs:           ExternalIDs{MBID: "rg-9", DiscogsMasterID: 100, Sources: []Source{SourceMusicBrainz, SourceDiscogs}},
		FirstReleased: 1980,
		Traits:        []ReleaseTrait{TraitLive},
		Styles:        []string{"Religious", "Renaissance"},
		Tracklist:     []TrackRef{{Position: 1, Title: "Kyrie", DurationS: 200}},
	}
}

func TestAlbumHasTrait(t *testing.T) {
	a := sampleAlbum()
	if !a.HasTrait(TraitLive) {
		t.Error("expected live trait")
	}
	if a.HasTrait(TraitCompilation) {
		t.Error("did not expect compilation trait")
	}
}

func TestAlbumJSONRoundTrip(t *testing.T) {
	in := sampleAlbum()
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out Album
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round-trip mismatch:\n in=%+v\n out=%+v\n json=%s", in, out, b)
	}
}

func TestAlbumDiscogsOnlyIsFirstClass(t *testing.T) {
	a := Album{Title: "x", IDs: ExternalIDs{DiscogsMasterID: 5, Sources: []Source{SourceDiscogs}}}
	if a.IDs.Empty() {
		t.Error("a Discogs-only album is a full identity, not empty")
	}
}
