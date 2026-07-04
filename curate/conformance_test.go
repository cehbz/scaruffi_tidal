package curate

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// conformanceSelectionsJSON exercises the four disposition/identity shapes the
// golden schema must round-trip through the Python reader: an MB-resolved album,
// an MB-resolved track, an absent stub, and a Discogs-only album (no rg_mbid) —
// the branch TestMaterializeAlbumEntryDiscogsOnly covers on the Go side and this
// test proves the Python reader accepts too.
const conformanceSelectionsJSON = `{
  "name": "Conformance Fixture",
  "brief": {"criteria": [{"type": "no_live"}]},
  "selections": [
    {"kind": "album", "rg_mbid": "rg-jbmd",
     "edition": {"prefer_original": true},
     "provenance": {"source": "test", "note": "the JBMD pick"}},
    {"kind": "track", "recording_mbid": "r-glad",
     "criteria": [{"type": "studio"}],
     "provenance": {"source": "test", "note": "glad"},
     "marginal": "only one candidate considered"},
    {"kind": "album", "rg_mbid": "rg-nope", "artist": "Nobody", "title": "Nothing",
     "provenance": {"source": "test", "note": "absent"}},
    {"kind": "album", "discogs_master_id": 69017,
     "provenance": {"source": "test", "note": "discogs-only pick"}}
  ]
}`

// pyGoldenEntry mirrors the record printed by the conformance script below: one
// row per parsed GoldenEntry, precise enough to catch a schema divergence rather
// than merely a count mismatch.
type pyGoldenEntry struct {
	Kind            string  `json:"kind"`
	Admitted        bool    `json:"admitted"`
	MBID            *string `json:"mbid"`
	DiscogsMasterID *int64  `json:"discogs_master_id"`
}

// TestGoldenParsesInPython is a durable Go<->Python conformance gate: it
// materializes a fixture Golden Master exactly as curate.Materialize emits it,
// then feeds that JSON to the Python reader (tidalist.core.spec.from_golden) and
// asserts the parsed entries agree with the Go report entry-by-entry. It is a
// conformance gate, not a red/green feature — it must pass on first run. A
// failure here means the Go writer and Python reader have diverged on the wire
// schema; do not adapt either side to force a pass without understanding why.
//
// Not covered: the exact track kind label (the Python reader treats any
// non-"album" kind as a track) and fields beyond kind/admitted/identity —
// credits, isrc, performance, year, traits, tracklist, edition, and provenance
// are parsed by from_golden but not compared here.
func TestGoldenParsesInPython(t *testing.T) {
	if _, err := exec.LookPath("uv"); err != nil {
		t.Skip("uv not on PATH")
	}

	m := newTestMirror(t)
	sel, err := ParseSelections([]byte(conformanceSelectionsJSON))
	if err != nil {
		t.Fatalf("ParseSelections: %v", err)
	}
	doc, rep, err := Materialize(m, sel)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if len(doc.Entries) != len(sel.Items) {
		t.Fatalf("materialized %d entries, want %d (one per selection)", len(doc.Entries), len(sel.Items))
	}

	gmJSON, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal GoldenDoc: %v", err)
	}
	gmPath := filepath.Join(t.TempDir(), "gm.json")
	if err := os.WriteFile(gmPath, gmJSON, 0o644); err != nil {
		t.Fatalf("write gm.json: %v", err)
	}

	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}

	const py = `import json, sys
from tidalist.core.spec import from_golden
from tidalist.core.album import Album

g = from_golden(json.load(open(sys.argv[1])))
out = []
for e in g.entries:
    if isinstance(e.item, Album):
        out.append({"kind": "album", "admitted": e.verdict.admitted,
                     "mbid": e.item.ids.mbid,
                     "discogs_master_id": e.item.ids.discogs_master_id})
    else:
        out.append({"kind": "track", "admitted": e.verdict.admitted,
                     "mbid": e.item.mbid, "discogs_master_id": None})
print(json.dumps(out))
`
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "uv", "run", "python", "-c", py, gmPath)
	cmd.Dir = repoRoot
	// Parse stdout only: a cold-cache `uv run` writes sync/venv noise to stderr,
	// which must not pollute the JSON the gate unmarshals.
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("python from_golden timed out after 60s\nstdout: %s\nstderr: %s", out, stderr.String())
	}
	if err != nil {
		t.Fatalf("python from_golden failed: %v\nstdout: %s\nstderr: %s", err, out, stderr.String())
	}

	var got []pyGoldenEntry
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("parse python output: %v\nstdout: %s\nstderr: %s", err, out, stderr.String())
	}

	if len(got) != len(doc.Entries) {
		t.Fatalf("python parsed %d entries, want %d (Go entry count)", len(got), len(doc.Entries))
	}

	goAdmitted := 0
	for _, item := range rep.Items {
		if item.Admitted {
			goAdmitted++
		}
	}
	pyAdmitted := 0
	for _, e := range got {
		if e.Admitted {
			pyAdmitted++
		}
	}
	if pyAdmitted != goAdmitted {
		t.Fatalf("python admitted count = %d, want %d (Go report's admitted count)", pyAdmitted, goAdmitted)
	}

	for i, e := range got {
		if e.Kind != rep.Items[i].Kind {
			t.Errorf("entry[%d].kind = %q, want %q", i, e.Kind, rep.Items[i].Kind)
		}
		if e.Admitted != rep.Items[i].Admitted {
			t.Errorf("entry[%d].admitted = %v, want %v", i, e.Admitted, rep.Items[i].Admitted)
		}
	}

	// Entry 0: MB-resolved album (rg-jbmd) — carries both its MB identity and the
	// FK-linked Discogs master (69017), sourced from musicbrainz.
	if got[0].MBID == nil || *got[0].MBID != "rg-jbmd" {
		t.Errorf("entry[0].mbid = %v, want rg-jbmd", got[0].MBID)
	}
	if got[0].DiscogsMasterID == nil || *got[0].DiscogsMasterID != 69017 {
		t.Errorf("entry[0].discogs_master_id = %v, want 69017", got[0].DiscogsMasterID)
	}

	// Entry 1: MB-resolved track (r-glad).
	if got[1].MBID == nil || *got[1].MBID != "r-glad" {
		t.Errorf("entry[1].mbid = %v, want r-glad", got[1].MBID)
	}

	// Entry 2: absent album stub — no identity survives the miss.
	if got[2].MBID != nil {
		t.Errorf("entry[2].mbid = %v, want nil (absent stub)", *got[2].MBID)
	}
	if got[2].DiscogsMasterID != nil {
		t.Errorf("entry[2].discogs_master_id = %v, want nil (absent stub)", *got[2].DiscogsMasterID)
	}

	// Entry 3: Discogs-only album (master 69017, no rg_mbid) — the branch this
	// gate exists to cover. It must resolve to the same underlying release as
	// entry 0 (same discogs_master_id) while omitting the MB identity entirely,
	// proving the Python reader distinguishes source-of-identity, not just kind.
	if got[3].MBID != nil {
		t.Errorf("entry[3].mbid = %v, want nil (discogs-only must omit mbid)", *got[3].MBID)
	}
	if got[3].DiscogsMasterID == nil || *got[3].DiscogsMasterID != 69017 {
		t.Errorf("entry[3].discogs_master_id = %v, want 69017", got[3].DiscogsMasterID)
	}
	if !got[3].Admitted {
		t.Error("entry[3] (discogs-only) must be admitted (permissive-flagged; not_live/no_live unverifiable on Discogs)")
	}
}
