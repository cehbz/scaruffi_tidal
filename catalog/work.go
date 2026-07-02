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

	// (b) keep candidates whose composer matches (skip the filter when none
	// requested). The requested name is expanded to its alias set, so a Latin
	// query matches a Cyrillic-named composer.
	var composerNames []string
	if composer != "" {
		var err error
		if composerNames, err = m.nameVariants(composer); err != nil {
			return WorkGroup{}, false, err
		}
	}
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
		if composerAmong(names, composerNames) {
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

	return m.groupByRoot(root)
}

// workGroupMaxDepth caps the 281 descent/ascent: MB movement hierarchies are
// recursive (work → part → movement), rarely deeper than three levels.
const workGroupMaxDepth = 4

// groupByRoot assembles a WorkGroup (root ∪ its TRANSITIVE 281 descendants —
// parts have movements, so one level misses the recordings) from a root work id.
func (m *MirrorDB) groupByRoot(root int64) (WorkGroup, bool, error) {
	ids := []int64{root}
	seen := map[int64]bool{root: true}
	frontier := []int64{root}
	for depth := 0; depth < workGroupMaxDepth && len(frontier) > 0; depth++ {
		inClause, args := intInClause(frontier)
		childRows, err := m.DB.Query(
			`SELECT lww.entity1 FROM l_work_work lww
			   JOIN link l ON l.id = lww.link
			  WHERE lww.entity0 IN (`+inClause+`) AND l.link_type = ?
			  ORDER BY lww.link_order, lww.entity1`,
			append(args, workGroupLinkParts)...)
		if err != nil {
			return WorkGroup{}, false, err
		}
		var next []int64
		for childRows.Next() {
			var id int64
			if err := childRows.Scan(&id); err != nil {
				childRows.Close()
				return WorkGroup{}, false, err
			}
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
				next = append(next, id)
			}
		}
		childRows.Close()
		if err := childRows.Err(); err != nil {
			return WorkGroup{}, false, err
		}
		frontier = next
	}

	var g WorkGroup
	g.RootID = root
	g.WorkIDs = ids
	if err := m.DB.QueryRow(`SELECT gid, name FROM work WHERE id = ?`, root).
		Scan((*string)(&g.RootMBID), &g.RootName); err != nil {
		return WorkGroup{}, false, err
	}
	composers, err := m.workComposers(root)
	if err != nil {
		return WorkGroup{}, false, err
	}
	g.Composers = composers
	return g, true, nil
}

// workGroupFromPerformers rediscovers the work-group from a constraint performer's
// recordings: their 278-linked works, climbed to 281 parents, filtered by composer
// and work-title tokens, best-first by recording-link mass. This is the escape from
// the title-twin family trap (MB holds unmerged same-composition families under
// different-language names; title FTS can only ever see the queried language's
// family, which may hold none of the performer's recordings).
func (m *MirrorDB) workGroupFromPerformers(q PerformanceQuery, composer string) (WorkGroup, bool, error) {
	workTokens := significantWorkTokens(q.Work)
	var composerVariants []string
	if composer != "" {
		var err error
		if composerVariants, err = m.nameVariants(composer); err != nil {
			return WorkGroup{}, false, err
		}
	}
	for _, w := range performerCredits(q.Credits) {
		aid, ok, err := m.resolveArtistID(w.Name)
		if err != nil {
			return WorkGroup{}, false, err
		}
		if !ok {
			continue
		}
		// Candidate roots: the performer's recordings → works (278) → 281 parent
		// (or the work itself when parentless), heaviest recording mass first.
		rows, err := m.DB.Query(
			`WITH precs AS (
			   SELECT rec.id FROM recording rec
			     JOIN artist_credit_name acn ON acn.artist_credit = rec.artist_credit
			    WHERE acn.artist = ?1
			   UNION
			   SELECT lar.entity1 FROM l_artist_recording lar WHERE lar.entity0 = ?1
			 )
			 SELECT COALESCE(pw.parent, lrw.entity1) AS root, COUNT(*) AS n
			   FROM precs
			   JOIN l_recording_work lrw ON lrw.entity0 = precs.id
			   JOIN link l ON l.id = lrw.link AND l.link_type = ?2
			   LEFT JOIN (SELECT lww.entity1 AS child, lww.entity0 AS parent
			                FROM l_work_work lww
			                JOIN link lk ON lk.id = lww.link AND lk.link_type = ?3) pw ON pw.child = lrw.entity1
			  GROUP BY root ORDER BY n DESC LIMIT 50`, aid, linkTypePerformance, workGroupLinkParts)
		if err != nil {
			return WorkGroup{}, false, err
		}
		var roots []int64
		for rows.Next() {
			var root, n int64
			if err := rows.Scan(&root, &n); err != nil {
				rows.Close()
				return WorkGroup{}, false, err
			}
			roots = append(roots, root)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return WorkGroup{}, false, err
		}
		for _, root := range roots {
			// Climb transitively to the TOP ancestor (part → work): the one-level
			// COALESCE in the query only reaches a movement's immediate parent.
			for depth := 0; depth < workGroupMaxDepth; depth++ {
				var parent int64
				err := m.DB.QueryRow(
					`SELECT lww.entity0 FROM l_work_work lww
					   JOIN link l ON l.id = lww.link
					  WHERE lww.entity1 = ? AND l.link_type = ?
					  ORDER BY lww.entity0 LIMIT 1`, root, workGroupLinkParts).Scan(&parent)
				if err == sql.ErrNoRows {
					break
				}
				if err != nil {
					return WorkGroup{}, false, err
				}
				root = parent
			}
			var name string
			if err := m.DB.QueryRow(`SELECT name FROM work WHERE id = ?`, root).Scan(&name); err == sql.ErrNoRows {
				continue
			} else if err != nil {
				return WorkGroup{}, false, err
			}
			if !albumMatchesWork(workTokens, name) {
				continue
			}
			if composer != "" {
				names, err := m.workComposers(root)
				if err != nil {
					return WorkGroup{}, false, err
				}
				if !composerAmong(names, composerVariants) {
					continue
				}
			}
			return m.groupByRoot(root)
		}
	}
	return WorkGroup{}, false, nil
}

// composerAmong reports whether any credited composer name matches any requested
// variant (normalized comparison via core.Credits.MatchesRole).
func composerAmong(credited, variants []string) bool {
	cs := make(core.Credits, 0, len(credited))
	for _, n := range credited {
		cs = append(cs, core.Credit{Role: core.RoleComposer, Name: n})
	}
	for _, v := range variants {
		if cs.MatchesRole(core.RoleComposer, v) {
			return true
		}
	}
	return false
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
