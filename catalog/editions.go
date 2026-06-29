package catalog

import (
	"database/sql"
	"strconv"
	"strings"

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
	rows, err := m.DB.Query(
		`SELECT rel.gid, rel.name,
		        COALESCE(rc.date_year, ruc.date_year) AS year,
		        a.name AS country,
		        GROUP_CONCAT(DISTINCT mf.name) AS formats,
		        GROUP_CONCAT(DISTINCT l.name) AS labels,
		        (SELECT COUNT(*) FROM track t JOIN medium m2 ON t.medium = m2.id WHERE m2.release = rel.id) AS track_count
		   FROM release rel
		   JOIN release_group rg ON rg.id = rel.release_group
		   LEFT JOIN release_country rc ON rc.release = rel.id
		   LEFT JOIN release_unknown_country ruc ON ruc.release = rel.id
		   LEFT JOIN area a ON a.id = rc.country
		   LEFT JOIN medium m ON m.release = rel.id
		   LEFT JOIN medium_format mf ON mf.id = m.format
		   LEFT JOIN release_label rl ON rl.release = rel.id
		   LEFT JOIN label l ON l.id = rl.label
		  WHERE rg.gid = ?
		  GROUP BY rel.id
		  ORDER BY COALESCE(rc.date_year, ruc.date_year), rel.gid`, rgGID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Edition
	for rows.Next() {
		var gid, name string
		var year sql.NullInt64
		var country, formats, labels sql.NullString
		var tc int
		if err := rows.Scan(&gid, &name, &year, &country, &formats, &labels, &tc); err != nil {
			return nil, err
		}
		e := Edition{MBID: core.MBID(gid), Title: name, Country: country.String, TrackCount: tc, Source: core.SourceMusicBrainz}
		if year.Valid {
			e.Year = int(year.Int64)
		}
		e.Formats = splitConcat(formats.String)
		e.Labels = splitConcat(labels.String)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (m *MirrorDB) AlbumEditionsDiscogs(masterID int64) ([]Edition, error) {
	rows, err := m.DB.Query(
		`SELECT rel.id, rel.title, rel.released_raw, rel.country, rel.is_main_release,
		        GROUP_CONCAT(DISTINCT rf.name) AS formats,
		        GROUP_CONCAT(DISTINCT rl.name) AS labels,
		        (SELECT COUNT(*) FROM dc.track t WHERE t.release_id = rel.id) AS track_count
		   FROM dc.release rel
		   LEFT JOIN dc.release_format rf ON rf.release_id = rel.id
		   LEFT JOIN dc.release_label rl ON rl.release_id = rel.id
		  WHERE rel.master_id = ?
		  GROUP BY rel.id
		  ORDER BY rel.is_main_release DESC, rel.released_raw, rel.id`, masterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Edition
	for rows.Next() {
		var id int64
		var title string
		var released, country, formats, labels sql.NullString
		var isMain int
		var tc int
		if err := rows.Scan(&id, &title, &released, &country, &isMain, &formats, &labels, &tc); err != nil {
			return nil, err
		}
		e := Edition{
			DiscogsReleaseID: id, Title: title, Year: parseYear(released.String),
			Country: country.String, TrackCount: tc, IsMainRelease: isMain == 1,
			Source: core.SourceDiscogs,
		}
		e.Formats = splitConcat(formats.String)
		e.Labels = splitConcat(labels.String)
		out = append(out, e)
	}
	return out, rows.Err()
}

// splitConcat splits a GROUP_CONCAT result (comma-separated) into a slice; "" → nil.
func splitConcat(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
