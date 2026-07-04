package catalog

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/cehbz/tidalist/core"
)

// linkTypePerformance is the l_recording_work link_type for "recording is a
// performance of work" (the only arc that links a recording to a work here).
const linkTypePerformance = 278

// RecordingQuery describes a recording to find.
type RecordingQuery struct {
	Title      string
	ArtistName string
	ArtistMBID core.MBID
	ISRC       core.ISRC
	Work       string
	Limit      int
	Credits    core.Credits
}

// RecordingCandidate is a ranked recording result.
type RecordingCandidate struct {
	MBID      core.MBID    `json:"mbid,omitempty"`
	ISRC      core.ISRC    `json:"isrc,omitempty"`
	Title     string       `json:"title"`
	DurationS int          `json:"duration_s,omitempty"`
	Match     Match        `json:"match"`
	Credits   core.Credits `json:"credits,omitempty"`
}

// RecordingResult is the return value of FindRecording.
type RecordingResult struct {
	Candidates []RecordingCandidate `json:"candidates"`
	Warnings   []string             `json:"warnings,omitempty"`
	// WorkResolution is WorkGroup.Resolution ("title"|"alias" only — this path
	// never calls the performer-discography fallback, so "performer-fallback"
	// cannot appear here), set only on the --work resolution path
	// (findRecordingsByWork).
	WorkResolution string `json:"work_resolution,omitempty"`
}

// FindRecording returns ranked recording candidates (port of mb_mirror recordings_for).
func (m *MirrorDB) FindRecording(q RecordingQuery) (RecordingResult, error) {
	if q.Limit <= 0 {
		q.Limit = 25
	}
	if q.Work != "" {
		return m.findRecordingsByWork(q)
	}
	// Resolve the artist filter: explicit MBID first, then the name via FTS.
	var artistID int64
	var confirmed bool
	if q.ArtistMBID != "" {
		err := m.DB.QueryRow(`SELECT id FROM artist WHERE gid = ?`, string(q.ArtistMBID)).Scan(&artistID)
		if err == nil {
			confirmed = true
		} else if err != sql.ErrNoRows {
			return RecordingResult{}, err
		}
	} else if q.ArtistName != "" {
		id, ok, err := m.resolveArtistID(q.ArtistName)
		if err != nil {
			return RecordingResult{}, err
		}
		artistID, confirmed = id, ok
	}

	// If an artist was requested but did not resolve, return no candidates.
	artistRequested := q.ArtistMBID != "" || q.ArtistName != ""
	if artistRequested && !confirmed {
		return RecordingResult{}, nil
	}

	var rows *sql.Rows
	var err error
	if confirmed {
		rows, err = m.DB.Query(
			`SELECT r.id, r.gid, r.name, r.length, GROUP_CONCAT(i.isrc, ', ') AS isrcs
			   FROM recording_fts f
			   JOIN recording r ON r.id = f.rowid
			   JOIN artist_credit_name acn ON acn.artist_credit = r.artist_credit
			   LEFT JOIN isrc i ON i.recording = r.id
			  WHERE recording_fts MATCH ? AND acn.artist = ?
			  GROUP BY r.id
			  ORDER BY f.rank
			  LIMIT ?`, ftsTitle(q.Title), artistID, q.Limit)
	} else {
		rows, err = m.DB.Query(
			`SELECT r.id, r.gid, r.name, r.length, GROUP_CONCAT(i.isrc, ', ') AS isrcs
			   FROM recording_fts f
			   JOIN recording r ON r.id = f.rowid
			   LEFT JOIN isrc i ON i.recording = r.id
			  WHERE recording_fts MATCH ?
			  GROUP BY r.id
			  ORDER BY f.rank
			  LIMIT ?`, ftsTitle(q.Title), q.Limit)
	}
	if err != nil {
		return RecordingResult{}, err
	}
	defer rows.Close()

	var out []RecordingCandidate
	for rows.Next() {
		var id int64
		var gid, name string
		var length sql.NullInt64
		var isrcs sql.NullString
		if err := rows.Scan(&id, &gid, &name, &length, &isrcs); err != nil {
			return RecordingResult{}, err
		}
		isrc := firstISRC(isrcs.String)
		match := Match{TitleDistance: floatPtr(titleDistance(q.Title, name))}
		if confirmed {
			match.ArtistConfirmed = boolPtr(true)
		}
		if q.ISRC != "" {
			match.ISRCExact = boolPtr(core.ISRC(isrc) == q.ISRC)
		}
		c := RecordingCandidate{MBID: core.MBID(gid), ISRC: core.ISRC(isrc), Title: name, Match: match}
		if length.Valid {
			c.DurationS = int(length.Int64 / 1000)
		}
		credits, err := m.recordingCredits(id)
		if err != nil {
			return RecordingResult{}, err
		}
		c.Credits = credits
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return RecordingResult{}, err
	}
	// Apply any non-artist --credit filter uniformly (the SQL above narrows by
	// artist only); no-op when q.Credits is empty.
	out, err = m.filterByCredits(out, q.Credits)
	if err != nil {
		return RecordingResult{}, err
	}
	return RecordingResult{Candidates: out}, nil
}

// filterByCredits keeps only candidates whose attached credits satisfy ALL
// requested credits (AND semantics), alias-expanded via expandWants so a query
// name variant (e.g. Latin "Valery Gergiev") matches a credit stored under any
// of the artist's known names (e.g. Cyrillic primary "Валерий Гергиев") — the
// same variant-aware satisfaction check ResolvePerformance already applies via
// expandWants/creditsSatisfy (catalog/performance.go), reused here rather than
// duplicated. Empty want returns the input unchanged.
func (m *MirrorDB) filterByCredits(cands []RecordingCandidate, want core.Credits) ([]RecordingCandidate, error) {
	if len(want) == 0 {
		return cands, nil
	}
	wants, err := m.expandWants(want)
	if err != nil {
		return nil, err
	}
	var filtered []RecordingCandidate
	for _, cand := range cands {
		if creditsSatisfy(cand.Credits, wants) {
			filtered = append(filtered, cand)
		}
	}
	return filtered, nil
}

// firstISRC returns the first entry of a "a, b, c" GROUP_CONCAT, or "".
func firstISRC(concat string) string {
	if concat == "" {
		return ""
	}
	return strings.SplitN(concat, ", ", 2)[0]
}

// findRecordingsByWork resolves --work to its work-group's recordings.
// Multi-root candidacy (see resolveWorkGroups' doc comment / rr-task-5-report.md
// FINDING 1): a title-FTS collision can resolve to a childful-but-empty decoy
// root before the alias-recovered family that actually carries the requested
// recordings — try each candidate root's recording query in order and use the
// first one that actually yields a candidate, the same retry
// ResolvePerformance already does across mbPerformances. The top-level
// WorkResolution reflects the WINNING root's Resolution, not necessarily
// groups[0]'s.
func (m *MirrorDB) findRecordingsByWork(q RecordingQuery) (RecordingResult, error) {
	composer := ""
	if names := q.Credits.Names(core.RoleComposer); len(names) > 0 {
		composer = names[0]
	}
	groups, groupWarnings, err := m.resolveWorkGroups(q.Work, composer)
	if err != nil {
		return RecordingResult{}, err
	}
	if len(groups) == 0 {
		return RecordingResult{Warnings: groupWarnings}, nil // unresolved work → no candidates
	}

	hasCredits := len(performerCredits(q.Credits)) > 0
	for _, g := range groups {
		cands, warnings, err := m.recordingsForGroup(g, q, hasCredits)
		if err != nil {
			return RecordingResult{}, err
		}
		if len(cands) > 0 {
			return RecordingResult{
				Candidates:     cands,
				Warnings:       append(groupWarnings, warnings...),
				WorkResolution: g.Resolution,
			}, nil
		}
	}
	// No candidate root's recording query yielded anything: report the
	// title-priority group's resolution, mirroring the pre-multi-root
	// single-candidate-miss behavior.
	return RecordingResult{Warnings: groupWarnings, WorkResolution: groups[0].Resolution}, nil
}

// recordingsForGroup runs the work-group recording query (+ --credit filter, +
// over-limit warning) for a single candidate root.
func (m *MirrorDB) recordingsForGroup(g WorkGroup, q RecordingQuery, hasCredits bool) ([]RecordingCandidate, []string, error) {
	inClause, args := intInClause(g.WorkIDs)

	// The SQL LIMIT must never bind before the --credit filter: for a work-group
	// with many more recordings than Limit, the top-Limit-by-ISRC-count window can
	// easily contain zero of the credit-matching rows even though the full
	// work-group has plenty (measured on the real mirror: a 2412-recording family
	// whose top-20 window held none of its 19 conductor-matching recordings — see
	// rr-task-5-report.md FINDING 2). When credits are requested, fetch every
	// work-group recording (no SQL LIMIT), filter by credit, THEN cap at Limit;
	// the SQL-side LIMIT is safe again once there is no --credit filter to starve.
	query := `SELECT r.id, r.gid, r.name, r.length, GROUP_CONCAT(i.isrc, ', ') AS isrcs
		   FROM l_recording_work lrw
		   JOIN link l ON l.id = lrw.link
		   JOIN recording r ON r.id = lrw.entity0
		   LEFT JOIN isrc i ON i.recording = r.id
		  WHERE lrw.entity1 IN (` + inClause + `) AND l.link_type = ?
		  GROUP BY r.id
		  ORDER BY COUNT(i.isrc) DESC, r.id ASC`
	queryArgs := append(append([]any{}, args...), linkTypePerformance)
	if !hasCredits {
		query += `
		  LIMIT ?`
		queryArgs = append(queryArgs, q.Limit)
	}
	rows, err := m.DB.Query(query, queryArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var cands []RecordingCandidate
	for rows.Next() {
		var id int64
		var gid, name string
		var length sql.NullInt64
		var isrcs sql.NullString
		if err := rows.Scan(&id, &gid, &name, &length, &isrcs); err != nil {
			return nil, nil, err
		}
		c := RecordingCandidate{MBID: core.MBID(gid), ISRC: core.ISRC(firstISRC(isrcs.String)), Title: name, Match: Match{}}
		if length.Valid {
			c.DurationS = int(length.Int64 / 1000)
		}
		credits, err := m.recordingCredits(id)
		if err != nil {
			return nil, nil, err
		}
		c.Credits = credits
		cands = append(cands, c)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	cands, err = m.filterByCredits(cands, performerCredits(q.Credits))
	if err != nil {
		return nil, nil, err
	}
	if hasCredits && len(cands) > q.Limit {
		cands = cands[:q.Limit]
	}

	var warnings []string
	if !hasCredits {
		var total int
		if err := m.DB.QueryRow(
			`SELECT COUNT(DISTINCT lrw.entity0) FROM l_recording_work lrw
			   JOIN link l ON l.id = lrw.link
			  WHERE lrw.entity1 IN (`+inClause+`) AND l.link_type = ?`,
			append(append([]any{}, args...), linkTypePerformance)...).Scan(&total); err != nil {
			return nil, nil, err
		}
		if total > q.Limit {
			warnings = append(warnings, fmt.Sprintf(
				"work %q has %d recordings; showing %d (most-released first) — narrow with --credit (orchestra:/conductor:/soloist:/chorus:)",
				q.Work, total, len(cands)))
		}
	}
	return cands, warnings, nil
}

// intInClause builds a "?,?,?" placeholder list and its args for an IN clause.
func intInClause(ids []int64) (string, []any) {
	if len(ids) == 0 {
		return "NULL", nil
	}
	ph := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		ph[i] = "?"
		args[i] = id
	}
	return strings.Join(ph, ","), args
}

// performerCredits drops the composer role (a work-level, not recording-level,
// credit) so the recording --credit filter isn't over-constrained by it.
func performerCredits(cs core.Credits) core.Credits {
	var out core.Credits
	for _, c := range cs {
		if c.Role != core.RoleComposer {
			out = append(out, c)
		}
	}
	return out
}

// RecordingInfo is a recording's identity for GM materialization: what a track
// entry needs to be self-contained (identity, credit, vintage, credits).
type RecordingInfo struct {
	MBID         core.MBID
	Title        string
	ArtistCredit string
	ISRC         core.ISRC
	DurationS    int
	Album        string
	Year         int
	Credits      core.Credits
}

// RecordingByGID returns a recording's identity by gid. ok=false when unknown.
// Album/Year come from the recording's earliest release-group (the original
// release, never a reissue's).
func (m *MirrorDB) RecordingByGID(gid string) (RecordingInfo, bool, error) {
	var id int64
	var name string
	var length sql.NullInt64
	var credit, isrcs sql.NullString
	err := m.DB.QueryRow(
		`SELECT r.id, r.name, r.length,
		        (SELECT GROUP_CONCAT(a.name, ', ')
		           FROM artist_credit_name acn JOIN artist a ON a.id = acn.artist
		          WHERE acn.artist_credit = r.artist_credit),
		        (SELECT GROUP_CONCAT(i.isrc, ', ') FROM isrc i WHERE i.recording = r.id)
		   FROM recording r WHERE r.gid = ?`, gid).Scan(&id, &name, &length, &credit, &isrcs)
	if err == sql.ErrNoRows {
		return RecordingInfo{}, false, nil
	}
	if err != nil {
		return RecordingInfo{}, false, err
	}
	info := RecordingInfo{
		MBID:         core.MBID(gid),
		Title:        name,
		ArtistCredit: credit.String,
		ISRC:         core.ISRC(firstISRC(isrcs.String)),
	}
	if length.Valid {
		info.DurationS = int(length.Int64 / 1000)
	}
	rgID, year, ok, err := m.earliestReleaseGroup(id)
	if err != nil {
		return RecordingInfo{}, false, err
	}
	if ok {
		info.Year = year
		if err := m.DB.QueryRow(`SELECT name FROM release_group WHERE id = ?`, rgID).
			Scan(&info.Album); err != nil && err != sql.ErrNoRows {
			return RecordingInfo{}, false, err
		}
	}
	if info.Credits, err = m.recordingCredits(id); err != nil {
		return RecordingInfo{}, false, err
	}
	return info, true, nil
}
