package core

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
