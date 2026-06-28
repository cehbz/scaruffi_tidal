package main

import (
	"testing"

	"github.com/cehbz/tidalist/core"
)

func TestParseCreditBasic(t *testing.T) {
	c, err := parseCredit("composer:Palestrina")
	if err != nil {
		t.Fatal(err)
	}
	if c.Role != core.RoleComposer || c.Name != "Palestrina" {
		t.Errorf("got %+v", c)
	}
}

func TestParseCreditNameWithSpacesAndAttrs(t *testing.T) {
	c, err := parseCredit("soloist:Emma Kirkby:instrument=soprano")
	if err != nil {
		t.Fatal(err)
	}
	if c.Role != core.RoleSoloist || c.Name != "Emma Kirkby" {
		t.Errorf("name/role wrong: %+v", c)
	}
	if c.Attrs["instrument"] != "soprano" {
		t.Errorf("attrs = %v, want instrument=soprano", c.Attrs)
	}
}

func TestParseCreditUnknownRoleErrors(t *testing.T) {
	if _, err := parseCredit("bogus:Whoever"); err == nil {
		t.Error("an unknown role must error")
	}
}

func TestParseCreditMissingColonErrors(t *testing.T) {
	if _, err := parseCredit("Palestrina"); err == nil {
		t.Error("a credit without role:name must error")
	}
}
