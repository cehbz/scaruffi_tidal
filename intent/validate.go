package intent

import (
	"fmt"
	"strings"

	"github.com/cehbz/tidalist/core"
)

const performedByPrefix = "performed-by:"

// validCriteria is the closed set of bare criteria tokens (kebab surface). The
// valued token performed-by:<name> is handled separately. These mirror the
// internal tags in core/criteria_json.go (studio/no_compilation/no_live).
var validCriteria = map[string]bool{
	"studio":         true,
	"no-compilation": true,
	"no-live":        true,
}

// criterionSynonyms folds common human phrasings onto canonical tokens. Kept
// minimal (YAGNI); grow only when a real intent needs it.
var criterionSynonyms = map[string]string{
	"no comps":       "no-compilation",
	"no compilation": "no-compilation",
	"no live":        "no-live",
}

// Validate checks d against the closed vocabulary and required fields, returning
// diagnostics. It normalizes criteria tokens in place (synonym folding emits a
// warning; performed-by spacing is canonicalized).
func Validate(d *Doc) []Diagnostic {
	var ds []Diagnostic
	if strings.TrimSpace(d.Name) == "" {
		ds = append(ds, Diagnostic{SevError, 0, "playlist needs a name (a '# ' heading)"})
	}
	d.Brief = normalizeCriteria(d.Brief, "Brief", &ds)
	for i := range d.Items {
		it := &d.Items[i]
		label := it.Title
		if strings.TrimSpace(label) == "" {
			ds = append(ds, Diagnostic{SevError, 0, "item has no title"})
			label = "(untitled)"
		}
		switch it.Kind {
		case core.KindAlbum, core.KindRecording:
		case "":
			ds = append(ds, Diagnostic{SevError, 0, fmt.Sprintf("item %q has no kind (use '## <title> · album' or '· recording')", label)})
		default:
			ds = append(ds, Diagnostic{SevError, 0, fmt.Sprintf("item %q has unknown kind %q", label, it.Kind)})
		}
		it.Criteria = normalizeCriteria(it.Criteria, label, &ds)
		if it.Disposition != "" && it.Disposition != "drop" {
			ds = append(ds, Diagnostic{SevError, 0, fmt.Sprintf("item %q has unknown disposition %q (expected 'drop')", label, it.Disposition)})
		}
	}
	return ds
}

func normalizeCriteria(toks []string, where string, ds *[]Diagnostic) []string {
	out := make([]string, 0, len(toks))
	for _, t := range toks {
		t = strings.TrimSpace(t)
		if canon, ok := criterionSynonyms[strings.ToLower(t)]; ok {
			*ds = append(*ds, Diagnostic{SevWarning, 0, fmt.Sprintf("%s: criterion %q normalized to %q", where, t, canon)})
			t = canon
		}
		if strings.HasPrefix(strings.ToLower(t), performedByPrefix) {
			name := strings.TrimSpace(t[len(performedByPrefix):])
			if name == "" {
				*ds = append(*ds, Diagnostic{SevError, 0, fmt.Sprintf("%s: performed-by criterion needs a name", where)})
			}
			out = append(out, performedByPrefix+" "+name)
			continue
		}
		if !validCriteria[t] {
			*ds = append(*ds, Diagnostic{SevError, 0, fmt.Sprintf("%s: unknown criterion %q", where, t)})
		}
		out = append(out, t)
	}
	return out
}
