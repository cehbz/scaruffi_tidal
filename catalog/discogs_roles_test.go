package catalog

import (
	"testing"

	"github.com/cehbz/tidalist/core"
)

func TestDiscogsRoleCrosswalk(t *testing.T) {
	cases := map[string]core.Role{
		"Conductor":      core.RoleConductor,
		"Conducted By":   core.RoleConductor,
		"Orchestra":      core.RoleOrchestra,
		"Ensemble":       core.RoleOrchestra,
		"Chorus Master":  core.RoleChorusMaster,
		"Chorus":         core.RoleChorus,
		"Choir":          core.RoleChorus,
		"Composed By":    core.RoleComposer,
		"Cello":          core.RoleSoloist,
		"Piano":          core.RoleSoloist,
		"Lacquer Cut By": core.Role(""),
	}
	for in, want := range cases {
		if got := discogsRole(in); got != want {
			t.Errorf("discogsRole(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDiscogsRolesCombined(t *testing.T) {
	cases := map[string][]core.Role{
		"Composed By, Conductor":           {core.RoleComposer, core.RoleConductor},
		"Conductor, Composed By":           {core.RoleConductor, core.RoleComposer},
		"Conductor [Orchestra]":            {core.RoleConductor},
		"Chorus Master [Camerata Singers]": {core.RoleChorusMaster},
		"Composed By":                      {core.RoleComposer},
		"Producer, Engineer":               nil, // neither maps
		"Cello, Composed By":               {core.RoleSoloist, core.RoleComposer},
	}
	for in, want := range cases {
		got := discogsRoles(in)
		if !sameRoleSet(got, want) {
			t.Errorf("discogsRoles(%q) = %v, want %v", in, got, want)
		}
	}
}

func sameRoleSet(a, b []core.Role) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[core.Role]bool{}
	for _, r := range a {
		m[r] = true
	}
	for _, r := range b {
		if !m[r] {
			return false
		}
	}
	return true
}
