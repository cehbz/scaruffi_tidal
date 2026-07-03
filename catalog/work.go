package catalog

import (
	"database/sql"
	"strings"
	"unicode"

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

// composerLinkType is l_artist_work link_type 168 (composer).
const composerLinkType = 168

func (m *MirrorDB) resolveWorkGroup(title, composer string) (WorkGroup, bool, error) {
	// (a) candidate works by title FTS, composer-CONDITIONED when the composer
	// resolves to a known artist: l_artist_work (168, composer) is joined into
	// the candidate query itself so the top-25 window is composed of works
	// credited to that artist, not the unconditioned title-FTS top-25. A generic
	// title ("Symphony No. 5") otherwise drowns among hundreds of same-titled
	// works by other composers and the intended composer's work never enters an
	// unconditioned window (see TestResolveWorkGroupSurvivesGenericTitleFlood).
	// When no composer is requested, or the requested name resolves to no known
	// artist, the query is unconditioned — existing behavior is unchanged, and
	// (b) below still filters by composer name/alias as before.
	var rows *sql.Rows
	var err error
	if composerID, ok, cerr := m.composerIDFor(composer); cerr != nil {
		return WorkGroup{}, false, cerr
	} else if ok {
		rows, err = m.DB.Query(
			`SELECT DISTINCT f.rowid
			   FROM work_fts f
			   JOIN l_artist_work law ON law.entity1 = f.rowid
			   JOIN link l ON l.id = law.link AND l.link_type = ?
			  WHERE work_fts MATCH ? AND law.entity0 = ?
			  ORDER BY f.rank LIMIT 25`, composerLinkType, ftsTitle(title), composerID)
	} else {
		rows, err = m.DB.Query(
			`SELECT rowid FROM work_fts WHERE work_fts MATCH ? ORDER BY rank LIMIT 25`, ftsTitle(title))
	}
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

	// work_alias union (BOTH arms of step (a), composer-conditioned or not):
	// English forms live mostly on movement-level works, not the family root, so a
	// title-FTS miss on the root still recovers the family via a movement alias —
	// the alias-sourced ids pass through the same composer filter (b) and root
	// resolution (c) as the title-FTS candidates, unchanged.
	aliasIDs, err := m.workAliasCandidates(title)
	if err != nil {
		return WorkGroup{}, false, err
	}
	seen := make(map[int64]bool, len(candIDs))
	for _, id := range candIDs {
		seen[id] = true
	}
	for _, id := range aliasIDs {
		if !seen[id] {
			seen[id] = true
			candIDs = append(candIDs, id)
		}
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

// workAliasCandidates scans work_alias for aliases whose folded form begins
// with the folded query title (strings.HasPrefix over foldWorkTitle). Real
// mirror aliases carry punctuation the queries lack — e.g. alias "St. Matthew
// Passion, BWV 244: Part I" vs query "St Matthew Passion" — so the comparison
// folds punctuation out entirely, not just the core.NormalizeName diacritic/
// case fold the Go credit matcher uses. A full table scan (158k rows on the
// mirror) is milliseconds; distinct work ids, capped at 50.
func (m *MirrorDB) workAliasCandidates(title string) ([]int64, error) {
	want := foldWorkTitle(title)
	if want == "" {
		return nil, nil
	}
	rows, err := m.DB.Query(`SELECT work, name FROM work_alias`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[int64]bool{}
	var out []int64
	for rows.Next() {
		var work int64
		var name string
		if err := rows.Scan(&work, &name); err != nil {
			return nil, err
		}
		if !strings.HasPrefix(foldWorkTitle(name), want) {
			continue
		}
		if seen[work] {
			continue
		}
		seen[work] = true
		out = append(out, work)
		if len(out) >= 50 {
			break
		}
	}
	return out, rows.Err()
}

// foldWorkTitle folds a title for the alias-prefix comparison in
// workAliasCandidates ONLY: core.NormalizeName(s) (lowercase, diacritic-fold),
// then strip every rune that is not a letter, digit, or space, then collapse
// whitespace runs to a single space. This is deliberately scoped here rather
// than changing core.NormalizeName itself — that function is the credit
// matcher's fold and has a much wider blast radius across the package.
func foldWorkTitle(s string) string {
	folded := core.NormalizeName(s)
	var b strings.Builder
	b.Grow(len(folded))
	for _, r := range folded {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
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
		aid, ok, err := m.resolveArtistIDForRole(w.Name, w.Role)
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

// composerIDFor resolves a composer name to its artist id for candidate-query
// conditioning (resolveWorkGroup step (a)). Returns ok=false, nil error when
// composer is empty or resolves to no known artist — the caller then falls back
// to an unconditioned candidate query, preserving existing behavior.
func (m *MirrorDB) composerIDFor(composer string) (int64, bool, error) {
	if composer == "" {
		return 0, false, nil
	}
	return m.resolveArtistIDForRole(composer, core.RoleComposer)
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
		  WHERE law.entity1 = ? AND l.link_type = ?
		  ORDER BY law.link_order`, workID, composerLinkType)
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
