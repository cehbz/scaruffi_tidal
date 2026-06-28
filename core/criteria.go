package core

import "fmt"

// Verdict is the outcome of judging a GoldenItem against a set of criteria.
type Verdict struct {
	Admitted   bool     `json:"admitted"`
	Violations []string `json:"violations,omitempty"`
}

// Criterion is a type-aware admission rule (the Specification pattern). Violation
// returns the reason the item fails, or "" if it passes. A criterion that does not
// apply to the item's grain returns "" (a no-op), so recording rules pass albums
// and vice-versa.
type Criterion interface {
	Violation(GoldenItem) string
}

// NoCompilation rejects a compilation album. No-op on recordings.
type NoCompilation struct{}

// NoLive rejects a live album. No-op on recordings.
type NoLive struct{}

// Violation reports "compilation" when the item is a compilation album.
func (NoCompilation) Violation(item GoldenItem) string {
	if a, ok := item.(Album); ok && a.HasTrait(TraitCompilation) {
		return "compilation"
	}
	return ""
}

// Violation reports "live album" when the item is a live album.
func (NoLive) Violation(item GoldenItem) string {
	if a, ok := item.(Album); ok && a.HasTrait(TraitLive) {
		return "live album"
	}
	return ""
}

// Judge collects the non-empty violations of each criterion; the item is admitted
// only when there are none.
func Judge(item GoldenItem, criteria []Criterion) Verdict {
	var violations []string
	for _, c := range criteria {
		if v := c.Violation(item); v != "" {
			violations = append(violations, v)
		}
	}
	return Verdict{Admitted: len(violations) == 0, Violations: violations}
}

// Studio rejects a live recording. No-op on albums. Only an explicit live take
// fails; studio and unknown pass.
type Studio struct{}

// PerformedBy requires Name to be among the recording's performing credits (so:
// not a cover). No-op on albums.
type PerformedBy struct {
	Name string
}

// Violation reports "live recording" when the item is a live take.
func (Studio) Violation(item GoldenItem) string {
	if r, ok := item.(Recording); ok && r.Performance == PerfLive {
		return "live recording"
	}
	return ""
}

// Violation reports a cover when Name is not among the recording's performing credits.
func (c PerformedBy) Violation(item GoldenItem) string {
	r, ok := item.(Recording)
	if !ok {
		return ""
	}
	if r.Credits.Performs(c.Name) {
		return ""
	}
	return fmt.Sprintf("%s not in performer credits (likely a cover)", c.Name)
}
