package main

import (
	"encoding/json"
	"testing"
)

func TestResolveWorkCommandEmitsJSON(t *testing.T) {
	mb, dc := writeFixtureDBs(t)
	out, err := runCmd(t, "resolve-work", "--name", "Missa Papae Marcelli",
		"--musicbrainz-db", mb, "--discogs-db", dc)
	if err != nil {
		t.Fatalf("execute: %v (out=%s)", err, out)
	}
	var got struct {
		Candidates []struct {
			MBID      string   `json:"mbid"`
			Composers []string `json:"composers"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not candidates JSON: %v\n%s", err, out)
	}
	if len(got.Candidates) != 1 || got.Candidates[0].MBID != "work-mpm" {
		t.Fatalf("candidates = %+v", got.Candidates)
	}
	if len(got.Candidates[0].Composers) != 1 || got.Candidates[0].Composers[0] != "Palestrina" {
		t.Errorf("composers = %v", got.Candidates[0].Composers)
	}
}
