package catalog

import (
	"testing"

	"github.com/cehbz/tidalist/internal/mirrorfixture"
)

// newTestMirror builds the shared SQLite fixture in a temp dir and opens a
// read-only MirrorDB over it. Reused by every catalog test.
func newTestMirror(t *testing.T) *MirrorDB {
	t.Helper()
	mbPath, dcPath, err := mirrorfixture.Build(t.TempDir())
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	m, err := Open(mbPath, dcPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { m.Close() })
	return m
}

func TestMirrorOpensAndAttaches(t *testing.T) {
	m := newTestMirror(t)
	var name string
	if err := m.DB.QueryRow(`SELECT name FROM artist WHERE id = 1`).Scan(&name); err != nil {
		t.Fatalf("read MB: %v", err)
	}
	if name != "Traffic" {
		t.Errorf("artist name = %q, want Traffic", name)
	}
	var ok int
	if err := m.DB.QueryRow(`SELECT ok FROM dc.dc_marker`).Scan(&ok); err != nil {
		t.Fatalf("read dc: %v", err)
	}
	if ok != 1 {
		t.Errorf("dc.dc_marker ok = %d, want 1", ok)
	}
}

func TestEscapeFTS(t *testing.T) {
	if got := escapeFTS(`Dear "Mr." Fantasy`); got != `"Dear ""Mr."" Fantasy"` {
		t.Errorf("escapeFTS = %s", got)
	}
	if got := ftsTitle("Mr. Fantasy"); got != `title:"Mr. Fantasy"` {
		t.Errorf("ftsTitle = %s", got)
	}
}
