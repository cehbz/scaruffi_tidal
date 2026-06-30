// Package intent models tidalist's intent markdown — the role-tagged playlist
// description the interpret stage produces and the curate stage consumes. It
// provides Parse (markdown → Doc), Validate (vocabulary + required-field checks),
// and Canonical (deterministic re-emit). The package is pure: stdlib + core only.
package intent

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cehbz/tidalist/core"
)

// Doc is a parsed intent document.
type Doc struct {
	Name  string   // playlist name (the "# " heading)
	Brief []string // playlist-wide criteria tokens
	Items []Item
}

// Item is one playlist entry.
type Item struct {
	Title         string
	Kind          core.Kind
	Credits       core.Credits
	Work          string
	Year          int
	Criteria      []string
	Edition       []string
	Rendering     []string
	MBID          string
	DiscogsMaster string
	ISRC          string
	Disposition   string
	Notes         []string
}

// Severity classifies a Diagnostic.
type Severity int

const (
	SevError Severity = iota
	SevWarning
)

func (s Severity) String() string {
	if s == SevWarning {
		return "warning"
	}
	return "error"
}

// Diagnostic is one problem found during Parse or Validate.
type Diagnostic struct {
	Severity Severity
	Line     int // 1-based source line; 0 when not tied to a line
	Msg      string
}

func (d Diagnostic) String() string {
	if d.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", d.Severity, d.Line, d.Msg)
	}
	return fmt.Sprintf("%s: %s", d.Severity, d.Msg)
}

// HasError reports whether any diagnostic is error-level.
func HasError(ds []Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == SevError {
			return true
		}
	}
	return false
}

// Parse turns intent markdown into a Doc. It is lenient: structural problems are
// recorded as Diagnostics rather than aborting, so the caller reports them all.
func Parse(src []byte) (Doc, []Diagnostic) {
	var doc Doc
	var ds []Diagnostic
	curIdx := -1

	for i, raw := range strings.Split(string(src), "\n") {
		ln := i + 1
		t := strings.TrimSpace(raw)
		switch {
		case t == "":
			// blank line: ignore
		case strings.HasPrefix(t, "## "):
			doc.Items = append(doc.Items, parseHeading(strings.TrimSpace(t[3:])))
			curIdx = len(doc.Items) - 1
		case strings.HasPrefix(t, "# "):
			doc.Name = strings.TrimSpace(t[2:])
		case strings.HasPrefix(t, "Brief:"):
			doc.Brief = append(doc.Brief, splitTokens(t[len("Brief:"):])...)
		case strings.HasPrefix(t, "- "):
			if curIdx < 0 {
				ds = append(ds, Diagnostic{SevError, ln, "bullet before any '## ' item heading"})
				continue
			}
			parseBullet(&doc.Items[curIdx], strings.TrimSpace(t[2:]), ln, &ds)
		default:
			ds = append(ds, Diagnostic{SevWarning, ln, fmt.Sprintf("ignored unrecognized line %q", t)})
		}
	}
	return doc, ds
}

// parseHeading splits "<title> · <kind>" (middle dot U+00B7). A missing dot
// leaves Kind empty for Validate to flag.
func parseHeading(s string) Item {
	title, kind, ok := strings.Cut(s, "·")
	it := Item{Title: strings.TrimSpace(title)}
	if ok {
		it.Kind = core.Kind(strings.TrimSpace(kind))
	}
	return it
}

func parseBullet(it *Item, s string, ln int, ds *[]Diagnostic) {
	key, val, ok := strings.Cut(s, ":")
	if !ok {
		*ds = append(*ds, Diagnostic{SevError, ln, fmt.Sprintf("bullet must be 'key: value', got %q", s)})
		return
	}
	key = strings.ToLower(strings.TrimSpace(key))
	val = strings.TrimSpace(val)
	switch key {
	case "work":
		it.Work = val
	case "year":
		y, err := strconv.Atoi(val)
		if err != nil {
			*ds = append(*ds, Diagnostic{SevError, ln, fmt.Sprintf("year must be an integer, got %q", val)})
			return
		}
		it.Year = y
	case "criteria":
		it.Criteria = append(it.Criteria, splitTokens(val)...)
	case "edition":
		it.Edition = append(it.Edition, splitList(val)...)
	case "rendering":
		it.Rendering = append(it.Rendering, splitList(val)...)
	case "mbid":
		it.MBID = val
	case "discogs-master":
		it.DiscogsMaster = val
	case "isrc":
		it.ISRC = val
	case "disposition":
		it.Disposition = val
	case "note":
		it.Notes = append(it.Notes, val)
	default:
		r := core.Role(key)
		if !core.ValidRole(r) {
			*ds = append(*ds, Diagnostic{SevError, ln, fmt.Sprintf("unknown field or role %q", key)})
			return
		}
		it.Credits = append(it.Credits, parseCreditValue(r, val))
	}
}

// parseCreditValue extracts a trailing "(...)" attribute group. A bare value
// (no '=') is the role's positional attribute, which for soloist is the
// instrument/voice — so "Emma Kirkby (soprano)" → {instrument: soprano}.
func parseCreditValue(role core.Role, val string) core.Credit {
	c := core.Credit{Role: role, Name: val}
	if i := strings.LastIndex(val, "("); i >= 0 && strings.HasSuffix(val, ")") {
		inside := strings.TrimSpace(val[i+1 : len(val)-1])
		if inside != "" {
			if attrs := parseAttrSurface(inside); attrs != nil {
				c.Name = strings.TrimSpace(val[:i])
				c.Attrs = attrs
			}
		}
	}
	return c
}

func parseAttrSurface(inside string) map[string]string {
	attrs := map[string]string{}
	for _, p := range strings.Split(inside, ",") {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		if k, v, ok := strings.Cut(p, "="); ok {
			attrs[strings.TrimSpace(k)] = strings.TrimSpace(v)
		} else {
			attrs["instrument"] = p
		}
	}
	if len(attrs) == 0 {
		return nil
	}
	return attrs
}

// splitTokens splits a ';'-separated list (criteria / Brief), trimming and
// dropping empties.
func splitTokens(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ";") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// splitList splits a ','-separated list (edition / rendering markers).
func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
