package catalog

import (
	"database/sql"
	"strconv"
	"strings"

	"github.com/cehbz/tidalist/core"
)

// Track is one position in a tracklist.
type Track struct {
	Position  int       `json:"position"`
	Title     string    `json:"title"`
	MBID      core.MBID `json:"mbid,omitempty"`
	ISRC      core.ISRC `json:"isrc,omitempty"`
	DurationS int       `json:"duration_s,omitempty"`
}

// parseDuration converts "M:SS" or "H:MM:SS" to seconds; returns 0 if unparseable.
func parseDuration(s string) int {
	if s == "" {
		return 0
	}
	parts := strings.Split(s, ":")
	total := 0
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return 0
		}
		total = total*60 + n
	}
	return total
}

// TracklistByReleaseGroup resolves the MB canonical tracklist for a release-group
// gid via canonical_musicbrainz_data (entered by recording_mbid, the only index),
// falling through recordings until one has a canonical release.
func (m *MirrorDB) TracklistByReleaseGroup(rgGID string) ([]Track, error) {
	rows, err := m.DB.Query(
		`SELECT r.gid
		   FROM release_group rg
		   JOIN release rel ON rel.release_group = rg.id
		   JOIN medium m ON m.release = rel.id
		   JOIN track t ON t.medium = m.id
		   JOIN recording r ON r.id = t.recording
		  WHERE rg.gid = ?
		  ORDER BY m.position, t.position`, rgGID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var recGID string
		if err := rows.Scan(&recGID); err != nil {
			return nil, err
		}
		var releaseGID string
		err := m.DB.QueryRow(
			`SELECT release_mbid FROM canonical_musicbrainz_data WHERE recording_mbid = ? LIMIT 1`,
			recGID).Scan(&releaseGID)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		return m.tracklistByRelease(releaseGID)
	}
	return nil, rows.Err()
}

// tracklistByRelease returns the ordered tracklist of an MB release (by gid).
func (m *MirrorDB) tracklistByRelease(releaseGID string) ([]Track, error) {
	rows, err := m.DB.Query(
		`SELECT m.position, t.position, t.name, r.gid, t.length,
		        GROUP_CONCAT(i.isrc, ', ') AS isrcs
		   FROM release rel
		   JOIN medium m ON m.release = rel.id
		   JOIN track t ON t.medium = m.id
		   JOIN recording r ON r.id = t.recording
		   LEFT JOIN isrc i ON i.recording = r.id
		  WHERE rel.gid = ?
		  GROUP BY t.id
		  ORDER BY m.position, t.position`, releaseGID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Track
	pos := 0
	for rows.Next() {
		var disc, tpos int
		var name, gid string
		var length sql.NullInt64
		var isrcs sql.NullString
		if err := rows.Scan(&disc, &tpos, &name, &gid, &length, &isrcs); err != nil {
			return nil, err
		}
		pos++
		tk := Track{Position: pos, Title: name, MBID: core.MBID(gid), ISRC: core.ISRC(firstISRC(isrcs.String))}
		if length.Valid {
			tk.DurationS = int(length.Int64 / 1000)
		}
		out = append(out, tk)
	}
	return out, rows.Err()
}

// TracklistByMaster returns the Discogs main-release tracklist for a master id.
func (m *MirrorDB) TracklistByMaster(masterID int64) ([]Track, error) {
	rows, err := m.DB.Query(
		`SELECT t.seq, t.title, t.duration
		   FROM dc.track t
		   JOIN dc.release r ON r.id = t.release_id
		  WHERE r.master_id = ? AND r.is_main_release = 1
		  ORDER BY t.seq`, masterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Track
	pos := 0
	for rows.Next() {
		var seq int
		var title, dur string
		if err := rows.Scan(&seq, &title, &dur); err != nil {
			return nil, err
		}
		pos++
		out = append(out, Track{Position: pos, Title: title, DurationS: parseDuration(dur)})
	}
	return out, rows.Err()
}
