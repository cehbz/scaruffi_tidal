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
// and bracket-qualified ("Conductor [Orchestra]"); split on commas, strip the
// bracketed qualifier from each fragment, map each via discogsRole, and dedupe. One
// credit can thus satisfy two constraints (a self-conducting composer).
func discogsRoles(s string) []core.Role {
	seen := map[core.Role]bool{}
	var out []core.Role
	for _, frag := range strings.Split(s, ",") {
		if i := strings.IndexByte(frag, '['); i >= 0 {
			frag = frag[:i]
		}
		if r := discogsRole(frag); r != "" && !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
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
