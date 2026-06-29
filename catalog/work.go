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

func (m *MirrorDB) resolveWorkID(name string) (int64, bool, error) {
	var id int64
	err := m.DB.QueryRow(
		`SELECT rowid FROM work_fts WHERE work_fts MATCH ? ORDER BY rank LIMIT 1`,
		ftsTitle(name)).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
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
