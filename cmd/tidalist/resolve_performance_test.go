package main

import (
	"encoding/json"
	"testing"
)

func TestResolvePerformanceCommandCaptures(t *testing.T) {
	mb, dc := writeFixtureDBs(t)
	out, err := runCmd(t, "resolve-performance",
		"--work", "Symphony no. 5 in C minor, op. 67",
		"--credit", "composer:Beethoven",
		"--credit", "conductor:Leonard Bernstein",
		"--credit", "orchestra:New York Philharmonic",
		"--year", "1963",
		"--musicbrainz-db", mb, "--discogs-db", dc)
	if err != nil {
		t.Fatalf("execute: %v (out=%s)", err, out)
	}
	var got struct {
		Outcome      string `json:"outcome"`
		Performances []struct {
			FirstYear     int    `json:"first_year"`
			DiscogsMaster int64  `json:"discogs_master_id"`
			Confidence    string `json:"confidence"`
		} `json:"performances"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not PerformanceResult JSON: %v\n%s", err, out)
	}
	if got.Outcome != "captured" || len(got.Performances) != 1 {
		t.Fatalf("want captured×1, got %q ×%d", got.Outcome, len(got.Performances))
	}
	if got.Performances[0].FirstYear != 1963 || got.Performances[0].Confidence != "high" {
		t.Errorf("captured perf = %+v", got.Performances[0])
	}
}

func TestResolvePerformanceCommandCandidates(t *testing.T) {
	mb, dc := writeFixtureDBs(t)
	out, err := runCmd(t, "resolve-performance",
		"--work", "Symphony no. 5 in C minor, op. 67",
		"--credit", "composer:Beethoven",
		"--credit", "conductor:Leonard Bernstein",
		"--credit", "orchestra:New York Philharmonic",
		"--musicbrainz-db", mb, "--discogs-db", dc)
	if err != nil {
		t.Fatalf("execute: %v (out=%s)", err, out)
	}
	var got struct {
		Outcome      string        `json:"outcome"`
		Performances []interface{} `json:"performances"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, out)
	}
	if got.Outcome != "candidates" || len(got.Performances) != 2 {
		t.Fatalf("want candidates×2, got %q ×%d", got.Outcome, len(got.Performances))
	}
}
