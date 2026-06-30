package intent

import (
	"strings"
	"testing"

	"github.com/cehbz/tidalist/core"
)

func validate(t *testing.T, src string) (Doc, []Diagnostic) {
	t.Helper()
	doc, ds := Parse([]byte(src))
	ds = append(ds, Validate(&doc)...)
	return doc, ds
}

func TestValidateAcceptsGoodDoc(t *testing.T) {
	_, ds := validate(t, sample) // sample from intent_test.go
	if HasError(ds) {
		t.Fatalf("unexpected errors: %v", ds)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := map[string]string{
		"no name":            "## T · album\n- artist: X\n",
		"no kind":            "# X\n## T\n- artist: Y\n",
		"unknown kind":       "# X\n## T · single\n- artist: Y\n",
		"no title":           "# X\n##  · album\n- artist: Y\n",
		"unknown criterion":  "# X\nBrief: loud\n## T · album\n- artist: Y\n",
		"empty performed-by": "# X\n## T · recording\n- criteria: performed-by:\n- artist: Y\n",
		"bad disposition":    "# X\n## T · album\n- disposition: maybe\n- artist: Y\n",
	}
	for name, src := range cases {
		_, ds := validate(t, src)
		if !HasError(ds) {
			t.Errorf("%s: expected error, got %v", name, ds)
		}
	}
}

func TestValidateFoldsSynonym(t *testing.T) {
	doc, ds := validate(t, "# X\nBrief: no comps\n## T · album\n- artist: Y\n")
	if HasError(ds) {
		t.Fatalf("unexpected errors: %v", ds)
	}
	if len(doc.Brief) != 1 || doc.Brief[0] != "no-compilation" {
		t.Errorf("Brief not folded: %v", doc.Brief)
	}
	var warned bool
	for _, d := range ds {
		if d.Severity == SevWarning && strings.Contains(d.Msg, "no-compilation") {
			warned = true
		}
	}
	if !warned {
		t.Error("expected a normalization warning")
	}
}

func TestValidatePerformedBy(t *testing.T) {
	doc, ds := validate(t, "# X\n## T · recording\n- criteria: performed-by:Glenn Gould\n- artist: Y\n")
	if HasError(ds) {
		t.Fatalf("unexpected errors: %v", ds)
	}
	if doc.Items[0].Criteria[0] != "performed-by: Glenn Gould" {
		t.Errorf("performed-by not canonicalized: %q", doc.Items[0].Criteria[0])
	}
	_ = core.KindRecording
}
