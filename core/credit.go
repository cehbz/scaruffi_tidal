package core

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

type Role string

const (
	RoleComposer     Role = "composer"
	RoleConductor    Role = "conductor"
	RoleSoloist      Role = "soloist"
	RoleOrchestra    Role = "orchestra"
	RoleChorus       Role = "chorus"
	RoleChorusMaster Role = "chorus_master"
	RoleArtist       Role = "artist"
	RoleProducer     Role = "producer"
	RoleEngineer     Role = "engineer"
	RoleMastering    Role = "mastering"
)

// roles is the canonical role vocabulary in precedence order. The order is
// load-bearing: it is the order credits are emitted in canonical intent markdown
// (creative leads, then performers, then production).
var roles = []Role{
	RoleComposer, RoleConductor, RoleSoloist, RoleOrchestra, RoleChorus,
	RoleChorusMaster, RoleArtist, RoleProducer, RoleEngineer, RoleMastering,
}

var roleSet = func() map[Role]bool {
	m := make(map[Role]bool, len(roles))
	for _, r := range roles {
		m[r] = true
	}
	return m
}()

// Roles returns the canonical role vocabulary in precedence order. The returned
// slice is a copy; callers may not mutate the package vocabulary.
func Roles() []Role { return append([]Role(nil), roles...) }

// ValidRole reports whether r is one of the known credit roles.
func ValidRole(r Role) bool { return roleSet[r] }

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

// normalizeName folds a name for matching: lowercase, NFD-decompose and strip
// combining marks (so "Brüggen" matches "Bruggen"), and map curly apostrophes to
// ASCII. Mirrors the mirrors' FTS remove_diacritics so the Go filter and FTS agree.
func normalizeName(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) { // Mn = combining mark; drop it
			continue
		}
		b.WriteRune(r)
	}
	out := strings.ToLower(b.String())
	return apostropheFolder.Replace(out)
}

// apostropheFolder maps the apostrophe variants MB uses (curly U+2019/U+2018 and
// the modifier letter apostrophe U+02BC) to ASCII, so ASCII-apostrophe queries match.
var apostropheFolder = strings.NewReplacer(
	"’", "'",
	"‘", "'",
	"ʼ", "'",
)

// MatchesRole reports whether some credit in the given role has a name matching
// name by bidirectional case-insensitive substring (so "Tallis Scholars" matches
// "The Tallis Scholars"). Names are normalized (diacritics/apostrophes folded) so
// the Go filter agrees with the FTS layer. Used to filter by a requested credit.
func (cs Credits) MatchesRole(role Role, name string) bool {
	n := normalizeName(name)
	if n == "" {
		return false
	}
	for _, c := range cs {
		if c.Role != role {
			continue
		}
		cn := normalizeName(c.Name)
		if cn == "" {
			continue
		}
		if strings.Contains(cn, n) || strings.Contains(n, cn) {
			return true
		}
	}
	return false
}

// performingRoles are the credit roles that count as performing a recording (as
// opposed to composing or engineering it). PerformedBy matches against these.
var performingRoles = map[Role]bool{
	RoleArtist: true, RoleSoloist: true, RoleOrchestra: true,
	RoleChorus: true, RoleConductor: true, RoleChorusMaster: true,
}

// Performs reports whether name appears among the performing-role credits, using
// bidirectional case-insensitive substring matching (so "Tallis Scholars" matches
// "The Tallis Scholars"). Names are normalized (diacritics/apostrophes folded) so
// the Go filter agrees with the FTS layer.
func (cs Credits) Performs(name string) bool {
	n := normalizeName(name)
	if n == "" {
		return false
	}
	for _, c := range cs {
		if !performingRoles[c.Role] {
			continue
		}
		cn := normalizeName(c.Name)
		if cn == "" {
			continue
		}
		if strings.Contains(cn, n) || strings.Contains(n, cn) {
			return true
		}
	}
	return false
}
