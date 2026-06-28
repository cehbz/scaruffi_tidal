package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/cehbz/tidalist/internal/mirrorfixture"
)

// runCmd executes the root command with args, returning stdout.
func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

// writeFixtureDBs builds the shared SQLite fixture in a temp dir and returns the paths.
func writeFixtureDBs(t *testing.T) (mbPath, dcPath string) {
	t.Helper()
	mb, dc, err := mirrorfixture.Build(t.TempDir())
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	return mb, dc
}

func TestResolveArtistCommandEmitsJSON(t *testing.T) {
	mb, dc := writeFixtureDBs(t)
	out, err := runCmd(t, "resolve-artist", "--name", "Traffic",
		"--musicbrainz-db", mb, "--discogs-db", dc)
	if err != nil {
		t.Fatalf("execute: %v (out=%s)", err, out)
	}
	var got struct {
		Candidates []struct {
			MBID string `json:"mbid"`
			Name string `json:"name"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not the candidates JSON: %v\n%s", err, out)
	}
	if len(got.Candidates) == 0 || got.Candidates[0].Name != "Traffic" {
		t.Errorf("candidates = %+v", got.Candidates)
	}
}
