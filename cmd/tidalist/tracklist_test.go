package main

import (
	"encoding/json"
	"testing"
)

func TestTracklistCommandByRG(t *testing.T) {
	mb, dc := writeFixtureDBs(t)
	out, err := runCmd(t, "tracklist", "--rg", "rg-jbmd",
		"--musicbrainz-db", mb, "--discogs-db", dc)
	if err != nil {
		t.Fatalf("execute: %v (out=%s)", err, out)
	}
	var got struct {
		Tracks []struct {
			Position int    `json:"position"`
			Title    string `json:"title"`
		} `json:"tracks"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not tracks JSON: %v\n%s", err, out)
	}
	if len(got.Tracks) != 2 || got.Tracks[0].Title != "Glad" {
		t.Fatalf("tracks = %+v", got.Tracks)
	}
}

func TestTracklistCommandRequiresOneSelector(t *testing.T) {
	mb, dc := writeFixtureDBs(t)
	_, err := runCmd(t, "tracklist", "--musicbrainz-db", mb, "--discogs-db", dc)
	if err == nil {
		t.Error("tracklist with neither --rg nor --master must error")
	}
}
