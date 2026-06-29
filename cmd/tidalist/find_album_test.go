package main

import (
	"encoding/json"
	"testing"
)

func TestFindAlbumCommandEmitsJSON(t *testing.T) {
	mb, dc := writeFixtureDBs(t)
	out, err := runCmd(t, "find-album", "--title", "John Barleycorn Must Die",
		"--credit", "artist:Traffic", "--musicbrainz-db", mb, "--discogs-db", dc)
	if err != nil {
		t.Fatalf("execute: %v (out=%s)", err, out)
	}
	var got struct {
		Candidates []struct {
			MBID            string   `json:"mbid"`
			DiscogsMasterID int64    `json:"discogs_master_id"`
			Sources         []string `json:"sources"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not candidates JSON: %v\n%s", err, out)
	}
	if len(got.Candidates) != 2 {
		t.Fatalf("expected 2 source-tagged candidates (MB + Discogs), got %d", len(got.Candidates))
	}
}
