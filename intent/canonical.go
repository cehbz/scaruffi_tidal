package intent

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cehbz/tidalist/core"
)

// Canonical re-emits d in canonical form: credits sorted by role precedence
// (stable within a role), then the fixed bullet order. The output round-trips
// through Parse to the same bytes.
func Canonical(d Doc) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n", strings.TrimSpace(d.Name))
	if len(d.Brief) > 0 {
		fmt.Fprintf(&b, "Brief: %s\n", strings.Join(d.Brief, "; "))
	}
	for _, it := range d.Items {
		fmt.Fprintf(&b, "\n## %s · %s\n", strings.TrimSpace(it.Title), it.Kind)
		for _, c := range sortedCredits(it.Credits) {
			fmt.Fprintf(&b, "- %s: %s\n", c.Role, creditSurface(c))
		}
		if it.Work != "" {
			fmt.Fprintf(&b, "- work: %s\n", it.Work)
		}
		if it.Year != 0 {
			fmt.Fprintf(&b, "- year: %d\n", it.Year)
		}
		if len(it.Criteria) > 0 {
			fmt.Fprintf(&b, "- criteria: %s\n", strings.Join(it.Criteria, "; "))
		}
		if len(it.Edition) > 0 {
			fmt.Fprintf(&b, "- edition: %s\n", strings.Join(it.Edition, ", "))
		}
		if len(it.Rendering) > 0 {
			fmt.Fprintf(&b, "- rendering: %s\n", strings.Join(it.Rendering, ", "))
		}
		if it.MBID != "" {
			fmt.Fprintf(&b, "- mbid: %s\n", it.MBID)
		}
		if it.DiscogsMaster != "" {
			fmt.Fprintf(&b, "- discogs-master: %s\n", it.DiscogsMaster)
		}
		if it.ISRC != "" {
			fmt.Fprintf(&b, "- isrc: %s\n", it.ISRC)
		}
		if it.Disposition != "" {
			fmt.Fprintf(&b, "- disposition: %s\n", it.Disposition)
		}
		for _, n := range it.Notes {
			fmt.Fprintf(&b, "- note: %s\n", n)
		}
	}
	return []byte(b.String())
}

func sortedCredits(cs core.Credits) core.Credits {
	out := make(core.Credits, len(cs))
	copy(out, cs)
	rank := map[core.Role]int{}
	for i, r := range core.Roles() {
		rank[r] = i
	}
	sort.SliceStable(out, func(i, j int) bool { return rank[out[i].Role] < rank[out[j].Role] })
	return out
}

// creditSurface renders a credit's name + parenthetical attrs. A single
// "instrument" attribute uses the bare shorthand "(value)"; anything else uses
// explicit "(k=v, k=v)" with keys sorted for determinism.
func creditSurface(c core.Credit) string {
	if len(c.Attrs) == 0 {
		return c.Name
	}
	if len(c.Attrs) == 1 {
		if v, ok := c.Attrs["instrument"]; ok {
			return fmt.Sprintf("%s (%s)", c.Name, v)
		}
	}
	keys := make([]string, 0, len(c.Attrs))
	for k := range c.Attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + c.Attrs[k]
	}
	return fmt.Sprintf("%s (%s)", c.Name, strings.Join(parts, ", "))
}

// Summary returns a one-line coverage report: item count, per-kind counts, total
// credits, and credits broken down by role (in precedence order).
func Summary(d Doc) string {
	var albums, recordings, credits int
	byRole := map[core.Role]int{}
	for _, it := range d.Items {
		switch it.Kind {
		case core.KindAlbum:
			albums++
		case core.KindRecording:
			recordings++
		}
		for _, c := range it.Credits {
			byRole[c.Role]++
			credits++
		}
	}
	var roleParts []string
	for _, r := range core.Roles() {
		if n := byRole[r]; n > 0 {
			roleParts = append(roleParts, fmt.Sprintf("%s=%d", r, n))
		}
	}
	rolesStr := ""
	if len(roleParts) > 0 {
		rolesStr = " [" + strings.Join(roleParts, ", ") + "]"
	}
	return fmt.Sprintf("%d items: %d album, %d recording; %d credits%s",
		len(d.Items), albums, recordings, credits, rolesStr)
}
