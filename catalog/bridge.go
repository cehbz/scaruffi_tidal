package catalog

import (
	"database/sql"

	"github.com/cehbz/tidalist/core"
)

// discogsArtistID returns the MB artist's pre-materialised discogs_artist_id (the
// l_artist_url 180 bridge column). ok=false when the column is NULL/0.
func (m *MirrorDB) discogsArtistID(mbArtistID int64) (int64, bool, error) {
	var id sql.NullInt64
	err := m.DB.QueryRow(`SELECT discogs_artist_id FROM artist WHERE id = ?`, mbArtistID).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if !id.Valid || id.Int64 == 0 {
		return 0, false, nil
	}
	return id.Int64, true, nil
}

// dcArtistIDByName is the DQ fallback for the unlinked residual / dangling bridge
// ids: an ASCII-normalised name match against dc.artist.name. Folds the key, not the
// identity (returns the real Discogs id). Ambiguous (>1 distinct id) → ok=false.
func (m *MirrorDB) dcArtistIDByName(name string) (int64, bool, error) {
	key := core.NormalizeName(name)
	if key == "" {
		return 0, false, nil
	}
	rows, err := m.DB.Query(`SELECT id, name FROM dc.artist`)
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()
	var found int64
	var n int
	for rows.Next() {
		var id int64
		var nm string
		if err := rows.Scan(&id, &nm); err != nil {
			return 0, false, err
		}
		if core.NormalizeName(nm) == key {
			if id != found {
				n++
				found = id
			}
		}
	}
	if err := rows.Err(); err != nil {
		return 0, false, err
	}
	if n != 1 {
		return 0, false, nil // absent or ambiguous
	}
	return found, true, nil
}

// bridgedDiscogsID resolves a credit name to a Discogs artist id: the MB bridge
// column first (resolve the MB artist by FTS), else the name fallback. The second
// return reports whether the fallback path was used (a lower-confidence signal).
func (m *MirrorDB) bridgedDiscogsID(name string) (id int64, viaFallback, ok bool, err error) {
	if mbID, ok, err := m.resolveArtistID(name); err != nil {
		return 0, false, false, err
	} else if ok {
		if dcID, ok, err := m.discogsArtistID(mbID); err != nil {
			return 0, false, false, err
		} else if ok {
			return dcID, false, true, nil
		}
	}
	dcID, ok, err := m.dcArtistIDByName(name)
	if err != nil {
		return 0, false, false, err
	}
	return dcID, true, ok, nil
}
