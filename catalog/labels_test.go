package catalog

import "testing"

func TestLabelFamily(t *testing.T) {
	m := newTestMirror(t)
	// CBS (11) is a sublabel of Columbia (10): same family.
	same, err := m.sameLabelFamily(11, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !same {
		t.Error("CBS and Columbia must be the same label family")
	}
	// Deutsche Grammophon (12) is unrelated.
	same, err = m.sameLabelFamily(11, 12)
	if err != nil {
		t.Fatal(err)
	}
	if same {
		t.Error("CBS and Deutsche Grammophon are different families")
	}
	// A label reaches itself.
	if same, _ := m.sameLabelFamily(12, 12); !same {
		t.Error("a label is its own family")
	}
}
