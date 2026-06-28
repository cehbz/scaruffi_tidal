package core

import (
	"encoding/json"
	"reflect"
	"testing"
)

func sampleCredits() Credits {
	return Credits{
		{Role: RoleComposer, Name: "Palestrina"},
		{Role: RoleConductor, Name: "Peter Phillips"},
		{Role: RoleSoloist, Name: "Emma Kirkby", Attrs: map[string]string{"instrument": "soprano"}},
		{Role: RoleOrchestra, Name: "The Tallis Scholars"},
	}
}

func TestCreditsNames(t *testing.T) {
	got := sampleCredits().Names(RoleComposer)
	if !reflect.DeepEqual(got, []string{"Palestrina"}) {
		t.Errorf("Names(composer) = %v", got)
	}
	if n := sampleCredits().Names(RoleProducer); n != nil {
		t.Errorf("Names(producer) on a classical credit set = %v, want nil", n)
	}
}

func TestCreditsHas(t *testing.T) {
	cs := sampleCredits()
	if !cs.Has(RoleOrchestra, "The Tallis Scholars") {
		t.Error("expected orchestra credit")
	}
	if cs.Has(RoleConductor, "The Tallis Scholars") {
		t.Error("wrong role should not match")
	}
}

func TestCreditJSONRoundTrip(t *testing.T) {
	in := Credit{Role: RoleSoloist, Name: "Glenn Gould", Attrs: map[string]string{"instrument": "piano"}}
	b, _ := json.Marshal(in)
	var out Credit
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round-trip mismatch: %+v -> %s -> %+v", in, b, out)
	}
}

func TestCreditsPerforms(t *testing.T) {
	cs := Credits{
		{Role: RoleOrchestra, Name: "The Tallis Scholars"},
		{Role: RoleComposer, Name: "Palestrina"},
	}
	if !cs.Performs("Tallis Scholars") {
		t.Error("a performing-role credit should match by bidirectional substring")
	}
	if cs.Performs("Palestrina") {
		t.Error("composer is not a performing role")
	}
	if cs.Performs("The Beatles") {
		t.Error("an absent name must not match")
	}
}
