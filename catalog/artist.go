package catalog

import (
	"database/sql"
	"math"
	"sort"

	"github.com/cehbz/tidalist/core"
)

// artistTypePerson and artistTypeGroup are artist.type values used alongside
// artistTypeOrchestra and artistTypeChoir (catalog/credits.go) to rank credit-role
// type preference in roleTypePreference below.
const (
	artistTypePerson    = 1
	artistTypeGroup     = 2
	artistTypeOrchestra = 5
)

// roleTypePreference ranks artist.type by fit for a credit role — a rank, never a
// filter: every listed type is still a candidate, an unlisted (or NULL, surfaced
// as 0) type just sorts after all of them. Roles absent from this map (producer,
// engineer, mastering, …) have no type preference — typeRank returns 0 for every
// type, so ranking falls through to FTS rank unchanged.
var roleTypePreference = map[core.Role][]int64{
	core.RoleOrchestra:    {artistTypeOrchestra, artistTypeGroup, artistTypeChoir},
	core.RoleChorus:       {artistTypeChoir, artistTypeGroup, artistTypeOrchestra},
	core.RoleChorusMaster: {artistTypeChoir, artistTypeGroup, artistTypeOrchestra},
	core.RoleConductor:    {artistTypePerson, artistTypeGroup},
	core.RoleSoloist:      {artistTypePerson, artistTypeGroup},
	core.RoleComposer:     {artistTypePerson, artistTypeGroup},
	core.RoleArtist:       {artistTypePerson, artistTypeGroup},
}

// typeRank returns atype's preference tier for role: the index into the role's
// preference list (0 = best), or len(list) — worse than every listed type — when
// atype is unlisted or NULL (0). A role with no roleTypePreference entry ranks
// every type in tier 0 (no discrimination).
func typeRank(role core.Role, atype int64) int {
	prefs, ok := roleTypePreference[role]
	if !ok {
		return 0
	}
	for i, t := range prefs {
		if t == atype {
			return i
		}
	}
	return len(prefs)
}

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

// artistIDCand is one artist-resolution candidate with ranking evidence.
type artistIDCand struct {
	ID         int64
	Type       int64 // artist.type; 0 when NULL
	ExactAlias bool  // name matched artist.name or an artist_alias row exactly (NOCASE)
	FTSRank    float64
}

// noFTSRank is the FTSRank sentinel for a candidate sourced only from the exact-
// match query (not present in the FTS top-10): worse than any real bm25 rank, so
// it only breaks ties among multiple exact-alias candidates, never among the
// FTS-observed ones.
const noFTSRank = math.MaxFloat64

// artistIDCandidates returns merged candidates: FTS top-10 (joined to artist.type)
// ∪ exact primary-name/alias matches (COLLATE NOCASE). This is the merge point for
// the alias-shadowing defect: an artist whose MB primary name shares no tokens
// with the query (e.g. a German primary name queried by its English alias) never
// appears in the FTS arm at all, so the exact-match arm is what surfaces it.
func (m *MirrorDB) artistIDCandidates(name string) ([]artistIDCand, error) {
	cands := make(map[int64]*artistIDCand)

	ftsRows, err := m.DB.Query(
		`SELECT a.id, a.type, f.rank
		   FROM artist_fts f
		   JOIN artist a ON a.id = f.rowid
		  WHERE artist_fts MATCH ?
		  ORDER BY f.rank LIMIT 10`, escapeFTS(name))
	if err != nil {
		return nil, err
	}
	for ftsRows.Next() {
		var id int64
		var atype sql.NullInt64
		var rank float64
		if err := ftsRows.Scan(&id, &atype, &rank); err != nil {
			ftsRows.Close()
			return nil, err
		}
		cands[id] = &artistIDCand{ID: id, Type: atype.Int64, FTSRank: rank}
	}
	ftsRows.Close()
	if err := ftsRows.Err(); err != nil {
		return nil, err
	}

	exactRows, err := m.DB.Query(
		`SELECT a.id, a.type FROM artist a WHERE a.name = ?1 COLLATE NOCASE
		 UNION
		 SELECT a.id, a.type FROM artist a
		   JOIN artist_alias al ON al.artist = a.id
		  WHERE al.name = ?1 COLLATE NOCASE`, name)
	if err != nil {
		return nil, err
	}
	for exactRows.Next() {
		var id int64
		var atype sql.NullInt64
		if err := exactRows.Scan(&id, &atype); err != nil {
			exactRows.Close()
			return nil, err
		}
		if c, ok := cands[id]; ok {
			c.ExactAlias = true
		} else {
			cands[id] = &artistIDCand{ID: id, Type: atype.Int64, ExactAlias: true, FTSRank: noFTSRank}
		}
	}
	exactRows.Close()
	if err := exactRows.Err(); err != nil {
		return nil, err
	}

	out := make([]artistIDCand, 0, len(cands))
	for _, c := range cands {
		out = append(out, *c)
	}
	return out, nil
}

// rankArtistCands orders candidates in place, best first: exact-alias before any
// FTS-only hit; then (when role != "") role-compatible artist type via typeRank;
// then FTS rank ascending; ID ascending is the final deterministic tiebreaker (map
// iteration order over artistIDCandidates' result is otherwise unspecified).
func rankArtistCands(cands []artistIDCand, role core.Role) {
	sort.Slice(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.ExactAlias != b.ExactAlias {
			return a.ExactAlias
		}
		if role != "" {
			if ra, rb := typeRank(role, a.Type), typeRank(role, b.Type); ra != rb {
				return ra < rb
			}
		}
		if a.FTSRank != b.FTSRank {
			return a.FTSRank < b.FTSRank
		}
		return a.ID < b.ID
	})
}

// resolveArtistID returns the artist.id for a name: merged FTS+alias candidates
// (artistIDCandidates), exact-alias first, then FTS rank — role-less ranking. The
// alias arm is what finds artists whose MB primary name is in another script (e.g.
// Валерий Гергиев for "Valery Gergiev") or another language (e.g. Berliner
// Philharmoniker for "Berlin Philharmonic"), and now applies regardless of whether
// FTS also returned candidates (previously alias only fired on an FTS miss).
func (m *MirrorDB) resolveArtistID(name string) (int64, bool, error) {
	cands, err := m.artistIDCandidates(name)
	if err != nil {
		return 0, false, err
	}
	if len(cands) == 0 {
		return 0, false, nil
	}
	rankArtistCands(cands, "")
	return cands[0].ID, true, nil
}

// resolveArtistIDForRole ranks candidates for a credit role: exact-alias first,
// then role-compatible artist type (roleTypePreference), then FTS rank. Use this
// over resolveArtistID wherever the caller knows the credit role — e.g. an
// "orchestra" credit should prefer an artist typed Orchestra over a same-ranked
// FTS hit typed Group, but a "soloist" credit has no such preference.
func (m *MirrorDB) resolveArtistIDForRole(name string, role core.Role) (int64, bool, error) {
	cands, err := m.artistIDCandidates(name)
	if err != nil {
		return 0, false, err
	}
	if len(cands) == 0 {
		return 0, false, nil
	}
	rankArtistCands(cands, role)
	return cands[0].ID, true, nil
}

// nameVariants returns every name the requested artist is known by (primary name
// + all aliases), always including the queried form itself. Credit matching
// accepts any variant, so a Latin query matches a Cyrillic-credited artist.
// Role-less: see nameVariantsForRole for the role-aware form.
func (m *MirrorDB) nameVariants(name string) ([]string, error) {
	return m.nameVariantsForRole(name, "")
}

// nameVariantsForRole is nameVariants with role-aware underlying resolution
// (resolveArtistIDForRole instead of resolveArtistID): when the caller knows the
// credit role, a same-FTS-rank ambiguity between artist types (e.g. an ensemble
// "Trio" vs "Orchestra" tied on bm25) resolves toward the role-compatible type,
// so the variant set (and therefore credit matching) targets the right artist.
// role == "" behaves exactly as resolveArtistID (typeRank no-ops on an empty
// role), so nameVariants above is a thin call-through, not a fork.
func (m *MirrorDB) nameVariantsForRole(name string, role core.Role) ([]string, error) {
	out := []string{name}
	id, ok, err := m.resolveArtistIDForRole(name, role)
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
