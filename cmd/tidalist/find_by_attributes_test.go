package main

import (
	"encoding/json"
	"testing"
)

func TestFindByAttributesCommandEmitsJSON(t *testing.T) {
	mb, dc := writeFixtureDBs(t)
	out, err := runCmd(t, "find-by-attributes", "--style", "Psychedelic Rock",
		"--musicbrainz-db", mb, "--discogs-db", dc)
	if err != nil {
		t.Fatalf("execute: %v (out=%s)", err, out)
	}
	var got struct {
		Candidates []struct {
			DiscogsMasterID int64          `json:"discogs_master_id"`
			Title           string         `json:"title"`
			Artist          string         `json:"artist"`
			Match           map[string]any `json:"match"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not candidates JSON: %v\n%s", err, out)
	}
	if len(got.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %+v", got.Candidates)
	}
	c := got.Candidates[0]
	if c.DiscogsMasterID != 69017 {
		t.Errorf("discogs_master_id = %d, want 69017", c.DiscogsMasterID)
	}
	if c.Artist != "Traffic" {
		t.Errorf("artist = %q, want Traffic", c.Artist)
	}
	if _, ok := c.Match["year_match"]; ok {
		t.Errorf("year_match should be absent with no year window; got %v", c.Match)
	}
}

func TestFindByAttributesCommandYearWindowAndGenre(t *testing.T) {
	mb, dc := writeFixtureDBs(t)
	out, err := runCmd(t, "find-by-attributes", "--style", "Gothic Rock", "--genre", "Electronic",
		"--year-from", "1980", "--year-to", "1990",
		"--musicbrainz-db", mb, "--discogs-db", dc)
	if err != nil {
		t.Fatalf("execute: %v (out=%s)", err, out)
	}
	var got struct {
		Candidates []struct {
			DiscogsMasterID int64          `json:"discogs_master_id"`
			Match           map[string]any `json:"match"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not candidates JSON: %v\n%s", err, out)
	}
	if len(got.Candidates) != 1 || got.Candidates[0].DiscogsMasterID != 70003 {
		t.Fatalf("expected master 70003, got %+v", got.Candidates)
	}
	if ym, ok := got.Candidates[0].Match["year_match"].(bool); !ok || !ym {
		t.Errorf("year_match = %v, want true", got.Candidates[0].Match["year_match"])
	}
}

func TestFindByAttributesCommandEmptyResult(t *testing.T) {
	mb, dc := writeFixtureDBs(t)
	out, err := runCmd(t, "find-by-attributes", "--style", "Nonexistent Style",
		"--musicbrainz-db", mb, "--discogs-db", dc)
	if err != nil {
		t.Fatalf("execute: %v (out=%s)", err, out)
	}
	var got struct {
		Candidates []any `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not candidates JSON: %v\n%s", err, out)
	}
	if got.Candidates == nil || len(got.Candidates) != 0 {
		t.Fatalf("expected {\"candidates\":[]}, got %s", out)
	}
}

func TestFindByAttributesCommandRequiresStyleOrGenre(t *testing.T) {
	mb, dc := writeFixtureDBs(t)
	_, err := runCmd(t, "find-by-attributes", "--musicbrainz-db", mb, "--discogs-db", dc)
	if err == nil {
		t.Error("expected an error when neither --style nor --genre is given")
	}
}
