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
}

// FindRecording returns ranked recording candidates (port of mb_mirror recordings_for).
func (m *MirrorDB) FindRecording(q RecordingQuery) (RecordingResult, error) {
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
	out = filterByCredits(out, q.Credits)
	return RecordingResult{Candidates: out}, nil
}

// filterByCredits keeps only candidates whose attached credits satisfy ALL
// requested credits (AND semantics). A requested conductor also matches a
// chorus_master credit — the directing umbrella; asymmetric, so a literal
// chorus_master request stays exact. Empty want returns the input unchanged.
func filterByCredits(cands []RecordingCandidate, want core.Credits) []RecordingCandidate {
	if len(want) == 0 {
		return cands
	}
	matches := func(req core.Credit, have core.Credits) bool {
		if have.MatchesRole(req.Role, req.Name) {
			return true
		}
		// directing umbrella: a requested conductor also matches a chorus_master credit
		return req.Role == core.RoleConductor && have.MatchesRole(core.RoleChorusMaster, req.Name)
	}
	var filtered []RecordingCandidate
	for _, cand := range cands {
		ok := true
		for _, req := range want {
			if !matches(req, cand.Credits) {
				ok = false
				break
			}
		}
		if ok {
			filtered = append(filtered, cand)
		}
	}
	return filtered
}

// firstISRC returns the first entry of a "a, b, c" GROUP_CONCAT, or "".
func firstISRC(concat string) string {
	if concat == "" {
		return ""
	}
	return strings.SplitN(concat, ", ", 2)[0]
}

func (m *MirrorDB) findRecordingsByWork(q RecordingQuery) (RecordingResult, error) {
	workID, ok, err := m.resolveWorkID(q.Work)
	if err != nil {
		return RecordingResult{}, err
	}
	if !ok {
		return RecordingResult{}, nil // unresolved work → no candidates
	}

	rows, err := m.DB.Query(
		`SELECT r.id, r.gid, r.name, r.length, GROUP_CONCAT(i.isrc, ', ') AS isrcs
		   FROM l_recording_work lrw
		   JOIN link l ON l.id = lrw.link
		   JOIN recording r ON r.id = lrw.entity0
		   LEFT JOIN isrc i ON i.recording = r.id
		  WHERE lrw.entity1 = ? AND l.link_type = ?
		  GROUP BY r.id
		  ORDER BY COUNT(i.isrc) DESC, r.id ASC
		  LIMIT ?`, workID, linkTypePerformance, q.Limit)
	if err != nil {
		return RecordingResult{}, err
	}
	defer rows.Close()

	var cands []RecordingCandidate
	for rows.Next() {
		var id int64
		var gid, name string
		var length sql.NullInt64
		var isrcs sql.NullString
		if err := rows.Scan(&id, &gid, &name, &length, &isrcs); err != nil {
			return RecordingResult{}, err
		}
		c := RecordingCandidate{
			MBID:  core.MBID(gid),
			ISRC:  core.ISRC(firstISRC(isrcs.String)),
			Title: name,
			Match: Match{},
		}
		if length.Valid {
			c.DurationS = int(length.Int64 / 1000)
		}
		credits, err := m.recordingCredits(id)
		if err != nil {
			return RecordingResult{}, err
		}
		c.Credits = credits
		cands = append(cands, c)
	}
	if err := rows.Err(); err != nil {
		return RecordingResult{}, err
	}

	// Apply credit filter: ALL requested credits must match (AND semantics),
	// with the conductor→chorus_master umbrella.
	cands = filterByCredits(cands, q.Credits)

	// Warning: unfiltered work query that was truncated by limit.
	var warnings []string
	if len(q.Credits) == 0 {
		var total int
		err := m.DB.QueryRow(
			`SELECT COUNT(DISTINCT lrw.entity0) FROM l_recording_work lrw
			   JOIN link l ON l.id = lrw.link
			  WHERE lrw.entity1 = ? AND l.link_type = ?`, workID, linkTypePerformance).Scan(&total)
		if err != nil {
			return RecordingResult{}, err
		}
		if total > q.Limit {
			warnings = append(warnings, fmt.Sprintf(
				"work %q has %d recordings; showing %d (most-released first) — narrow with --credit (orchestra:/conductor:/soloist:/chorus:)",
				q.Work, total, len(cands)))
		}
	}

	return RecordingResult{Candidates: cands, Warnings: warnings}, nil
}
