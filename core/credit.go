package core

import "strings"

type Role string

const (
	RoleComposer  Role = "composer"
	RoleConductor Role = "conductor"
	RoleSoloist   Role = "soloist"
	RoleOrchestra Role = "orchestra"
	RoleChorus    Role = "chorus"
	RoleArtist    Role = "artist"
	RoleProducer  Role = "producer"
	RoleEngineer  Role = "engineer"
	RoleMastering Role = "mastering"
)

type Credit struct {
	Role  Role              `json:"role"`
	Name  string            `json:"name"`
	Attrs map[string]string `json:"attrs,omitempty"`
}

// Credits is a set of role-tagged contributors; it replaces the single flattened
// artist string that made classical curation fail.
type Credits []Credit

// Names returns the contributor names for a role, in credit order.
func (cs Credits) Names(role Role) []string {
	var out []string
	for _, c := range cs {
		if c.Role == role {
			out = append(out, c.Name)
		}
	}
	return out
}

// Has reports whether name appears under role (exact match).
func (cs Credits) Has(role Role, name string) bool {
	for _, c := range cs {
		if c.Role == role && c.Name == name {
			return true
		}
	}
	return false
}

// performingRoles are the credit roles that count as performing a recording (as
// opposed to composing or engineering it). PerformedBy matches against these.
var performingRoles = map[Role]bool{
	RoleArtist: true, RoleSoloist: true, RoleOrchestra: true,
	RoleChorus: true, RoleConductor: true,
}

// Performs reports whether name appears among the performing-role credits, using
// bidirectional case-insensitive substring matching (so "Tallis Scholars" matches
// "The Tallis Scholars"). ToLower approximates Python casefold for ASCII names.
func (cs Credits) Performs(name string) bool {
	n := strings.ToLower(name)
	for _, c := range cs {
		if !performingRoles[c.Role] {
			continue
		}
		cn := strings.ToLower(c.Name)
		if strings.Contains(cn, n) || strings.Contains(n, cn) {
			return true
		}
	}
	return false
}
