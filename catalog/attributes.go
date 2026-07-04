package catalog

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/cehbz/tidalist/core"
)

// defaultAttributeLimit is applied when AttributeQuery.Limit is unset (<=0).
const defaultAttributeLimit = 25

// AttributeQuery describes a Discogs style/genre/year descriptor predicate:
// an agent-driven album-candidate generator that doesn't rely on model recall
// (e.g. "gothic rock, 1979-1986"). Styles and Genres are AND-ed together —
// every requested term must match the same master — and matched
// case-insensitively against dc.master_style / dc.master_genre.
type AttributeQuery struct {
	Styles   []string // AND across all; matched case-insensitively against dc.master_style
	Genres   []string // AND; dc.master_genre
	YearFrom int
	YearTo   int
	Limit    int // default 25
}

// AttributeCandidate is a Discogs master matched by an AttributeQuery.
type AttributeCandidate struct {
	DiscogsMasterID int64          `json:"discogs_master_id"`
	Title           string         `json:"title"`
	Artist          string         `json:"artist"`
	Year            int            `json:"year,omitempty"`
	Styles          []string       `json:"styles,omitempty"`
	Match           map[string]any `json:"match"`
}

// FindByAttributes returns Discogs masters matching every requested style and
// genre term (AND) within an optional year window, ordered chronologically
// (m.year, m.id) for determinism.
//
// PLANNER SAFETY: the query always drives off the smallest matching attribute
// table — dc.master_style when any style is requested (styles are the more
// selective attribute in practice), else dc.master_genre — with one row
// lookup into dc.master per driving row and one EXISTS arm per additional
// style/genre term. It must never scan dc.master itself; see the EXPLAIN
// QUERY PLAN check recorded in the task report.
//
// Duration is deliberately not a filter here: the agent composes
// find-by-attributes -> tracklist --master <id> and filters durations itself.
func (m *MirrorDB) FindByAttributes(q AttributeQuery) ([]AttributeCandidate, error) {
	if len(q.Styles) == 0 && len(q.Genres) == 0 {
		return nil, fmt.Errorf("at least one style or genre is required")
	}
	limit := q.Limit
	if limit <= 0 {
		limit = defaultAttributeLimit
	}

	// Drive off styles when any are given (the more selective attribute);
	// otherwise drive off genres. Every OTHER requested term (including every
	// remaining style/genre) becomes an EXISTS arm keyed on the driving row's
	// master_id, so the plan is always: scan the driving table, look up
	// dc.master by id, then confirm each extra term with an indexed EXISTS.
	var driveTable, driveCol, driveTerm string
	var extraStyles, extraGenres []string
	if len(q.Styles) > 0 {
		driveTable, driveCol = "master_style", "style"
		driveTerm = q.Styles[0]
		extraStyles = q.Styles[1:]
		extraGenres = q.Genres
	} else {
		driveTable, driveCol = "master_genre", "genre"
		driveTerm = q.Genres[0]
		extraGenres = q.Genres[1:]
	}

	var sb strings.Builder
	args := make([]any, 0, 4+len(extraStyles)+len(extraGenres))
	fmt.Fprintf(&sb, `SELECT m.id, m.title, m.year
		FROM dc.%s d
		JOIN dc.master m ON m.id = d.master_id
		WHERE d.%s = ? COLLATE NOCASE`, driveTable, driveCol)
	args = append(args, driveTerm)

	if q.YearFrom != 0 {
		sb.WriteString(` AND m.year >= ?`)
		args = append(args, q.YearFrom)
	}
	if q.YearTo != 0 {
		sb.WriteString(` AND m.year <= ?`)
		args = append(args, q.YearTo)
	}
	for _, s := range extraStyles {
		sb.WriteString(` AND EXISTS (SELECT 1 FROM dc.master_style ms2 WHERE ms2.master_id = m.id AND ms2.style = ? COLLATE NOCASE)`)
		args = append(args, s)
	}
	for _, g := range extraGenres {
		sb.WriteString(` AND EXISTS (SELECT 1 FROM dc.master_genre mg2 WHERE mg2.master_id = m.id AND mg2.genre = ? COLLATE NOCASE)`)
		args = append(args, g)
	}
	// Nulls-last: dc.master.year can be NULL in the real mirror (unlike this
	// package's other year columns, which are always populated or absent
	// entirely); a plain "ORDER BY m.year" would sort those rows FIRST under
	// SQLite's default ASC null ordering, which is wrong for a chronological
	// "canonical X" reading.
	sb.WriteString(` ORDER BY (m.year IS NULL), m.year, m.id LIMIT ?`)
	args = append(args, limit)

	rows, err := m.DB.Query(sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	yearMatch := q.YearFrom != 0 || q.YearTo != 0
	stylesMatched := len(q.Styles) + len(q.Genres)

	var out []AttributeCandidate
	for rows.Next() {
		var id int64
		var title string
		var year sql.NullInt64
		if err := rows.Scan(&id, &title, &year); err != nil {
			return nil, err
		}
		credits, err := m.masterArtistCredits(id)
		if err != nil {
			return nil, err
		}
		styles, err := m.masterStyles(id)
		if err != nil {
			return nil, err
		}
		c := AttributeCandidate{
			DiscogsMasterID: id,
			Title:           title,
			Artist:          strings.Join(credits.Names(core.RoleArtist), ", "),
			Styles:          styles,
			Match:           map[string]any{"styles_matched": stylesMatched},
		}
		if year.Valid {
			c.Year = int(year.Int64)
		}
		if yearMatch {
			c.Match["year_match"] = true
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
