package catalog

import (
	"database/sql"

	"github.com/cehbz/tidalist/core"
)

// ArtistCandidate is a ranked artist resolution result.
type ArtistCandidate struct {
	MBID           core.MBID `json:"mbid,omitempty"`
	Name           string    `json:"name"`
	Disambiguation string    `json:"disambiguation,omitempty"`
	Match          Match     `json:"match"`
}

// ResolveArtist returns ranked artist candidates for a name (FTS over artist_fts).
func (m *MirrorDB) ResolveArtist(name string, limit int) ([]ArtistCandidate, error) {
	rows, err := m.DB.Query(
		`SELECT a.gid, a.name, a.comment, f.rank
		   FROM artist_fts f
		   JOIN artist a ON a.id = f.rowid
		  WHERE artist_fts MATCH ?
		  ORDER BY f.rank
		  LIMIT ?`, escapeFTS(name), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ArtistCandidate
	for rows.Next() {
		var gid, nm, comment string
		var rank float64
		if err := rows.Scan(&gid, &nm, &comment, &rank); err != nil {
			return nil, err
		}
		r := rank
		out = append(out, ArtistCandidate{
			MBID: core.MBID(gid), Name: nm, Disambiguation: comment,
			Match: Match{FTSRank: &r},
		})
	}
	return out, rows.Err()
}

// resolveArtistID returns the artist.id of the FTS top hit for name.
func (m *MirrorDB) resolveArtistID(name string) (int64, bool, error) {
	var id int64
	err := m.DB.QueryRow(
		`SELECT rowid FROM artist_fts WHERE artist_fts MATCH ? ORDER BY rank LIMIT 1`,
		escapeFTS(name)).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}
