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

func TestCanonicalEditionRoundTrip(t *testing.T) {
	src := `# X

## T · album
- edition: original, mofi, label=DG, catno=2530 516, year=1975
`
	doc, ds := Parse([]byte(src))
	ds = append(ds, Validate(&doc)...)
	if HasError(ds) {
		t.Fatalf("errors: %v", ds)
	}
	once := Canonical(doc)
	if !strings.Contains(string(once), "- edition: original, mofi, label=DG, catno=2530 516, year=1975") {
		t.Fatalf("edition cue not emitted in expected order:\n%s", once)
	}
	doc2, ds2 := Parse(once)
	ds2 = append(ds2, Validate(&doc2)...)
	if HasError(ds2) {
		t.Fatalf("errors on reparse: %v", ds2)
	}
	twice := Canonical(doc2)
	if string(once) != string(twice) {
		t.Errorf("edition canonical form not idempotent:\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
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
