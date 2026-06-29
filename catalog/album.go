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
	return m.findAlbumMB(q)
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
	defer rows.Close()

	var out []AlbumCandidate
	for rows.Next() {
		var gid, name string
		var dmid sql.NullInt64
		var year sql.NullInt64
		if err := rows.Scan(&gid, &name, &dmid, &year); err != nil {
			return nil, err
		}
		c := AlbumCandidate{
			MBID:    core.MBID(gid),
			Title:   name,
			Sources: []core.Source{core.SourceMusicBrainz},
			Match:   Match{TitleDistance: floatPtr(titleDistance(q.Title, name))},
		}
		if dmid.Valid && dmid.Int64 != 0 {
			c.DiscogsMasterID = core.DiscogsMasterID(dmid.Int64)
		}
		if year.Valid && year.Int64 != 0 {
			c.Year = int(year.Int64)
			if q.Year != 0 {
				c.Match.YearMatch = boolPtr(int(year.Int64) == q.Year)
			}
		}
		if confirmed {
			c.Match.ArtistConfirmed = boolPtr(true)
		}
		if names, err := m.albumArtistCredits(gid); err != nil {
			return nil, err
		} else {
			c.Credits = names
		}
		if traits, err := m.albumTraits(gid); err != nil {
			return nil, err
		} else {
			c.Traits = traits
		}
		out = append(out, c)
	}
	return out, rows.Err()
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
