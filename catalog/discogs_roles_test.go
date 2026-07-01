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
