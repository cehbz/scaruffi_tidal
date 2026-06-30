package main

import (
	"fmt"
	"strings"

	"github.com/cehbz/tidalist/core"
)

// parseCredit parses "role:name[:k=v,k=v]". Role is split at the first colon; the
// remainder is the name unless a trailing colon-segment looks like attrs (k=v…).
func parseCredit(spec string) (core.Credit, error) {
	role, rest, ok := strings.Cut(spec, ":")
	if !ok || role == "" || rest == "" {
		return core.Credit{}, fmt.Errorf("credit must be role:name, got %q", spec)
	}
	r := core.Role(role)
	if !core.ValidRole(r) {
		return core.Credit{}, fmt.Errorf("unknown credit role %q", role)
	}
	name := rest
	var attrs map[string]string
	if i := strings.LastIndex(rest, ":"); i >= 0 {
		if a, ok := parseAttrs(rest[i+1:]); ok {
			name = rest[:i]
			attrs = a
		}
	}
	return core.Credit{Role: r, Name: name, Attrs: attrs}, nil
}

// parseAttrs parses "k=v,k=v" into a map; ok=false if any token is not k=v.
func parseAttrs(s string) (map[string]string, bool) {
	if s == "" {
		return nil, false
	}
	out := map[string]string{}
	for _, kv := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return nil, false
		}
		out[k] = v
	}
	return out, true
}

func parseCredits(specs []string) (core.Credits, error) {
	var cs core.Credits
	for _, s := range specs {
		c, err := parseCredit(s)
		if err != nil {
			return nil, err
		}
		cs = append(cs, c)
	}
	return cs, nil
}
