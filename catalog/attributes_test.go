package catalog

import "testing"

// TestFindByAttributesSingleStyle checks a single-style predicate returns the
// styled master, with its artist resolved via masterArtistCredits and its
// full genre∪style list via masterStyles.
func TestFindByAttributesSingleStyle(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindByAttributes(AttributeQuery{Styles: []string{"Psychedelic Rock"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 candidate, got %d: %+v", len(got), got)
	}
	c := got[0]
	if c.DiscogsMasterID != 69017 {
		t.Errorf("DiscogsMasterID = %d, want 69017", c.DiscogsMasterID)
	}
	if c.Title != "John Barleycorn Must Die" {
		t.Errorf("Title = %q", c.Title)
	}
	if c.Artist != "Traffic" {
		t.Errorf("Artist = %q, want Traffic", c.Artist)
	}
	if c.Year != 1970 {
		t.Errorf("Year = %d, want 1970", c.Year)
	}
	var found bool
	for _, s := range c.Styles {
		if s == "Psychedelic Rock" {
			found = true
		}
	}
	if !found {
		t.Errorf("Styles = %v, want it to include Psychedelic Rock", c.Styles)
	}
}

// TestFindByAttributesANDAcrossStylesEmpty checks that requesting two styles
// held by no single master (AND, not OR) returns an empty, error-free result.
func TestFindByAttributesANDAcrossStylesEmpty(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindByAttributes(AttributeQuery{Styles: []string{"Psychedelic Rock", "Gothic Rock"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 candidates (no master has both styles), got %d: %+v", len(got), got)
	}
}

// TestFindByAttributesYearWindowFilters checks the year window narrows results
// to the master inside it and excludes masters outside it.
func TestFindByAttributesYearWindowFilters(t *testing.T) {
	m := newTestMirror(t)

	inWindow, err := m.FindByAttributes(AttributeQuery{Styles: []string{"Gothic Rock"}, YearFrom: 1980, YearTo: 1990})
	if err != nil {
		t.Fatal(err)
	}
	if len(inWindow) != 1 || inWindow[0].DiscogsMasterID != 70003 {
		t.Fatalf("expected master 70003 within 1980-1990, got %+v", inWindow)
	}

	outOfWindow, err := m.FindByAttributes(AttributeQuery{Styles: []string{"Gothic Rock"}, YearFrom: 1990})
	if err != nil {
		t.Fatal(err)
	}
	if len(outOfWindow) != 0 {
		t.Errorf("expected 0 candidates when YearFrom excludes 1984, got %+v", outOfWindow)
	}
}

// TestFindByAttributesCaseInsensitiveStyle checks the style filter matches
// regardless of case.
func TestFindByAttributesCaseInsensitiveStyle(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindByAttributes(AttributeQuery{Styles: []string{"psychedelic rock"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].DiscogsMasterID != 69017 {
		t.Fatalf("expected case-insensitive match on master 69017, got %+v", got)
	}
}

// TestFindByAttributesUnknownStyleEmptyNoError checks an unknown style yields
// an empty, error-free result (never an error for "no matches").
func TestFindByAttributesUnknownStyleEmptyNoError(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindByAttributes(AttributeQuery{Styles: []string{"Nonexistent Style"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 candidates, got %d: %+v", len(got), got)
	}
}

// TestFindByAttributesGenreOnlyDrivesGenreTable checks that a genre-only query
// (no styles) drives from dc.master_genre and still resolves the candidate.
func TestFindByAttributesGenreOnlyDrivesGenreTable(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindByAttributes(AttributeQuery{Genres: []string{"Electronic"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].DiscogsMasterID != 70003 {
		t.Fatalf("expected master 70003 for genre Electronic, got %+v", got)
	}
	if got[0].Artist != "Bleak Moon" {
		t.Errorf("Artist = %q, want Bleak Moon", got[0].Artist)
	}
}

// TestFindByAttributesMatchSignals checks the match{} object: styles_matched
// counts every requested style+genre term, and year_match is present (true)
// only when a year window was given.
func TestFindByAttributesMatchSignals(t *testing.T) {
	m := newTestMirror(t)

	noWindow, err := m.FindByAttributes(AttributeQuery{Styles: []string{"Psychedelic Rock"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(noWindow) != 1 {
		t.Fatalf("expected 1 candidate, got %+v", noWindow)
	}
	if sm, _ := noWindow[0].Match["styles_matched"].(int); sm != 1 {
		t.Errorf("styles_matched = %v, want 1", noWindow[0].Match["styles_matched"])
	}
	if _, ok := noWindow[0].Match["year_match"]; ok {
		t.Errorf("year_match should be absent when no window given; got %v", noWindow[0].Match["year_match"])
	}

	withWindow, err := m.FindByAttributes(AttributeQuery{Styles: []string{"Gothic Rock"}, Genres: []string{"Electronic"}, YearFrom: 1980, YearTo: 1990})
	if err != nil {
		t.Fatal(err)
	}
	if len(withWindow) != 1 {
		t.Fatalf("expected 1 candidate, got %+v", withWindow)
	}
	if sm, _ := withWindow[0].Match["styles_matched"].(int); sm != 2 {
		t.Errorf("styles_matched = %v, want 2 (1 style + 1 genre)", withWindow[0].Match["styles_matched"])
	}
	if ym, ok := withWindow[0].Match["year_match"].(bool); !ok || !ym {
		t.Errorf("year_match = %v, want true", withWindow[0].Match["year_match"])
	}
}

// TestFindByAttributesNoPredicatesErrors checks that a query with neither
// styles nor genres is rejected (mirrors the CLI's --style/--genre requirement).
func TestFindByAttributesNoPredicatesErrors(t *testing.T) {
	m := newTestMirror(t)
	if _, err := m.FindByAttributes(AttributeQuery{}); err == nil {
		t.Error("expected an error when neither styles nor genres are given")
	}
}

// TestFindByAttributesDefaultLimit checks the default Limit (0) behaves as 25,
// not zero rows.
func TestFindByAttributesDefaultLimit(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindByAttributes(AttributeQuery{Genres: []string{"Classical"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Error("expected at least one candidate with the default limit")
	}
}
