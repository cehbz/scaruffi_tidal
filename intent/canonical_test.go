package intent

import (
	"strings"
	"testing"
)

func TestCanonicalRoundTripAndOrder(t *testing.T) {
	// Credits given out of precedence order; canonical must reorder them.
	src := `# Demo
Brief: studio

## Missa Papae Marcelli · album
- soloist: Emma Kirkby (soprano)
- composer: Palestrina
- note: a note
- year: 1980
- work: Missa Papae Marcelli
`
	doc, ds := Parse([]byte(src))
	ds = append(ds, Validate(&doc)...)
	if HasError(ds) {
		t.Fatalf("errors: %v", ds)
	}
	got := string(Canonical(doc))
	want := `# Demo
Brief: studio

## Missa Papae Marcelli · album
- composer: Palestrina
- soloist: Emma Kirkby (soprano)
- work: Missa Papae Marcelli
- year: 1980
- note: a note
`
	if got != want {
		t.Errorf("canonical mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestCanonicalIdempotent(t *testing.T) {
	doc, _ := Parse([]byte(sample))
	Validate(&doc)
	once := Canonical(doc)
	doc2, _ := Parse(once)
	Validate(&doc2)
	twice := Canonical(doc2)
	if string(once) != string(twice) {
		t.Errorf("not idempotent:\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
	}
}

func TestCanonicalExplicitAttrs(t *testing.T) {
	doc, _ := Parse([]byte("# X\n## T · recording\n- soloist: Glenn Gould (instrument=piano)\n"))
	Validate(&doc)
	got := string(Canonical(doc))
	if !strings.Contains(got, "- soloist: Glenn Gould (piano)") {
		t.Errorf("instrument attr not emitted as bare shorthand:\n%s", got)
	}
}

func TestSummary(t *testing.T) {
	doc, _ := Parse([]byte(sample))
	Validate(&doc)
	s := Summary(doc)
	if !strings.Contains(s, "2 items") || !strings.Contains(s, "1 album") || !strings.Contains(s, "1 recording") {
		t.Errorf("summary = %q", s)
	}
}
