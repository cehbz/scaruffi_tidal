package catalog

import (
	"testing"

	"github.com/cehbz/tidalist/core"
)

func TestLatinAliasVariants(t *testing.T) {
	m := newTestMirror(t)
	// Gergiev's MB primary name is Cyrillic; "Valery Gergiev" is a Latin artist_alias.
	got, err := m.LatinAliasVariants("Валерий Гергиев", core.RoleConductor)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "Valery Gergiev" {
		t.Errorf("LatinAliasVariants(Gergiev) = %v, want [Valery Gergiev]", got)
	}
}

func TestLatinAliasVariantsEmptyWhenAlreadyLatin(t *testing.T) {
	m := newTestMirror(t)
	// Peter Phillips resolves (exact-name arm) but has no distinct Latin alias.
	got, err := m.LatinAliasVariants("Peter Phillips", core.RoleConductor)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("LatinAliasVariants(Peter Phillips) = %v, want empty", got)
	}
}
