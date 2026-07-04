package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaterializeGoldenCommand(t *testing.T) {
	mb, dc := writeFixtureDBs(t)
	sel := `{
	  "name": "CLI Test",
	  "brief": {"criteria": []},
	  "selections": [
	    {"kind": "album", "rg_mbid": "rg-jbmd", "provenance": {"source": "t", "note": "n"}}
	  ]
	}`
	dir := t.TempDir()
	selPath := filepath.Join(dir, "sel.json")
	if err := os.WriteFile(selPath, []byte(sel), 0o644); err != nil {
		t.Fatal(err)
	}
	repPath := filepath.Join(dir, "report.json")
	out, err := runCmd(t, "materialize-golden", selPath, "--report", repPath,
		"--musicbrainz-db", mb, "--discogs-db", dc)
	if err != nil {
		t.Fatalf("execute: %v (out=%s)", err, out)
	}
	var doc struct {
		Name    string           `json:"name"`
		Entries []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not golden JSON: %v\n%s", err, out)
	}
	if doc.Name != "CLI Test" || len(doc.Entries) != 1 {
		t.Fatalf("doc = %q ×%d", doc.Name, len(doc.Entries))
	}
	if doc.Entries[0]["title"] != "John Barleycorn Must Die" {
		t.Errorf("entry title = %v", doc.Entries[0]["title"])
	}
	rb, err := os.ReadFile(repPath)
	if err != nil {
		t.Fatalf("report not written: %v", err)
	}
	var rep struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rb, &rep); err != nil {
		t.Fatalf("bad report JSON: %v", err)
	}
	if len(rep.Items) != 1 {
		t.Errorf("report items = %d", len(rep.Items))
	}
}

func TestMaterializeGoldenReportMdFlag(t *testing.T) {
	mb, dc := writeFixtureDBs(t)
	sel := `{
	  "name": "Markdown Test",
	  "brief": {"criteria": []},
	  "selections": [
	    {"kind": "album", "rg_mbid": "rg-jbmd", "provenance": {"source": "t", "note": "test item"}}
	  ]
	}`
	dir := t.TempDir()
	selPath := filepath.Join(dir, "sel.json")
	if err := os.WriteFile(selPath, []byte(sel), 0o644); err != nil {
		t.Fatal(err)
	}
	reportMdPath := filepath.Join(dir, "report.md")
	_, err := runCmd(t, "materialize-golden", selPath, "--report-md", reportMdPath,
		"--musicbrainz-db", mb, "--discogs-db", dc)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	mdBytes, err := os.ReadFile(reportMdPath)
	if err != nil {
		t.Fatalf("report markdown not written: %v", err)
	}
	md := string(mdBytes)
	if !strings.Contains(md, "# Curate report: Markdown Test") {
		t.Errorf("missing or incorrect header in:\n%s", md)
	}
	if !strings.Contains(md, "1 items:") {
		t.Errorf("missing or incorrect count line in:\n%s", md)
	}
}
