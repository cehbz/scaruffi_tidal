package catalog

import (
	"database/sql"
	"strings"

	"github.com/cehbz/tidalist/core"
)

// RecordingQuery describes a recording to find.
type RecordingQuery struct {
	Title      string
	ArtistName string
	ArtistMBID core.MBID
	ISRC       core.ISRC
	Work       string
	Limit      int
}

// RecordingCandidate is a ranked recording result.
type RecordingCandidate struct {
	MBID      core.MBID `json:"mbid,omitempty"`
	ISRC      core.ISRC `json:"isrc,omitempty"`
	Title     string    `json:"title"`
	DurationS int       `json:"duration_s,omitempty"`
	Match     Match     `json:"match"`
}

// FindRecording returns ranked recording candidates (port of mb_mirror recordings_for).
func (m *MirrorDB) FindRecording(q RecordingQuery) ([]RecordingCandidate, error) {
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
			return nil, err
		}
	} else if q.ArtistName != "" {
		id, ok, err := m.resolveArtistID(q.ArtistName)
		if err != nil {
			return nil, err
		}
		artistID, confirmed = id, ok
	}

	// If an artist was requested but did not resolve, return no candidates.
	artistRequested := q.ArtistMBID != "" || q.ArtistName != ""
	if artistRequested && !confirmed {
		return nil, nil
	}

	var rows *sql.Rows
	var err error
	if confirmed {
		rows, err = m.DB.Query(
			`SELECT r.gid, r.name, r.length, GROUP_CONCAT(i.isrc, ', ') AS isrcs
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
			`SELECT r.gid, r.name, r.length, GROUP_CONCAT(i.isrc, ', ') AS isrcs
			   FROM recording_fts f
			   JOIN recording r ON r.id = f.rowid
			   LEFT JOIN isrc i ON i.recording = r.id
			  WHERE recording_fts MATCH ?
			  GROUP BY r.id
			  ORDER BY f.rank
			  LIMIT ?`, ftsTitle(q.Title), q.Limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RecordingCandidate
	for rows.Next() {
		var gid, name string
		var length sql.NullInt64
		var isrcs sql.NullString
		if err := rows.Scan(&gid, &name, &length, &isrcs); err != nil {
			return nil, err
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
		out = append(out, c)
	}
	return out, rows.Err()
}

// firstISRC returns the first entry of a "a, b, c" GROUP_CONCAT, or "".
func firstISRC(concat string) string {
	if concat == "" {
		return ""
	}
	return strings.SplitN(concat, ", ", 2)[0]
}

func (m *MirrorDB) findRecordingsByWork(q RecordingQuery) ([]RecordingCandidate, error) {
	workID, ok, err := m.resolveWorkID(q.Work)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil // unresolved work → no candidates
	}
	rows, err := m.DB.Query(
		`SELECT r.gid, r.name, r.length, GROUP_CONCAT(i.isrc, ', ') AS isrcs
		   FROM l_recording_work lrw
		   JOIN link l ON l.id = lrw.link
		   JOIN recording r ON r.id = lrw.entity0
		   LEFT JOIN isrc i ON i.recording = r.id
		  WHERE lrw.entity1 = ? AND l.link_type = 278
		  GROUP BY r.id
		  ORDER BY lrw.link_order
		  LIMIT ?`, workID, q.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RecordingCandidate
	for rows.Next() {
		var gid, name string
		var length sql.NullInt64
		var isrcs sql.NullString
		if err := rows.Scan(&gid, &name, &length, &isrcs); err != nil {
			return nil, err
		}
		c := RecordingCandidate{MBID: core.MBID(gid), ISRC: core.ISRC(firstISRC(isrcs.String)), Title: name, Match: Match{}}
		if length.Valid {
			c.DurationS = int(length.Int64 / 1000)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
