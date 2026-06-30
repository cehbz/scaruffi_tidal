package intent

import (
	"testing"

	"github.com/cehbz/tidalist/core"
)

const sample = `# Demo
Brief: studio; no-compilation

## Missa Papae Marcelli · album
- composer: Palestrina
- conductor: Peter Phillips
- soloist: Emma Kirkby (soprano)
- work: Missa Papae Marcelli
- year: 1980
- note: Tallis Scholars, 1980
- note: second note

## In the Court of the Crimson King · recording
- artist: King Crimson
- rendering: steven-wilson
`

func TestParseDocument(t *testing.T) {
	doc, ds := Parse([]byte(sample))
	if HasError(ds) {
		t.Fatalf("unexpected errors: %v", ds)
	}
	if doc.Name != "Demo" {
		t.Errorf("Name = %q, want Demo", doc.Name)
	}
	if len(doc.Brief) != 2 || doc.Brief[0] != "studio" || doc.Brief[1] != "no-compilation" {
		t.Errorf("Brief = %v", doc.Brief)
	}
	if len(doc.Items) != 2 {
		t.Fatalf("Items len = %d, want 2", len(doc.Items))
	}
	it := doc.Items[0]
	if it.Title != "Missa Papae Marcelli" || it.Kind != core.KindAlbum {
		t.Errorf("item0 title/kind = %q/%q", it.Title, it.Kind)
	}
	if it.Work != "Missa Papae Marcelli" || it.Year != 1980 {
		t.Errorf("item0 work/year = %q/%d", it.Work, it.Year)
	}
	if len(it.Notes) != 2 {
		t.Errorf("item0 notes = %v", it.Notes)
	}
	if len(it.Credits) != 3 {
		t.Fatalf("item0 credits = %v", it.Credits)
	}
	sol := it.Credits[2]
	if sol.Role != core.RoleSoloist || sol.Name != "Emma Kirkby" || sol.Attrs["instrument"] != "soprano" {
		t.Errorf("soloist parsed as %+v", sol)
	}
	if doc.Items[1].Kind != core.KindRecording || len(doc.Items[1].Rendering) != 1 {
		t.Errorf("item1 = %+v", doc.Items[1])
	}
}

func TestParseStructuralErrors(t *testing.T) {
	cases := map[string]string{
		"bullet before heading": "# X\n- artist: Y\n",
		"unknown field/role":    "# X\n## T · album\n- pianist: Glenn Gould\n",
		"non-integer year":      "# X\n## T · album\n- year: nineteen\n",
		"bullet without colon":  "# X\n## T · album\n- justtext\n",
	}
	for name, src := range cases {
		_, ds := Parse([]byte(src))
		if !HasError(ds) {
			t.Errorf("%s: expected an error diagnostic, got %v", name, ds)
		}
	}
}

func TestParseExplicitAttrs(t *testing.T) {
	doc, ds := Parse([]byte("# X\n## T · recording\n- soloist: Glenn Gould (instrument=piano)\n"))
	if HasError(ds) {
		t.Fatalf("errors: %v", ds)
	}
	c := doc.Items[0].Credits[0]
	if c.Name != "Glenn Gould" || c.Attrs["instrument"] != "piano" {
		t.Errorf("credit = %+v", c)
	}
}
