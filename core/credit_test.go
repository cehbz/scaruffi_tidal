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

func TestCreditsMatchesRole(t *testing.T) {
	cs := Credits{{Role: RoleOrchestra, Name: "The Tallis Scholars"}, {Role: RoleConductor, Name: "Peter Phillips"}}
	if !cs.MatchesRole(RoleOrchestra, "Tallis Scholars") {
		t.Error("bidirectional substring within the right role should match")
	}
	if cs.MatchesRole(RoleConductor, "Tallis Scholars") {
		t.Error("must not match across roles")
	}
	if cs.MatchesRole(RoleSoloist, "Phillips") {
		t.Error("absent role must not match")
	}
}

func TestCreditsMatchesRoleNormalizes(t *testing.T) {
	// Diacritic fold: MB stores "Brüggen"; an ASCII query "Bruggen" must match.
	cs := Credits{{Role: RoleConductor, Name: "Frans Brüggen"}}
	if !cs.MatchesRole(RoleConductor, "Frans Bruggen") {
		t.Error("ASCII query should match a diacritic-bearing credit name")
	}
	if !cs.MatchesRole(RoleConductor, "Brüggen") {
		t.Error("diacritic query should still match")
	}
	// Apostrophe fold: MB stores U+2019; an ASCII-apostrophe query must match.
	ap := Credits{{Role: RoleChorusMaster, Name: "James O’Donnell"}}
	if !ap.MatchesRole(RoleChorusMaster, "James O'Donnell") {
		t.Error("ASCII-apostrophe query should match a U+2019 credit name")
	}
	// Empty query name must never match.
	if cs.MatchesRole(RoleConductor, "") {
		t.Error("empty query name must return false")
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

func TestCreditsPerformsChorusMaster(t *testing.T) {
	cs := Credits{{Role: RoleChorusMaster, Name: "Barnaby Smith"}}
	if !cs.Performs("Barnaby Smith") {
		t.Error("chorus_master is a performing role and should match")
	}
}

func TestCreditsPerformsNormalizes(t *testing.T) {
	cs := Credits{{Role: RoleConductor, Name: "Frans Brüggen"}}
	if !cs.Performs("Frans Bruggen") {
		t.Error("ASCII query should match a diacritic-bearing performing credit")
	}
	ap := Credits{{Role: RoleChorusMaster, Name: "James O’Donnell"}}
	if !ap.Performs("James O'Donnell") {
		t.Error("ASCII-apostrophe query should match a U+2019 performing credit")
	}
	if cs.Performs("") {
		t.Error("empty query name must return false")
	}
}

func TestRolesVocabulary(t *testing.T) {
	roles := Roles()
	if len(roles) != 10 {
		t.Fatalf("Roles() len = %d, want 10", len(roles))
	}
	// Precedence order is load-bearing (canonical credit ordering).
	want := []Role{
		RoleComposer, RoleConductor, RoleSoloist, RoleOrchestra, RoleChorus,
		RoleChorusMaster, RoleArtist, RoleProducer, RoleEngineer, RoleMastering,
	}
	for i, r := range want {
		if roles[i] != r {
			t.Errorf("Roles()[%d] = %q, want %q", i, roles[i], r)
		}
	}
	for _, r := range want {
		if !ValidRole(r) {
			t.Errorf("ValidRole(%q) = false, want true", r)
		}
	}
	if ValidRole(Role("pianist")) {
		t.Error("ValidRole(\"pianist\") = true, want false")
	}
}

func TestRolesIsACopy(t *testing.T) {
	a := Roles()
	a[0] = "mutated"
	if Roles()[0] != RoleComposer {
		t.Error("Roles() returns a shared slice; callers can corrupt the vocabulary")
	}
}

func TestNormalizeNameExported(t *testing.T) {
	if NormalizeName("Sir Georg Solti") != normalizeName("Sir Georg Solti") {
		t.Error("exported NormalizeName must equal the internal fold")
	}
	if NormalizeName("Brüggen") != "bruggen" {
		t.Errorf("diacritics must fold: got %q", NormalizeName("Brüggen"))
	}
}

func TestNormalizeNameFoldsNonBreakingHyphen(t *testing.T) {
	// MB stores "Yo‑Yo Ma" with U+2011 NON-BREAKING HYPHEN; an ASCII-hyphen query
	// must fold to the same key so the Go filter agrees with the FTS layer.
	nbHyphen, ascii := NormalizeName("Yo‑Yo Ma"), NormalizeName("Yo-Yo Ma")
	if nbHyphen != ascii {
		t.Errorf("non-breaking hyphen must fold to ASCII: got %q, want %q", nbHyphen, ascii)
	}
	cs := Credits{{Role: RoleArtist, Name: "Yo‑Yo Ma"}}
	if !cs.MatchesRole(RoleArtist, "Yo-Yo Ma") {
		t.Error("ASCII-hyphen query should match a non-breaking-hyphen credit name")
	}
}
