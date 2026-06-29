package catalog

import (
	"database/sql"
	"strconv"

	"github.com/cehbz/tidalist/core"
)

// Edition is one release/pressing of an album, from a single source.
type Edition struct {
	MBID             core.MBID   `json:"mbid,omitempty"`
	DiscogsReleaseID int64       `json:"discogs_release_id,omitempty"`
	Title            string      `json:"title"`
	Year             int         `json:"year,omitempty"`
	Country          string      `json:"country,omitempty"`
	Formats          []string    `json:"formats,omitempty"`
	Labels           []string    `json:"labels,omitempty"`
	TrackCount       int         `json:"track_count,omitempty"`
	IsMainRelease    bool        `json:"is_main_release,omitempty"`
	Source           core.Source `json:"source"`
}

// parseYear extracts a leading 4-digit year from mixed date text, else 0.
func parseYear(s string) int {
	if len(s) < 4 {
		return 0
	}
	y, err := strconv.Atoi(s[:4])
	if err != nil {
		return 0
	}
	return y
}

func (m *MirrorDB) AlbumEditionsMB(rgGID string) ([]Edition, error) {
	// One row per release: release + date (country or unknown-country) + area name
	// + a correlated track count. Formats and labels are fetched per edition below,
	// so multi-valued names (e.g. labels with embedded commas) are never concatenated
	// and re-split. GROUP BY rel.id collapses the release_country multi-country fan-out.
	rows, err := m.DB.Query(
		`SELECT rel.id, rel.gid, rel.name,
		        COALESCE(rc.date_year, ruc.date_year) AS year,
		        a.name AS country,
		        (SELECT COUNT(*) FROM track t JOIN medium m2 ON t.medium = m2.id WHERE m2.release = rel.id) AS track_count,
		        rel.discogs_release_id
		   FROM release rel
		   JOIN release_group rg ON rg.id = rel.release_group
		   LEFT JOIN release_country rc ON rc.release = rel.id
		   LEFT JOIN release_unknown_country ruc ON ruc.release = rel.id
		   LEFT JOIN area a ON a.id = rc.country
		  WHERE rg.gid = ?
		  GROUP BY rel.id
		  ORDER BY COALESCE(rc.date_year, ruc.date_year), rel.gid`, rgGID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Edition
	for rows.Next() {
		var id int64
		var gid, name string
		var year, drid sql.NullInt64
		var country sql.NullString
		var tc int
		if err := rows.Scan(&id, &gid, &name, &year, &country, &tc, &drid); err != nil {
			return nil, err
		}
		e := Edition{MBID: core.MBID(gid), Title: name, Country: country.String, TrackCount: tc, Source: core.SourceMusicBrainz}
		if year.Valid {
			e.Year = int(year.Int64)
		}
		if drid.Valid {
			e.DiscogsReleaseID = drid.Int64
		}
		if e.Formats, err = m.editionFormatsMB(id); err != nil {
			return nil, err
		}
		if e.Labels, err = m.editionLabelsMB(id); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (m *MirrorDB) AlbumEditionsDiscogs(masterID int64) ([]Edition, error) {
	// Sub-tracks (parent_track_id set) are index entries under a parent track, not
	// playable positions, so they are excluded from the edition track count. Formats
	// and labels are fetched per edition below (comma-safe, no GROUP_CONCAT split).
	rows, err := m.DB.Query(
		`SELECT rel.id, rel.title, rel.released_raw, rel.country, rel.is_main_release,
		        (SELECT COUNT(*) FROM dc.track t
		          WHERE t.release_id = rel.id AND t.parent_track_id IS NULL) AS track_count
		   FROM dc.release rel
		  WHERE rel.master_id = ?
		  ORDER BY rel.is_main_release DESC, rel.released_raw, rel.id`, masterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Edition
	for rows.Next() {
		var id int64
		var title string
		var released, country sql.NullString
		var isMain int
		var tc int
		if err := rows.Scan(&id, &title, &released, &country, &isMain, &tc); err != nil {
			return nil, err
		}
		e := Edition{
			DiscogsReleaseID: id, Title: title, Year: parseYear(released.String),
			Country: country.String, TrackCount: tc, IsMainRelease: isMain == 1,
			Source: core.SourceDiscogs,
		}
		if e.Formats, err = m.editionFormatsDiscogs(id); err != nil {
			return nil, err
		}
		if e.Labels, err = m.editionLabelsDiscogs(id); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// editionFormatsMB returns the distinct medium formats of an MB release.
func (m *MirrorDB) editionFormatsMB(releaseID int64) ([]string, error) {
	return m.scanStrings(
		`SELECT DISTINCT mf.name
		   FROM medium m
		   JOIN medium_format mf ON mf.id = m.format
		  WHERE m.release = ?
		  ORDER BY mf.name`, releaseID)
}

// editionLabelsMB returns the distinct label names of an MB release.
func (m *MirrorDB) editionLabelsMB(releaseID int64) ([]string, error) {
	return m.scanStrings(
		`SELECT DISTINCT l.name
		   FROM release_label rl
		   JOIN label l ON l.id = rl.label
		  WHERE rl.release = ?
		  ORDER BY l.name`, releaseID)
}

// editionFormatsDiscogs returns a Discogs release's format names, in seq order.
func (m *MirrorDB) editionFormatsDiscogs(releaseID int64) ([]string, error) {
	return m.scanStrings(
		`SELECT name FROM dc.release_format WHERE release_id = ? ORDER BY seq`, releaseID)
}

// editionLabelsDiscogs returns a Discogs release's label names, in seq order.
func (m *MirrorDB) editionLabelsDiscogs(releaseID int64) ([]string, error) {
	return m.scanStrings(
		`SELECT name FROM dc.release_label WHERE release_id = ? ORDER BY seq`, releaseID)
}

// scanStrings runs a single-string-column query and collects every value.
func (m *MirrorDB) scanStrings(query string, args ...any) ([]string, error) {
	rows, err := m.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
