package main

import (
	"encoding/json"
	"testing"
)

func TestAlbumEditionsCommandByRG(t *testing.T) {
	mb, dc := writeFixtureDBs(t)
	out, err := runCmd(t, "album-editions", "--rg", "rg-jbmd",
		"--musicbrainz-db", mb, "--discogs-db", dc)
	if err != nil {
		t.Fatalf("execute: %v (out=%s)", err, out)
	}
	var got struct {
		Editions []struct {
			MBID    string `json:"mbid"`
			Country string `json:"country"`
		} `json:"editions"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not editions JSON: %v\n%s", err, out)
	}
	if len(got.Editions) != 2 {
		t.Fatalf("expected 2 editions, got %d", len(got.Editions))
	}
}

func TestAlbumEditionsCommandRequiresOneSelector(t *testing.T) {
	mb, dc := writeFixtureDBs(t)
	_, err := runCmd(t, "album-editions", "--musicbrainz-db", mb, "--discogs-db", dc)
	if err == nil {
		t.Error("album-editions with neither --rg nor --master must error")
	}
}
