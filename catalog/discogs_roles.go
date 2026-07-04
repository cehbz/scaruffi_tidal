package catalog

import (
	"strings"

	"github.com/cehbz/tidalist/core"
)

// discogsRole maps a free-text Discogs release_artist.role to a core.Role, or "" when
// it is not a performing/composing role we key on. Casefold substring, most specific
// first. Instrument-as-role (Cello/Piano/Violin/…) folds to soloist.
func discogsRole(s string) core.Role {
	l := strings.ToLower(strings.TrimSpace(s))
	switch {
	case l == "":
		return ""
	case strings.Contains(l, "chorus master") || strings.Contains(l, "choir master") || strings.Contains(l, "chorus-master"):
		return core.RoleChorusMaster
	case strings.Contains(l, "conduct"): // conductor, conducted by
		return core.RoleConductor
	case strings.Contains(l, "orchestra") || strings.Contains(l, "ensemble") || strings.Contains(l, "philharmon") || strings.Contains(l, "symphony orchestra"):
		return core.RoleOrchestra
	case strings.Contains(l, "chorus") || strings.Contains(l, "choir"):
		return core.RoleChorus
	case strings.Contains(l, "compos"): // composed by, composer
		return core.RoleComposer
	case instrumentRole(l):
		return core.RoleSoloist
	default:
		return ""
	}
}

// discogsRoles maps a Discogs role string to the distinct set of core.Roles it
// denotes. Discogs credits are free-text and combined ("Composed By, Conductor")
// and bracket-qualified ("Conductor [Orchestra, Chorus]"); the bracketed qualifier
// can itself contain commas, so every bracketed span is stripped BEFORE the comma
// split (else a qualifier's inner comma fragments into a spurious role). Each
// fragment maps via discogsRole and dedupes. One credit can thus satisfy two
// constraints (a self-conducting composer).
func discogsRoles(s string) []core.Role {
	seen := map[core.Role]bool{}
	var out []core.Role
	for _, frag := range strings.Split(stripBrackets(s), ",") {
		if r := discogsRole(frag); r != "" && !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}

// stripBrackets removes every bracketed "[...]" span from s (depth-tracked, so an
// unmatched ']' is a no-op rather than corrupting subsequent text).
func stripBrackets(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// instrumentRole reports whether a role string names a solo instrument/voice.
func instrumentRole(l string) bool {
	for _, inst := range []string{
		"cello", "violin", "viola", "piano", "harpsichord", "organ", "flute",
		"oboe", "clarinet", "trumpet", "horn", "guitar", "soprano", "alto",
		"tenor", "bass", "baritone", "mezzo", "vocals", "voice", "soloist",
	} {
		if strings.Contains(l, inst) {
			return true
		}
	}
	return false
}
