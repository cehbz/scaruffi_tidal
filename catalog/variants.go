package catalog

import (
	"unicode"

	"github.com/cehbz/tidalist/core"
)

// maxLatinVariants bounds the alias forms attached to one credit — enough to cover
// the Cyrillic/CJK-primary -> Latin case without inflating render's per-credit anchor
// query count (Tidal search is rate-limited).
const maxLatinVariants = 4

// LatinAliasVariants returns the Latin-script alias forms of a credited name that a
// Latin-scripted platform catalog (Tidal) indexes: it resolves the artist (role-aware,
// the same machinery credit matching uses) and keeps its artist_alias names that are
// Latin-script and not core.NormalizeName-equal to name, deduped, deterministic order,
// capped at maxLatinVariants. Empty (never an error) when the name does not resolve or
// carries no distinct Latin alias — a Latin-primary artist yields nothing to add.
func (m *MirrorDB) LatinAliasVariants(name string, role core.Role) ([]string, error) {
	variants, err := m.nameVariantsForRole(name, role)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{core.NormalizeName(name): true}
	var out []string
	for _, v := range variants {
		k := core.NormalizeName(v)
		if seen[k] || !isLatinScript(v) {
			continue
		}
		seen[k] = true
		out = append(out, v)
		if len(out) >= maxLatinVariants {
			break
		}
	}
	return out, nil
}

// isLatinScript reports whether every letter rune in s is in the Latin script
// (non-letters ignored, at least one letter required). A Cyrillic/CJK primary name
// returns false; its romanized aliases return true.
func isLatinScript(s string) bool {
	hasLetter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
			if !unicode.Is(unicode.Latin, r) {
				return false
			}
		}
	}
	return hasLetter
}
