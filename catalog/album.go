package catalog

import (
	"database/sql"
	"strings"

	"github.com/cehbz/tidalist/core"
)

// AlbumQuery describes an album to find.
type AlbumQuery struct {
	Title      string
	ArtistName string
	ArtistMBID core.MBID
	Year       int
	Limit      int
}

// AlbumCandidate is a ranked album result from one source (MB or Discogs).
type AlbumCandidate struct {
	MBID            core.MBID            `json:"mbid,omitempty"`
	DiscogsMasterID core.DiscogsMasterID `json:"discogs_master_id,omitempty"`
	Title           string               `json:"title"`
	Credits         core.Credits         `json:"credits,omitempty"`
	Year            int                  `json:"year,omitempty"`
	Traits          []core.ReleaseTrait  `json:"traits,omitempty"`
	Styles          []string             `json:"styles,omitempty"`
	TrackCount      int                  `json:"track_count,omitempty"`
	Sources         []core.Source        `json:"sources"`
	Match           Match                `json:"match"`
}

// FindAlbum returns ranked album candidates. MB release-groups and Discogs masters
// are returned as source-tagged peers (no reconcile); the agent correlates.
func (m *MirrorDB) FindAlbum(q AlbumQuery) ([]AlbumCandidate, error) {
	mb, err := m.findAlbumMB(q)
	if err != nil {
		return nil, err
	}
	dc, err := m.findAlbumDiscogs(q)
	if err != nil {
		return nil, err
	}
	return append(mb, dc...), nil
}

func (m *MirrorDB) findAlbumDiscogs(q AlbumQuery) ([]AlbumCandidate, error) {
	// Determine the artist NAME for the Discogs filter.
	name := q.ArtistName
	requested := q.ArtistName != "" || q.ArtistMBID != ""
	if name == "" && q.ArtistMBID != "" {
		err := m.DB.QueryRow(`SELECT name FROM artist WHERE gid = ?`, string(q.ArtistMBID)).Scan(&name)
		if err == sql.ErrNoRows {
			return nil, nil // requested artist unresolvable → no Discogs candidates
		} else if err != nil {
			return nil, err
		}
	}

	var rows *sql.Rows
	var err error
	if requested {
		rows, err = m.DB.Query(
			`SELECT m.id, m.title, m.year
			   FROM dc.master_fts f
			   JOIN dc.master m ON m.id = f.rowid
			   JOIN dc.master_artist ma ON ma.master_id = m.id
			   JOIN dc.artist a ON a.id = ma.artist_id
			  WHERE master_fts MATCH ? AND a.name = ?
			  GROUP BY m.id
			  ORDER BY f.rank
			  LIMIT ?`, ftsTitle(q.Title), name, q.Limit)
	} else {
		rows, err = m.DB.Query(
			`SELECT m.id, m.title, m.year
			   FROM dc.master_fts f
			   JOIN dc.master m ON m.id = f.rowid
			  WHERE master_fts MATCH ?
			  GROUP BY m.id
			  ORDER BY f.rank
			  LIMIT ?`, ftsTitle(q.Title), q.Limit)
	}
	if err != nil {
		return nil, err
	}

	// Drain the cursor before running per-row sub-queries. SQLite ATTACH is
	// per-connection; if we hold the outer rows open while calling back into
	// the pool, a new (unattached) connection may be issued for the inner
	// query. Closing first ensures the attached connection is idle and reused.
	type rawRow struct {
		id    int64
		title string
		year  sql.NullInt64
	}
	var raw []rawRow
	for rows.Next() {
		var r rawRow
		if err := rows.Scan(&r.id, &r.title, &r.year); err != nil {
			rows.Close()
			return nil, err
		}
		raw = append(raw, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	var out []AlbumCandidate
	for _, r := range raw {
		c := AlbumCandidate{
			DiscogsMasterID: core.DiscogsMasterID(r.id),
			Title:           r.title,
			Credits:         core.Credits{{Role: core.RoleArtist, Name: name}},
			Sources:         []core.Source{core.SourceDiscogs},
			Match:           Match{TitleDistance: floatPtr(titleDistance(q.Title, r.title))},
		}
		if name == "" {
			c.Credits = nil
		}
		if requested {
			c.Match.ArtistConfirmed = boolPtr(true)
		}
		if r.year.Valid {
			c.Year = int(r.year.Int64)
			if q.Year != 0 {
				c.Match.YearMatch = boolPtr(int(r.year.Int64) == q.Year)
			}
		}
		styles, err := m.masterStyles(r.id)
		if err != nil {
			return nil, err
		}
		c.Styles = styles
		var n int
		if err := m.DB.QueryRow(
			`SELECT COUNT(*) FROM dc.track t JOIN dc.release r ON r.id = t.release_id
			  WHERE r.master_id = ? AND r.is_main_release = 1`, r.id).Scan(&n); err != nil {
			return nil, err
		}
		c.TrackCount = n
		out = append(out, c)
	}
	return out, nil
}

// masterStyles returns the Discogs master's genres then styles, in seq order.
func (m *MirrorDB) masterStyles(masterID int64) ([]string, error) {
	rows, err := m.DB.Query(
		`SELECT value FROM (
		    SELECT genre AS value, seq, 0 AS kind FROM dc.master_genre WHERE master_id = ?
		    UNION ALL
		    SELECT style AS value, seq, 1 AS kind FROM dc.master_style WHERE master_id = ?
		 ) AS s ORDER BY kind, seq`, masterID, masterID)
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

func (m *MirrorDB) findAlbumMB(q AlbumQuery) ([]AlbumCandidate, error) {
	// Artist filter: explicit MBID (gid→id) else name via FTS. Requested-but-
	// unresolved → no MB candidates (per the 2a decision).
	var artistID int64
	var confirmed, requested bool
	if q.ArtistMBID != "" {
		requested = true
		err := m.DB.QueryRow(`SELECT id FROM artist WHERE gid = ?`, string(q.ArtistMBID)).Scan(&artistID)
		if err == nil {
			confirmed = true
		} else if err != sql.ErrNoRows {
			return nil, err
		}
	} else if q.ArtistName != "" {
		requested = true
		id, ok, err := m.resolveArtistID(q.ArtistName)
		if err != nil {
			return nil, err
		}
		artistID, confirmed = id, ok
	}
	if requested && !confirmed {
		return nil, nil
	}

	var rows *sql.Rows
	var err error
	if confirmed {
		rows, err = m.DB.Query(
			`SELECT rg.gid, rg.name, rg.discogs_master_id, rgm.first_release_date_year
			   FROM release_group_fts f
			   JOIN release_group rg ON rg.id = f.rowid
			   JOIN artist_credit_name acn ON acn.artist_credit = rg.artist_credit
			   LEFT JOIN release_group_meta rgm ON rgm.id = rg.id
			  WHERE release_group_fts MATCH ? AND acn.artist = ?
			  GROUP BY rg.id
			  ORDER BY f.rank
			  LIMIT ?`, ftsTitle(q.Title), artistID, q.Limit)
	} else {
		rows, err = m.DB.Query(
			`SELECT rg.gid, rg.name, rg.discogs_master_id, rgm.first_release_date_year
			   FROM release_group_fts f
			   JOIN release_group rg ON rg.id = f.rowid
			   LEFT JOIN release_group_meta rgm ON rgm.id = rg.id
			  WHERE release_group_fts MATCH ?
			  GROUP BY rg.id
			  ORDER BY f.rank
			  LIMIT ?`, ftsTitle(q.Title), q.Limit)
	}
	if err != nil {
		return nil, err
	}

	// Drain the cursor fully before issuing per-row sub-queries.  With
	// SetMaxOpenConns(1) the single connection cannot serve both an open rows
	// cursor and a concurrent sub-query — drain first to avoid deadlock.
	type rawRGRow struct {
		gid  string
		name string
		dmid sql.NullInt64
		year sql.NullInt64
	}
	var raw []rawRGRow
	for rows.Next() {
		var r rawRGRow
		if err := rows.Scan(&r.gid, &r.name, &r.dmid, &r.year); err != nil {
			rows.Close()
			return nil, err
		}
		raw = append(raw, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	var out []AlbumCandidate
	for _, r := range raw {
		c := AlbumCandidate{
			MBID:    core.MBID(r.gid),
			Title:   r.name,
			Sources: []core.Source{core.SourceMusicBrainz},
			Match:   Match{TitleDistance: floatPtr(titleDistance(q.Title, r.name))},
		}
		if r.dmid.Valid && r.dmid.Int64 != 0 {
			c.DiscogsMasterID = core.DiscogsMasterID(r.dmid.Int64)
		}
		if r.year.Valid && r.year.Int64 != 0 {
			c.Year = int(r.year.Int64)
			if q.Year != 0 {
				c.Match.YearMatch = boolPtr(int(r.year.Int64) == q.Year)
			}
		}
		if confirmed {
			c.Match.ArtistConfirmed = boolPtr(true)
		}
		if names, err := m.albumArtistCredits(r.gid); err != nil {
			return nil, err
		} else {
			c.Credits = names
		}
		if traits, err := m.albumTraits(r.gid); err != nil {
			return nil, err
		} else {
			c.Traits = traits
		}
		out = append(out, c)
	}
	return out, nil
}

// albumArtistCredits returns the release-group's credited artists as RoleArtist
// credits (the primary credits inline; richer role-tagged credits are a later tool).
// The canonical artist name comes from the artist table via the credit chain
// (ANV credited-name nuance is deferred to the album-credits tool).
func (m *MirrorDB) albumArtistCredits(rgGID string) (core.Credits, error) {
	rows, err := m.DB.Query(
		`SELECT a.name
		   FROM release_group rg
		   JOIN artist_credit_name acn ON acn.artist_credit = rg.artist_credit
		   JOIN artist a ON a.id = acn.artist
		  WHERE rg.gid = ?`, rgGID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cs core.Credits
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		cs = append(cs, core.Credit{Role: core.RoleArtist, Name: name})
	}
	return cs, rows.Err()
}

// albumTraits maps the release-group's secondary types to the curation ReleaseTraits.
func (m *MirrorDB) albumTraits(rgGID string) ([]core.ReleaseTrait, error) {
	rows, err := m.DB.Query(
		`SELECT st.name
		   FROM release_group rg
		   JOIN release_group_secondary_type_join j ON j.release_group = rg.id
		   JOIN release_group_secondary_type st ON st.id = j.secondary_type
		  WHERE rg.gid = ?`, rgGID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var traits []core.ReleaseTrait
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		switch strings.ToLower(name) {
		case "compilation":
			traits = append(traits, core.TraitCompilation)
		case "live":
			traits = append(traits, core.TraitLive)
		}
	}
	return traits, rows.Err()
}
