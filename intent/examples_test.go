package intent

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestExampleIntentsAreCleanAndCanonical is the automated gate for the interpret
// demonstration: every examples/*-intent.md must parse + validate with no errors
// and already be in canonical form (Canonical(doc) byte-equals the source). The
// `tidalist lint-intent --write` step that produced these files leaves them
// canonical, so this test pins that invariant against future drift.
//
// The glob picks up every worked example uniformly — Scaruffi (compound
// classical credits), Winwood (a single-artist career thread), and Descriptor
// (a generated-from-predicate discography, slice-6 4A) — with no per-file
// wiring: dropping a new examples/<name>-intent.md into place is sufficient to
// gate it here.
func TestExampleIntentsAreCleanAndCanonical(t *testing.T) {
	paths, err := filepath.Glob("../examples/*-intent.md")
	if err != nil {
		t.Fatalf("glob examples: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no example intents found under ../examples/*-intent.md")
	}
	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			doc, ds := Parse(src)
			ds = append(ds, Validate(&doc)...)
			for _, d := range ds {
				if d.Severity == SevError {
					t.Errorf("%s: %s", path, d)
				}
			}
			if HasError(ds) {
				t.Fatalf("%s has validation errors", path)
			}
			if got := Canonical(doc); !bytes.Equal(got, src) {
				t.Errorf("%s is not canonical\n--- got ---\n%s\n--- want ---\n%s", path, got, src)
			}
		})
	}
}
