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
// combining marks (so "Brüggen" matches "Bruggen"), and map curly apostrophes and
// the non-breaking hyphen to ASCII. Mirrors the mirrors' FTS remove_diacritics so
// the Go filter and FTS agree.
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
	return punctuationFolder.Replace(out)
}

// punctuationFolder maps the punctuation variants MB uses to their ASCII
// equivalents, so ASCII-punctuation queries match: curly apostrophes (U+2019/
// U+2018) and the modifier letter apostrophe (U+02BC) fold to "'"; the non-breaking
// hyphen (U+2011, e.g. "Yo‑Yo Ma") folds to "-". Only variants confirmed present in
// MB data are listed here (not the full Unicode hyphen/dash family: U+2010 HYPHEN
// and U+2012 FIGURE DASH are unobserved in this corpus and deliberately omitted).
var punctuationFolder = strings.NewReplacer(
	"’", "'",
	"‘", "'",
	"ʼ", "'",
	"‑", "-",
)

// NormalizeName exposes the matching fold (NFD + strip combining marks + fold curly
// apostrophes + lowercase) for raw name-key comparisons outside this package.
func NormalizeName(s string) string { return normalizeName(s) }

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
