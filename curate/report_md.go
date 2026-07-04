package curate

import (
	"fmt"
	"strings"
)

// ReportMarkdown produces a human-readable markdown rendering of a curate report.
func ReportMarkdown(r Report) []byte {
	var b strings.Builder

	// Calculate statistics
	totalItems := len(r.Items)
	resolvedCount := 0
	admittedCount := 0
	absentCount := 0
	marginalCount := 0
	unverifiableCount := 0

	for _, item := range r.Items {
		if item.Disposition == "resolved" {
			resolvedCount++
			if item.Admitted {
				admittedCount++
			}
		} else if item.Disposition == "absent" {
			absentCount++
		}
		if item.Marginal != "" {
			marginalCount++
		}
		if len(item.Unverifiable) > 0 {
			unverifiableCount++
		}
	}

	// Header
	fmt.Fprintf(&b, "# Curate report: %s\n", r.Name)
	fmt.Fprintf(&b, "%d items: %d resolved (%d admitted), %d absent, %d marginal, %d unverifiable\n",
		totalItems, resolvedCount, admittedCount, absentCount, marginalCount, unverifiableCount)

	// Absent section
	absentItems := filterByDisposition(r.Items, "absent")
	if len(absentItems) > 0 {
		fmt.Fprintf(&b, "\n## Absent (%d)\n", len(absentItems))
		for _, item := range absentItems {
			detail := item.Marginal
			if detail == "" {
				detail = strings.Join(item.Violations, ", ")
			}
			formatLine(&b, item, detail)
		}
	}

	// Marginal section
	marginalItems := filterByMarginal(r.Items)
	if len(marginalItems) > 0 {
		fmt.Fprintf(&b, "\n## Marginal (%d)\n", len(marginalItems))
		for _, item := range marginalItems {
			formatLine(&b, item, item.Marginal)
		}
	}

	// Unverifiable section
	unverifiableItems := filterByUnverifiable(r.Items)
	if len(unverifiableItems) > 0 {
		fmt.Fprintf(&b, "\n## Unverifiable (%d)\n", len(unverifiableItems))
		for _, item := range unverifiableItems {
			detail := strings.Join(item.Unverifiable, ", ")
			formatLine(&b, item, detail)
		}
	}

	return append([]byte(b.String()), '\n')
}

// formatLine writes a single item line: `- <index> · <kind> [· <id>] · <note> — <detail>`
func formatLine(b *strings.Builder, item ReportItem, detail string) {
	fmt.Fprintf(b, "- %d · %s", item.Index, item.Kind)
	if item.ID != "" {
		fmt.Fprintf(b, " · %s", item.ID)
	}
	fmt.Fprintf(b, " · %s — %s\n", item.Note, detail)
}

// filterByDisposition returns items matching the given disposition.
func filterByDisposition(items []ReportItem, disp string) []ReportItem {
	var out []ReportItem
	for _, item := range items {
		if item.Disposition == disp {
			out = append(out, item)
		}
	}
	return out
}

// filterByMarginal returns items with a non-empty Marginal field.
func filterByMarginal(items []ReportItem) []ReportItem {
	var out []ReportItem
	for _, item := range items {
		if item.Marginal != "" {
			out = append(out, item)
		}
	}
	return out
}

// filterByUnverifiable returns items with a non-empty Unverifiable slice.
func filterByUnverifiable(items []ReportItem) []ReportItem {
	var out []ReportItem
	for _, item := range items {
		if len(item.Unverifiable) > 0 {
			out = append(out, item)
		}
	}
	return out
}
