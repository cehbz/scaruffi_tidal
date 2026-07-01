package catalog

import (
	"database/sql"

	"github.com/cehbz/tidalist/core"
)

// WorkCandidate is a ranked MB work resolution result.
type WorkCandidate struct {
	MBID      core.MBID `json:"mbid,omitempty"`
	Name      string    `json:"name"`
	Composers []string  `json:"composers,omitempty"`
	Match     Match     `json:"match"`
}

// ResolveWork returns ranked work candidates (FTS over work_fts) with composers.
func (m *MirrorDB) ResolveWork(name string, limit int) ([]WorkCandidate, error) {
	rows, err := m.DB.Query(
		`SELECT w.id, w.gid, w.name, f.rank
		   FROM work_fts f
		   JOIN work w ON w.id = f.rowid
		  WHERE work_fts MATCH ?
		  ORDER BY f.rank
		  LIMIT ?`, ftsTitle(name), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WorkCandidate
	for rows.Next() {
		var id int64
		var gid, nm string
		var rank float64
		if err := rows.Scan(&id, &gid, &nm, &rank); err != nil {
			return nil, err
		}
		r := rank
		composers, err := m.workComposers(id)
		if err != nil {
			return nil, err
		}
		out = append(out, WorkCandidate{MBID: core.MBID(gid), Name: nm, Composers: composers, Match: Match{FTSRank: &r}})
	}
	return out, rows.Err()
}

// WorkGroup is a resolved composition: a parent work plus its movement sub-works
// (l_work_work 281 parts), disambiguated by composer.
type WorkGroup struct {
	RootID    int64
	RootMBID  core.MBID
	RootName  string
	Composers []string
	WorkIDs   []int64
}

// workGroupLinkParts is l_work_work link_type 281 (parent "has parts" movements).
// 314 based-on / 350 arrangement / 239 medley / 315 revision-of are NOT parts and
// are excluded by matching only 281.
const workGroupLinkParts = 281

func (m *MirrorDB) resolveWorkGroup(title, composer string) (WorkGroup, bool, error) {
	// (a) candidate works by title FTS.
	rows, err := m.DB.Query(
		`SELECT rowid FROM work_fts WHERE work_fts MATCH ? ORDER BY rank LIMIT 25`, ftsTitle(title))
	if err != nil {
		return WorkGroup{}, false, err
	}
	var candIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return WorkGroup{}, false, err
		}
		candIDs = append(candIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return WorkGroup{}, false, err
	}

	// (b) keep candidates whose composer matches (skip the filter when none requested).
	var matched []int64
	for _, id := range candIDs {
		if composer == "" {
			matched = append(matched, id)
			continue
		}
		names, err := m.workComposers(id)
		if err != nil {
			return WorkGroup{}, false, err
		}
		cs := make(core.Credits, 0, len(names))
		for _, n := range names {
			cs = append(cs, core.Credit{Role: core.RoleComposer, Name: n})
		}
		if cs.MatchesRole(core.RoleComposer, composer) {
			matched = append(matched, id)
		}
	}
	if len(matched) == 0 {
		return WorkGroup{}, false, nil
	}

	// (c) resolve the group root. Prefer a matched candidate that resolves to a work
	// with 281 children (a real work-group) over an arc-less title-twin stub, so FTS
	// rank ties between a stub and the true parent can't collapse the group. A genuine
	// standalone work (no parent, no children) falls back to matched[0].
	root := matched[0]
	for _, cand := range matched {
		r := cand
		var parent int64
		perr := m.DB.QueryRow(
			`SELECT lww.entity0 FROM l_work_work lww
			   JOIN link l ON l.id = lww.link
			  WHERE lww.entity1 = ? AND l.link_type = ?
			  ORDER BY lww.entity0 LIMIT 1`, cand, workGroupLinkParts).Scan(&parent)
		if perr == nil {
			r = parent
		} else if perr != sql.ErrNoRows {
			return WorkGroup{}, false, perr
		}
		var childCount int
		if err := m.DB.QueryRow(
			`SELECT COUNT(*) FROM l_work_work lww
			   JOIN link l ON l.id = lww.link
			  WHERE lww.entity0 = ? AND l.link_type = ?`, r, workGroupLinkParts).Scan(&childCount); err != nil {
			return WorkGroup{}, false, err
		}
		if childCount > 0 {
			root = r
			break
		}
	}

	// (d) the group = root ∪ its 281 children.
	ids := []int64{root}
	childRows, err := m.DB.Query(
		`SELECT lww.entity1 FROM l_work_work lww
		   JOIN link l ON l.id = lww.link
		  WHERE lww.entity0 = ? AND l.link_type = ?
		  ORDER BY lww.link_order, lww.entity1`, root, workGroupLinkParts)
	if err != nil {
		return WorkGroup{}, false, err
	}
	for childRows.Next() {
		var id int64
		if err := childRows.Scan(&id); err != nil {
			childRows.Close()
			return WorkGroup{}, false, err
		}
		ids = append(ids, id)
	}
	childRows.Close()
	if err := childRows.Err(); err != nil {
		return WorkGroup{}, false, err
	}

	var g WorkGroup
	g.RootID = root
	g.WorkIDs = ids
	if err := m.DB.QueryRow(`SELECT gid, name FROM work WHERE id = ?`, root).
		Scan((*string)(&g.RootMBID), &g.RootName); err != nil {
		return WorkGroup{}, false, err
	}
	if g.Composers, err = m.workComposers(root); err != nil {
		return WorkGroup{}, false, err
	}
	return g, true, nil
}

// workComposers returns the work's composer names (l_artist_work, link_type 168).
func (m *MirrorDB) workComposers(workID int64) ([]string, error) {
	rows, err := m.DB.Query(
		`SELECT a.name
		   FROM l_artist_work law
		   JOIN link l ON l.id = law.link
		   JOIN artist a ON a.id = law.entity0
		  WHERE law.entity1 = ? AND l.link_type = 168
		  ORDER BY law.link_order`, workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
