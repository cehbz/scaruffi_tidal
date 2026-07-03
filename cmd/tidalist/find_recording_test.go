package main

import (
	"encoding/json"
	"testing"
)

func TestFindRecordingCommandEmitsJSON(t *testing.T) {
	mb, dc := writeFixtureDBs(t)
	out, err := runCmd(t, "find-recording", "--title", "Dear Mr. Fantasy",
		"--credit", "artist:Traffic", "--isrc", "GBABC1234567",
		"--musicbrainz-db", mb, "--discogs-db", dc)
	if err != nil {
		t.Fatalf("execute: %v (out=%s)", err, out)
	}
	var got struct {
		Candidates []struct {
			MBID  string `json:"mbid"`
			ISRC  string `json:"isrc"`
			Match struct {
				ArtistConfirmed *bool `json:"artist_confirmed"`
				ISRCExact       *bool `json:"isrc_exact"`
			} `json:"match"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not candidates JSON: %v\n%s", err, out)
	}
	if len(got.Candidates) != 1 || got.Candidates[0].MBID != "r-dmf" {
		t.Fatalf("candidates = %+v", got.Candidates)
	}
	if got.Candidates[0].Match.ArtistConfirmed == nil || !*got.Candidates[0].Match.ArtistConfirmed {
		t.Error("expected artist_confirmed=true")
	}
	if got.Candidates[0].Match.ISRCExact == nil || !*got.Candidates[0].Match.ISRCExact {
		t.Error("expected isrc_exact=true")
	}
}

func TestFindRecordingCommandByWork(t *testing.T) {
	mb, dc := writeFixtureDBs(t)
	out, err := runCmd(t, "find-recording", "--work", "Missa Papae Marcelli",
		"--musicbrainz-db", mb, "--discogs-db", dc)
	if err != nil {
		t.Fatalf("execute: %v (out=%s)", err, out)
	}
	var got struct {
		Candidates []struct {
			MBID string `json:"mbid"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not candidates JSON: %v\n%s", err, out)
	}
	if len(got.Candidates) != 3 || got.Candidates[0].MBID != "r-kyrie" {
		t.Fatalf("candidates = %+v", got.Candidates)
	}
}

func TestFindRecordingCommandByWorkSurfacesWorkResolution(t *testing.T) {
	mb, dc := writeFixtureDBs(t)
	out, err := runCmd(t, "find-recording", "--work", "Missa Papae Marcelli",
		"--musicbrainz-db", mb, "--discogs-db", dc)
	if err != nil {
		t.Fatalf("execute: %v (out=%s)", err, out)
	}
	var got struct {
		WorkResolution string `json:"work_resolution"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not candidates JSON: %v\n%s", err, out)
	}
	if got.WorkResolution != "title" {
		t.Errorf("work_resolution = %q, want %q\n%s", got.WorkResolution, "title", out)
	}
}

func TestFindRecordingCommandRejectsBadCredit(t *testing.T) {
	mb, dc := writeFixtureDBs(t)
	_, err := runCmd(t, "find-recording", "--title", "X", "--credit", "bogus:Y",
		"--musicbrainz-db", mb, "--discogs-db", dc)
	if err == nil {
		t.Error("an unknown credit role must fail the command")
	}
}

func TestFindRecordingCommandWorkCreditUmbrella(t *testing.T) {
	mb, dc := writeFixtureDBs(t)
	// conductor umbrella reaches the standalone chorus-master recording (rec 32).
	out, err := runCmd(t, "find-recording", "--work", "Missa Papae Marcelli",
		"--credit", "conductor:Barnaby Smith", "--musicbrainz-db", mb, "--discogs-db", dc)
	if err != nil {
		t.Fatalf("execute: %v (out=%s)", err, out)
	}
	var got struct {
		Candidates []struct {
			MBID string `json:"mbid"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not candidates JSON: %v\n%s", err, out)
	}
	if len(got.Candidates) != 1 || got.Candidates[0].MBID != "r-mpm-acap" {
		t.Fatalf("conductor umbrella should match the chorus-master recording; got %+v", got.Candidates)
	}
}
