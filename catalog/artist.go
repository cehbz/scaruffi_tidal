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

// resolveArtistID returns the artist.id for a name: the FTS top hit, else an
// artist_alias exact match. The alias fallback is what finds artists whose MB
// primary name is in another script (e.g. Валерий Гергиев for "Valery Gergiev").
func (m *MirrorDB) resolveArtistID(name string) (int64, bool, error) {
	var id int64
	err := m.DB.QueryRow(
		`SELECT rowid FROM artist_fts WHERE artist_fts MATCH ? ORDER BY rank LIMIT 1`,
		escapeFTS(name)).Scan(&id)
	if err == nil {
		return id, true, nil
	}
	if err != sql.ErrNoRows {
		return 0, false, err
	}
	err = m.DB.QueryRow(
		`SELECT artist FROM artist_alias WHERE name = ? COLLATE NOCASE LIMIT 1`, name).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// nameVariants returns every name the requested artist is known by (primary name
// + all aliases), always including the queried form itself. Credit matching
// accepts any variant, so a Latin query matches a Cyrillic-credited artist.
func (m *MirrorDB) nameVariants(name string) ([]string, error) {
	out := []string{name}
	id, ok, err := m.resolveArtistID(name)
	if err != nil || !ok {
		return out, err
	}
	var primary string
	if err := m.DB.QueryRow(`SELECT name FROM artist WHERE id = ?`, id).Scan(&primary); err == nil && primary != "" {
		out = append(out, primary)
	} else if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	rows, err := m.DB.Query(`SELECT name FROM artist_alias WHERE artist = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
