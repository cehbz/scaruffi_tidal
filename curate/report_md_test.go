package curate

import (
	"strings"
	"testing"
)

// TestReportMarkdown pins the full document: header, count line, all three
// sections in order, item line format, and blank-line placement.
func TestReportMarkdown(t *testing.T) {
	r := Report{
		Name: "Test Report",
		Items: []ReportItem{
			// Clean resolved: admitted, no marginal, no unverifiable — no section.
			{Index: 0, Kind: "album", ID: "abc123", Note: "clean pick",
				Disposition: "resolved", Admitted: true},
			// Absent (violations only; no ID segment).
			{Index: 1, Kind: "album", Note: "missing",
				Disposition: "absent", Violations: []string{"no album found"}},
			// Marginal.
			{Index: 2, Kind: "track", ID: "rec123", Note: "one candidate",
				Disposition: "resolved", Admitted: true,
				Marginal: "only one option was available"},
			// Unverifiable.
			{Index: 3, Kind: "album", ID: "8ec7", Note: "live performance",
				Disposition: "resolved", Admitted: true,
				Unverifiable: []string{"criterion unverifiable: not_live"}},
		},
	}

	want := `# Curate report: Test Report
4 items: 3 resolved (3 admitted), 1 absent, 1 marginal, 1 unverifiable

## Absent (1)
- 1 · album · missing — no album found

## Marginal (1)
- 2 · track · rec123 · one candidate — only one option was available

## Unverifiable (1)
- 3 · album · 8ec7 · live performance — criterion unverifiable: not_live

`
	if got := string(ReportMarkdown(r)); got != want {
		t.Errorf("ReportMarkdown mismatch\ngot:\n%q\nwant:\n%q", got, want)
	}
}

// TestReportMarkdownOmitsEmptySections: no absent items → no Absent section.
func TestReportMarkdownOmitsEmptySections(t *testing.T) {
	r := Report{
		Name: "No Absent",
		Items: []ReportItem{
			{Index: 0, Kind: "album", ID: "abc", Note: "clean",
				Disposition: "resolved", Admitted: true},
			{Index: 1, Kind: "track", ID: "rec1", Note: "iffy",
				Disposition: "resolved", Admitted: true, Marginal: "close call"},
		},
	}
	got := string(ReportMarkdown(r))
	if strings.Contains(got, "## Absent") {
		t.Errorf("Absent section should be omitted:\n%s", got)
	}
	if !strings.Contains(got, "## Marginal") {
		t.Errorf("Marginal section missing:\n%s", got)
	}
}

// TestReportMarkdownAbsentWithMarginal: an absent item with BOTH Marginal and
// Violations uses the Marginal text as its Absent-section detail, and also
// appears in the Marginal section.
func TestReportMarkdownAbsentWithMarginal(t *testing.T) {
	r := Report{
		Name: "Both",
		Items: []ReportItem{
			{Index: 0, Kind: "album", Note: "x",
				Disposition: "absent",
				Violations:  []string{"no album found"},
				Marginal:    "marginal note"},
		},
	}

	want := `# Curate report: Both
1 items: 0 resolved (0 admitted), 1 absent, 1 marginal, 0 unverifiable

## Absent (1)
- 0 · album · x — marginal note

## Marginal (1)
- 0 · album · x — marginal note

`
	if got := string(ReportMarkdown(r)); got != want {
		t.Errorf("ReportMarkdown mismatch\ngot:\n%q\nwant:\n%q", got, want)
	}
}

// TestReportMarkdownZeroItems: exact output — header and zeroed count line only.
func TestReportMarkdownZeroItems(t *testing.T) {
	r := Report{Name: "Empty"}
	want := "# Curate report: Empty\n" +
		"0 items: 0 resolved (0 admitted), 0 absent, 0 marginal, 0 unverifiable\n\n"
	if got := string(ReportMarkdown(r)); got != want {
		t.Errorf("ReportMarkdown mismatch\ngot:\n%q\nwant:\n%q", got, want)
	}
}
