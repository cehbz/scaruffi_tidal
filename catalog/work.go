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
	// Resolution is the provenance of the winning root: "title" (a title-FTS
	// candidate produced it — title wins when both title and alias candidates
	// would have; see resolveWorkGroup step (c)), "alias" (only a
	// workAliasCandidates id did), or "performer-fallback" (workGroupFromPerformers
	// produced the group, not title/alias resolution at all). Threaded to
	// PerformanceResult/RecordingResult as JSON work_resolution.
	Resolution string
}

// workGroupLinkParts is l_work_work link_type 281 (parent "has parts" movements).
// 314 based-on / 350 arrangement / 239 medley / 315 revision-of are NOT parts and
// are excluded by matching only 281.
const workGroupLinkParts = 281

// composerLinkType is l_artist_work link_type 168 (composer).
const composerLinkType = 168

// resolveWorkGroup resolves the single title-priority work-group root for
// (title, composer): a thin compatibility wrapper over resolveWorkGroups that
// returns only its first element and discards its warnings. Both
// ResolvePerformance and findRecordingsByWork call resolveWorkGroups directly
// so they can retry across candidates; this wrapper remains for callers (and
// tests) that only want a single best-guess root and accept the known trap
// (see resolveWorkGroups' doc comment).
func (m *MirrorDB) resolveWorkGroup(title, composer string) (WorkGroup, bool, error) {
	groups, _, err := m.resolveWorkGroups(title, composer)
	if err != nil {
		return WorkGroup{}, false, err
	}
	if len(groups) == 0 {
		return WorkGroup{}, false, nil
	}
	return groups[0], true, nil
}

// resolveWorkGroups resolves the ordered, DISTINCT candidate work-group roots
// for (title, composer): title-sourced candidates before alias-sourced ones
// (see candSource below), each candidate collapsed to its childful 281-parent
// when one exists, deduplicated by root (first occurrence wins, so a root
// reached by both a title and an alias candidate keeps its title-sourced
// Resolution — see TestResolveWorkGroupTitleWinsWhenBothSourcesReachSameRoot).
//
// MB can hold two distinct, both-real, both-childful works under the
// IDENTICAL literal title in different scopes — e.g. a concert orchestral
// work and an unrelated instrument-transcription work both literally titled
// "The Rite of Spring" (an unmerged-duplicate/alternate-arrangement MB data
// quirk; see rr-task-5-report.md FINDING 1). A single "best root" resolution
// silently commits to whichever of them FTS/alias ranked first, even when
// that candidate has zero of the requested performers' recordings. Returning
// every distinct root lets ResolvePerformance walk the list and use whichever
// candidate actually carries matching performances, instead of failing
// outright (or worse, substituting a real-but-wrong work) when the
// first-ranked candidate is real but not the one the query means.
func (m *MirrorDB) resolveWorkGroups(title, composer string) ([]WorkGroup, []string, error) {
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
	composerID, hasComposer, cerr := m.composerIDFor(composer)
	if cerr != nil {
		return nil, nil, cerr
	}
	var rows *sql.Rows
	var err error
	if hasComposer {
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
		return nil, nil, err
	}
	var candIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, nil, err
		}
		candIDs = append(candIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	// work_alias union (BOTH arms of step (a), composer-conditioned or not):
	// English forms live mostly on movement-level works, not the family root, so a
	// title-FTS miss on the root still recovers the family via a movement alias —
	// the alias-sourced ids pass through the same composer filter (b) and root
	// resolution (c) as the title-FTS candidates, unchanged. truncated signals the
	// 50-id scan cap broke the alias scan short — surfaced as a warning so a
	// caller knows resolution may be incomplete, never silently.
	aliasIDs, truncated, err := m.workAliasCandidates(title, composerID, hasComposer)
	if err != nil {
		return nil, nil, err
	}
	var warnings []string
	if truncated {
		warnings = append(warnings, "work_alias candidates truncated at 50; resolution may be incomplete")
	}
	// candSource tracks each candidate id's origin for WorkGroup.Resolution
	// ("title" vs "alias"). Title-sourced ids keep their original position ahead
	// of any newly-appended alias-only id, so step (c) below — which iterates
	// candidates (via matched) in this same order and stops at the FIRST one
	// whose climb reaches a childful root — naturally prefers a title candidate
	// over an alias one whenever both would resolve to the same root: "title
	// wins when both sources contributed" falls out of iteration order, not a
	// separate rule.
	seen := make(map[int64]bool, len(candIDs))
	candSource := make(map[int64]string, len(candIDs)+len(aliasIDs))
	for _, id := range candIDs {
		seen[id] = true
		candSource[id] = "title"
	}
	for _, id := range aliasIDs {
		if !seen[id] {
			seen[id] = true
			candIDs = append(candIDs, id)
			candSource[id] = "alias"
		}
	}

	// (b) keep candidates whose composer matches (skip the filter when none
	// requested). The requested name is expanded to its alias set, so a Latin
	// query matches a Cyrillic-named composer.
	var composerNames []string
	if composer != "" {
		var err error
		if composerNames, err = m.nameVariants(composer); err != nil {
			return nil, warnings, err
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
			return nil, warnings, err
		}
		if composerAmong(names, composerNames) {
			matched = append(matched, id)
		}
	}
	if len(matched) == 0 {
		return nil, warnings, nil
	}

	// (c) resolve EVERY matched candidate's root: climb its 281 parent chain
	// TRANSITIVELY (bounded by workGroupMaxDepth, same bound and query shape as
	// workGroupFromPerformers' ancestor climb) — a one-level-only walk stops at a
	// childful MID-level part when a deeper alias hit (e.g. a grandchild movement)
	// gave that part a child of its own, wrongly surfacing it as a second root
	// instead of continuing to the true family root (see
	// TestResolveWorkGroupsAscendsPastMidLevelPartToMatthausRoot). Then check for
	// 281 children (a real work-group) vs. an arc-less title-twin stub — same
	// per-candidate logic as before, just no longer stopping at the first childful
	// hit. Distinct childful roots are kept in matched's order (title-sourced
	// candidates precede alias-sourced ones — see candSource above), first
	// occurrence wins on a duplicate root (the "title wins when both sources reach
	// the same root" guarantee). When NO candidate resolves to a childful root,
	// fall back to a single group rooted at matched[0] itself, unwalked — the
	// pre-existing standalone-work behavior.
	type rootHit struct {
		root         int64
		resolvedFrom string
	}
	seenRoot := make(map[int64]bool, len(matched))
	var roots []rootHit
	for _, cand := range matched {
		r := cand
		for depth := 0; depth < workGroupMaxDepth; depth++ {
			var parent int64
			perr := m.DB.QueryRow(
				`SELECT lww.entity0 FROM l_work_work lww
				   JOIN link l ON l.id = lww.link
				  WHERE lww.entity1 = ? AND l.link_type = ?
				  ORDER BY lww.entity0 LIMIT 1`, r, workGroupLinkParts).Scan(&parent)
			if perr == sql.ErrNoRows {
				break
			}
			if perr != nil {
				return nil, warnings, perr
			}
			r = parent
		}
		var childCount int
		if err := m.DB.QueryRow(
			`SELECT COUNT(*) FROM l_work_work lww
			   JOIN link l ON l.id = lww.link
			  WHERE lww.entity0 = ? AND l.link_type = ?`, r, workGroupLinkParts).Scan(&childCount); err != nil {
			return nil, warnings, err
		}
		if childCount == 0 || seenRoot[r] {
			continue
		}
		seenRoot[r] = true
		roots = append(roots, rootHit{root: r, resolvedFrom: candSource[cand]})
	}
	if len(roots) == 0 {
		roots = []rootHit{{root: matched[0], resolvedFrom: candSource[matched[0]]}}
	}

	out := make([]WorkGroup, 0, len(roots))
	for _, rh := range roots {
		g, ok, err := m.groupByRoot(rh.root)
		if err != nil {
			return nil, warnings, err
		}
		if !ok {
			continue
		}
		g.Resolution = rh.resolvedFrom
		out = append(out, g)
	}
	if len(out) == 0 {
		return nil, warnings, nil
	}
	return out, warnings, nil
}

// workAliasCandidates scans work_alias for aliases whose folded form begins with the
// folded query title (strings.HasPrefix over foldWorkTitle). When the composer resolves
// (hasComposer), the scan is CONDITIONED on that composer's works — joining l_artist_work
// (168) so only the composer's own aliased works are considered — driving the indexed
// l_artist_work.entity0 -> work_alias.work join. That bounds candidates to one composer,
// so same-prefix aliases of OTHER composers' works can never evict a relevant candidate:
// no cap is needed on this path (truncated is always false), and it is safe because
// resolveWorkGroups step (b) already drops every non-composer-credited alias hit. Without
// a resolved composer, it falls back to the full table scan capped at 50 (truncated=true
// when the cap broke the scan short — the caller surfaces an incomplete-resolution
// warning). ORDER BY work keeps candidate order deterministic (Resolution provenance
// depends on it); distinct work ids.
func (m *MirrorDB) workAliasCandidates(title string, composerID int64, hasComposer bool) (out []int64, truncated bool, err error) {
	want := foldWorkTitle(title)
	if want == "" {
		return nil, false, nil
	}
	var rows *sql.Rows
	if hasComposer {
		rows, err = m.DB.Query(
			`SELECT DISTINCT wa.work, wa.name
			   FROM l_artist_work law
			   JOIN link l ON l.id = law.link AND l.link_type = ?
			   JOIN work_alias wa ON wa.work = law.entity1
			  WHERE law.entity0 = ?
			  ORDER BY wa.work`, composerLinkType, composerID)
	} else {
		rows, err = m.DB.Query(`SELECT work, name FROM work_alias ORDER BY work`)
	}
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	seen := map[int64]bool{}
	for rows.Next() {
		var work int64
		var name string
		if err := rows.Scan(&work, &name); err != nil {
			return nil, false, err
		}
		if !strings.HasPrefix(foldWorkTitle(name), want) {
			continue
		}
		if seen[work] {
			continue
		}
		seen[work] = true
		out = append(out, work)
		if !hasComposer && len(out) >= 50 {
			truncated = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	return out, truncated, nil
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
			g, ok, err := m.groupByRoot(root)
			if err != nil || !ok {
				return g, ok, err
			}
			g.Resolution = "performer-fallback"
			return g, true, nil
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
