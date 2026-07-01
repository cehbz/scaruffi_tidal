package catalog

import (
	"database/sql"
	"sort"

	"github.com/cehbz/tidalist/core"
)

// Confidence is the exactness-gate rung: high = cross-source agreement, medium =
// MB-only with the full requested credit set, low = partial / single-source.
type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

// Outcome is the three-outcome gate: never two.
type Outcome string

const (
	OutcomeCaptured   Outcome = "captured"
	OutcomeCandidates Outcome = "candidates"
	OutcomeAbsent     Outcome = "absent"
)

// PerformanceQuery is a decomposed classical item to resolve. Credits carries the
// composer (work disambiguation) plus the conjunctive conductor/orchestra/soloist/
// chorus identity constraints. Year/Label/Catno are within-block selectors.
type PerformanceQuery struct {
	Work    string
	Credits core.Credits
	Year    int
	Label   string
	Catno   string
	Limit   int
}

// PerformanceRecording is one movement recording of a performance.
type PerformanceRecording struct {
	MBID      core.MBID `json:"mbid,omitempty"`
	ISRC      core.ISRC `json:"isrc,omitempty"`
	Title     string    `json:"title"`
	DurationS int       `json:"duration_s,omitempty"`
}

// WorkRef is the resolved work-group identity.
type WorkRef struct {
	MBID      core.MBID `json:"mbid,omitempty"`
	Name      string    `json:"name"`
	Composers []string  `json:"composers,omitempty"`
}

// Performance is a resolved (or candidate) performance: the movement recordings
// satisfying the conjunctive credit key, plus edition attributes and confidence.
type Performance struct {
	Work          WorkRef                `json:"work"`
	Recordings    []PerformanceRecording `json:"recordings"`
	Credits       core.Credits           `json:"matched_credits,omitempty"`
	FirstYear     int                    `json:"first_year,omitempty"`
	DiscogsMaster core.DiscogsMasterID   `json:"discogs_master_id,omitempty"`
	Label         string                 `json:"label,omitempty"`
	Catno         string                 `json:"catno,omitempty"`
	Sources       []core.Source          `json:"sources"`
	Confidence    Confidence             `json:"confidence"`

	clusterKey int64 // unexported: the earliest release-group id (cluster identity)
}

// PerformanceResult is the ResolvePerformance return value.
type PerformanceResult struct {
	Outcome      Outcome       `json:"outcome"`
	Performances []Performance `json:"performances"`
	Warnings     []string      `json:"warnings,omitempty"`
}

// mbPerformances returns the work-group's recordings satisfying the conjunctive
// credit AND, clustered by earliest co-release into performances.
func (m *MirrorDB) mbPerformances(g WorkGroup, q PerformanceQuery) ([]Performance, error) {
	inClause, args := intInClause(g.WorkIDs)
	rows, err := m.DB.Query(
		`SELECT DISTINCT r.id, r.gid, r.name, r.length, GROUP_CONCAT(i.isrc, ', ') AS isrcs
		   FROM l_recording_work lrw
		   JOIN link l ON l.id = lrw.link
		   JOIN recording r ON r.id = lrw.entity0
		   LEFT JOIN isrc i ON i.recording = r.id
		  WHERE lrw.entity1 IN (`+inClause+`) AND l.link_type = ?
		  GROUP BY r.id
		  ORDER BY r.id`, append(append([]any{}, args...), linkTypePerformance)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type rec struct {
		id     int64
		cand   PerformanceRecording
		credit core.Credits
	}
	var recs []rec
	for rows.Next() {
		var id int64
		var gid, name string
		var length sql.NullInt64
		var isrcs sql.NullString
		if err := rows.Scan(&id, &gid, &name, &length, &isrcs); err != nil {
			return nil, err
		}
		pr := PerformanceRecording{MBID: core.MBID(gid), ISRC: core.ISRC(firstISRC(isrcs.String)), Title: name}
		if length.Valid {
			pr.DurationS = int(length.Int64 / 1000)
		}
		credits, err := m.recordingCredits(id)
		if err != nil {
			return nil, err
		}
		recs = append(recs, rec{id: id, cand: pr, credit: credits})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Conjunctive credit AND (performer roles only; conductor umbrella via the shared helper).
	want := performerCredits(q.Credits)
	var kept []rec
	for _, r := range recs {
		if creditsSatisfy(r.credit, want) {
			kept = append(kept, r)
		}
	}
	if len(kept) == 0 {
		return nil, nil
	}

	// Cluster by earliest release-group. Each recording is assigned to its earliest
	// (min first-release year, then min rg id) release-group; recordings sharing that
	// key co-released first → one performance. Reissues/compilations that are NOT the
	// earliest release don't merge distinct performances.
	clusters := map[int64][]rec{}
	years := map[int64]int{}
	for _, r := range kept {
		rgID, year, ok, err := m.earliestReleaseGroup(r.id)
		if err != nil {
			return nil, err
		}
		if !ok {
			// A recording with no release is its own singleton cluster (keyed by -id).
			rgID, year = -r.id, 0
		}
		clusters[rgID] = append(clusters[rgID], r)
		years[rgID] = year
	}

	work := WorkRef{MBID: g.RootMBID, Name: g.RootName, Composers: g.Composers}
	var out []Performance
	for key, members := range clusters {
		sort.Slice(members, func(i, j int) bool { return members[i].id < members[j].id })
		p := Performance{
			Work:       work,
			FirstYear:  years[key],
			Sources:    []core.Source{core.SourceMusicBrainz},
			Confidence: ConfidenceMedium,
			clusterKey: key,
			Credits:    matchedForces(members[0].credit, want),
		}
		for _, r := range members {
			p.Recordings = append(p.Recordings, r.cand)
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FirstYear != out[j].FirstYear {
			return out[i].FirstYear < out[j].FirstYear
		}
		return out[i].clusterKey < out[j].clusterKey
	})
	return out, nil
}

// creditsSatisfy reports whether have matches EVERY requested credit (AND), with the
// conductor→chorus_master directing umbrella. Empty want → true.
func creditsSatisfy(have, want core.Credits) bool {
	for _, req := range want {
		if have.MatchesRole(req.Role, req.Name) {
			continue
		}
		if req.Role == core.RoleConductor && have.MatchesRole(core.RoleChorusMaster, req.Name) {
			continue
		}
		return false
	}
	return true
}

// matchedForces returns the subset of a recording's credits whose role is one of the
// requested roles (the "matched credit set" captured on the performance).
func matchedForces(have, want core.Credits) core.Credits {
	if len(want) == 0 {
		return nil
	}
	roles := map[core.Role]bool{}
	for _, w := range want {
		roles[w.Role] = true
	}
	var out core.Credits
	for _, c := range have {
		if roles[c.Role] {
			out = append(out, c)
		}
	}
	return out
}

// dcPerf is a Discogs-side performance candidate: a master whose track/title matches
// the work and whose release_artist roles satisfy the bridged credit AND.
type dcPerf struct {
	MasterID    int64
	Year        int
	LabelID     int64
	Label       string
	Catno       string
	ArtistIDs   []int64
	Credits     core.Credits
	viaFallback bool
}

// discogsPerformances returns Discogs masters matching the work title whose main
// release satisfies EVERY requested performer credit via the crossed-artist-id
// bridge (role-normalised). year/label/catno ride along as within-block attributes.
func (m *MirrorDB) discogsPerformances(q PerformanceQuery) ([]dcPerf, error) {
	want := performerCredits(q.Credits)

	// Resolve each requested credit to a Discogs artist id via the bridge.
	var wantIDs []creditID
	fallbackUsed := false
	for _, w := range want {
		id, viaFallback, ok, err := m.bridgedDiscogsID(w.Name)
		if err != nil {
			return nil, err
		}
		if !ok {
			// A requested force with no Discogs identity → no Discogs performance can
			// satisfy the AND; return nothing (MB may still resolve).
			return nil, nil
		}
		fallbackUsed = fallbackUsed || viaFallback
		wantIDs = append(wantIDs, creditID{role: w.Role, id: id})
	}

	rows, err := m.DB.Query(
		`SELECT m.id, m.main_release_id, m.year
		   FROM dc.master_fts f
		   JOIN dc.master m ON m.id = f.rowid
		  WHERE master_fts MATCH ?
		  ORDER BY f.rank
		  LIMIT ?`, ftsTitle(q.Work), max(q.Limit, 25))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []dcPerf
	for rows.Next() {
		var masterID, mainRelease int64
		var year sql.NullInt64
		if err := rows.Scan(&masterID, &mainRelease, &year); err != nil {
			return nil, err
		}
		ra, err := m.releaseArtists(mainRelease)
		if err != nil {
			return nil, err
		}
		if !releaseSatisfies(ra, wantIDs) {
			continue
		}
		dp := dcPerf{MasterID: masterID, viaFallback: fallbackUsed}
		if year.Valid {
			dp.Year = int(year.Int64)
		}
		for _, a := range ra {
			dp.ArtistIDs = append(dp.ArtistIDs, a.id)
			if r := discogsRole(a.role); r != "" {
				dp.Credits = append(dp.Credits, core.Credit{Role: r, Name: a.name})
			}
		}
		dp.LabelID, dp.Label, dp.Catno, err = m.mainReleaseLabel(mainRelease)
		if err != nil {
			return nil, err
		}
		out = append(out, dp)
	}
	return out, rows.Err()
}

type dcReleaseArtist struct {
	id   int64
	name string
	role string
}

func (m *MirrorDB) releaseArtists(releaseID int64) ([]dcReleaseArtist, error) {
	rows, err := m.DB.Query(
		`SELECT ra.artist_id, COALESCE(a.name,''), COALESCE(ra.role,'')
		   FROM dc.release_artist ra
		   LEFT JOIN dc.artist a ON a.id = ra.artist_id
		  WHERE ra.release_id = ?
		  ORDER BY ra.seq`, releaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []dcReleaseArtist
	for rows.Next() {
		var ra dcReleaseArtist
		if err := rows.Scan(&ra.id, &ra.name, &ra.role); err != nil {
			return nil, err
		}
		out = append(out, ra)
	}
	return out, rows.Err()
}

// creditID is a requested (role, Discogs-artist-id) constraint.
type creditID struct {
	role core.Role
	id   int64
}

// releaseSatisfies reports whether every requested (role,id) is present among the
// release's artists with a role-compatible credit (conductor umbrella included).
func releaseSatisfies(ra []dcReleaseArtist, want []creditID) bool {
	for _, w := range want {
		ok := false
		for _, a := range ra {
			if a.id != w.id {
				continue
			}
			r := discogsRole(a.role)
			if r == w.role || (w.role == core.RoleConductor && r == core.RoleChorusMaster) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

// mainReleaseLabel returns the first label (id, name, catno) of the release.
func (m *MirrorDB) mainReleaseLabel(releaseID int64) (int64, string, string, error) {
	var id sql.NullInt64
	var name, catno sql.NullString
	err := m.DB.QueryRow(
		`SELECT label_id, name, catno FROM dc.release_label
		  WHERE release_id = ? ORDER BY seq LIMIT 1`, releaseID).Scan(&id, &name, &catno)
	if err == sql.ErrNoRows {
		return 0, "", "", nil
	}
	if err != nil {
		return 0, "", "", err
	}
	return id.Int64, name.String, catno.String, nil
}

// earliestReleaseGroup returns the recording's earliest release-group id and its
// first-release year (min year, then min rg id). ok=false when the recording is on
// no release-group.
func (m *MirrorDB) earliestReleaseGroup(recordingID int64) (rgID int64, year int, ok bool, err error) {
	var y sql.NullInt64
	err = m.DB.QueryRow(
		`SELECT rg.id, rgm.first_release_date_year
		   FROM track t
		   JOIN medium md ON md.id = t.medium
		   JOIN release rel ON rel.id = md.release
		   JOIN release_group rg ON rg.id = rel.release_group
		   LEFT JOIN release_group_meta rgm ON rgm.id = rg.id
		  WHERE t.recording = ?
		  ORDER BY COALESCE(rgm.first_release_date_year, 999999) ASC, rg.id ASC
		  LIMIT 1`, recordingID).Scan(&rgID, &y)
	if err == sql.ErrNoRows {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	if y.Valid {
		year = int(y.Int64)
	}
	return rgID, year, true, nil
}
