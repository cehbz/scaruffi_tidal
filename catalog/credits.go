package catalog

import (
	"database/sql"
	"strings"

	"github.com/cehbz/tidalist/core"
)

// performer link-types (verified): 156 performer, 148 instrument, 149 vocal,
// 150 performing orchestra, 151 conductor, 152 chorus master. 152 maps to its own
// chorus_master role (the choir trainer / standalone a-cappella director); the
// --credit conductor: umbrella (in the filter) reconciles it for "who directed".
const (
	ltPerformer    = 156
	ltInstrument   = 148
	ltVocal        = 149
	ltOrchestra    = 150
	ltConductor    = 151
	ltChorusMaster = 152
)

const artistTypeChoir = 6 // artist.type for a Choir

// link_attribute_type magic numbers (verified): root buckets and the choir leaf.
const (
	rootInstrument  = 14 // link_attribute_type.root for instruments
	rootVocal       = 3  // link_attribute_type.root for vocals
	attrChoirVocals = 13 // link_attribute_type.id for "choir vocals" (root 3)
)

func (m *MirrorDB) recordingCredits(recordingID int64) (core.Credits, error) {
	var cs core.Credits
	// 1) credited artist(s) -> RoleArtist
	arts, err := m.DB.Query(
		`SELECT a.name
		   FROM recording r
		   JOIN artist_credit_name acn ON acn.artist_credit = r.artist_credit
		   JOIN artist a ON a.id = acn.artist
		  WHERE r.id = ?`, recordingID)
	if err != nil {
		return nil, err
	}
	for arts.Next() {
		var n string
		if err := arts.Scan(&n); err != nil {
			arts.Close()
			return nil, err
		}
		cs = append(cs, core.Credit{Role: core.RoleArtist, Name: n})
	}
	arts.Close()
	if err := arts.Err(); err != nil {
		return nil, err
	}
	// 2) performer relationships -> mapped roles
	rows, err := m.DB.Query(
		`SELECT lt.id,
		        COALESCE(NULLIF(lar.entity0_credit,''), a.name) AS name,
		        a.type,
		        GROUP_CONCAT(CASE WHEN lat.root IN (?,?) THEN lat.name END) AS instrument,
		        MAX(CASE WHEN la.attribute_type = ? THEN 1 ELSE 0 END) AS is_choir
		   FROM l_artist_recording lar
		   JOIN link l ON l.id = lar.link
		   JOIN link_type lt ON lt.id = l.link_type
		   JOIN artist a ON a.id = lar.entity0
		   LEFT JOIN link_attribute la ON la.link = l.id
		   LEFT JOIN link_attribute_type lat ON lat.id = la.attribute_type
		  WHERE lar.entity1 = ? AND l.link_type IN (?,?,?,?,?,?)
		  GROUP BY lar.id
		  ORDER BY lar.id`,
		rootInstrument, rootVocal, attrChoirVocals,
		recordingID, ltPerformer, ltInstrument, ltVocal, ltOrchestra, ltConductor, ltChorusMaster)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var lt int
		var name string
		var atype sql.NullInt64
		var instrument sql.NullString
		var isChoir int
		if err := rows.Scan(&lt, &name, &atype, &instrument, &isChoir); err != nil {
			return nil, err
		}
		c := core.Credit{Name: name}
		switch lt {
		case ltConductor:
			c.Role = core.RoleConductor
		case ltChorusMaster:
			c.Role = core.RoleChorusMaster
		case ltOrchestra:
			c.Role = core.RoleOrchestra
		case ltVocal:
			if isChoir == 1 || (atype.Valid && atype.Int64 == artistTypeChoir) {
				c.Role = core.RoleChorus
			} else {
				c.Role = core.RoleSoloist
			}
		case ltInstrument, ltPerformer:
			c.Role = core.RoleSoloist
		}
		if inst := strings.TrimSpace(instrument.String); inst != "" && c.Role == core.RoleSoloist {
			c.Attrs = map[string]string{"instrument": inst}
		}
		cs = append(cs, c)
	}
	return cs, rows.Err()
}

// ReleaseGroupCredits aggregates a release-group's role-tagged album credits: (a)
// the RG's artist_credit_name names as RoleArtist credits, in credit order; (b) the
// distinct performer arcs (recordingCredits' link-type role mapping — conductor,
// orchestra, chorus, chorus_master, soloist — excluding its recording-level
// RoleArtist rows, which are the recording's own artist credit, not a performer
// arc) aggregated over the recordings of the RG's canonical tracklist
// (TracklistByReleaseGroup — the same canonical-release resolution the golden-
// master tracklist uses; no second canonical-release path). Deduped by (role,
// core.NormalizeName(name)); order is first-seen: the RG artist credits, then each
// canonical track's performer arcs in tracklist order. Unknown rgGID yields an
// empty result, not an error (consistent with albumArtistCredits/
// TracklistByReleaseGroup).
func (m *MirrorDB) ReleaseGroupCredits(rgGID core.MBID) (core.Credits, error) {
	var out core.Credits
	seen := make(map[string]bool)
	add := func(c core.Credit) {
		key := string(c.Role) + "\x00" + core.NormalizeName(c.Name)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, c)
	}

	artistCredits, err := m.albumArtistCredits(string(rgGID))
	if err != nil {
		return nil, err
	}
	for _, c := range artistCredits {
		add(c)
	}

	tracks, err := m.TracklistByReleaseGroup(string(rgGID))
	if err != nil {
		return nil, err
	}
	for _, t := range tracks {
		if t.MBID == "" {
			continue
		}
		recID, ok, err := m.recordingIDByGID(string(t.MBID))
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		cs, err := m.recordingCredits(recID)
		if err != nil {
			return nil, err
		}
		for _, c := range cs {
			if c.Role == core.RoleArtist {
				continue // the recording's own artist credit, not a performer arc
			}
			add(c)
		}
	}
	return out, nil
}

// recordingIDByGID resolves a recording's internal id by gid. ok=false when unknown.
func (m *MirrorDB) recordingIDByGID(gid string) (int64, bool, error) {
	var id int64
	err := m.DB.QueryRow(`SELECT id FROM recording WHERE gid = ?`, gid).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}
