package catalog

import (
	"database/sql"
	"sort"
	"strings"
	"unicode"

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

// workStop drops pure connectors so the work match keys on the distinctive
// form word(s) + number, not on filler.
var workStop = map[string]bool{"in": true, "of": true, "the": true, "for": true, "a": true, "and": true, "on": true, "de": true}

// significantWorkTokens lowercases and splits a work title into content tokens
// (letters/digits), dropping connectors.
func significantWorkTokens(s string) map[string]bool {
	out := map[string]bool{}
	for _, t := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if !workStop[t] {
			out[t] = true
		}
	}
	return out
}

func isDigits(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return s != ""
}

// albumMatchesWork reports whether an album's text (title ∪ track titles) is plausibly
// the work. The composer already fixes the composer; this only disambiguates WHICH of
// that composer's works. Rule: share ≥1 non-digit content token (the form/name) AND,
// when the work carries digit tokens (opus/number), share ≥1 of them (so a No. 7 album
// can't match a No. 5 work). Loose by design — a ranker, never a gate.
func albumMatchesWork(work map[string]bool, albumText string) bool {
	if len(work) == 0 {
		return false
	}
	album := significantWorkTokens(albumText)
	sharedWord, sharedDigit := false, false
	workHasDigit := false
	for t := range work {
		if isDigits(t) {
			workHasDigit = true
			if album[t] {
				sharedDigit = true
			}
		} else if album[t] {
			sharedWord = true
		}
	}
	if !sharedWord {
		return false
	}
	return !workHasDigit || sharedDigit
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

// discogsPerformances finds Discogs ALBUMS (masters) that are a performance of the
// work by these forces — ARTIST-FIRST: the composer's bridged id (required) plus ≥1
// performer-constraint bridged id generate candidates via the bridge (no title gate);
// a loose work-token filter keeps the albums that are THIS work. Each candidate carries
// its album artist ids for RW-3 confidence grading. Returns nil when the composer has
// no Discogs identity (no work-identity anchor → no claim).
func (m *MirrorDB) discogsPerformances(q PerformanceQuery) ([]dcPerf, error) {
	// Composer is REQUIRED (it disambiguates same-numbered works across composers).
	composerNames := q.Credits.Names(core.RoleComposer)
	if len(composerNames) == 0 {
		return nil, nil
	}
	composerID, _, ok, err := m.bridgedDiscogsID(composerNames[0])
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	// Performer constraint ids (conductor/orchestra/soloist/chorus).
	var performerIDs []int64
	for _, c := range performerCredits(q.Credits) {
		id, _, ok, err := m.bridgedDiscogsID(c.Name)
		if err != nil {
			return nil, err
		}
		if ok {
			performerIDs = append(performerIDs, id)
		}
	}
	if len(performerIDs) == 0 {
		return nil, nil // a performance is identified by its performers
	}

	// Candidate masters: composer appears AND ≥1 performer appears, credit anywhere on
	// the album (release_artist ∪ track_artist). Two EXISTS over indexed artist_id.
	perfIn, perfArgs := intInClause(performerIDs)
	args := append([]any{composerID, composerID}, perfArgs...)
	args = append(args, perfArgs...)
	rows, err := m.DB.Query(
		`SELECT DISTINCT m.id, m.main_release_id, m.year
		   FROM dc.master m
		   JOIN dc.release r ON r.master_id = m.id
		  WHERE ( EXISTS(SELECT 1 FROM dc.release_artist ra WHERE ra.release_id=r.id AND ra.artist_id=?)
		       OR EXISTS(SELECT 1 FROM dc.track_artist ta JOIN dc.track t ON t.id=ta.track_id WHERE t.release_id=r.id AND ta.artist_id=?) )
		    AND ( EXISTS(SELECT 1 FROM dc.release_artist ra WHERE ra.release_id=r.id AND ra.artist_id IN (`+perfIn+`))
		       OR EXISTS(SELECT 1 FROM dc.track_artist ta JOIN dc.track t ON t.id=ta.track_id WHERE t.release_id=r.id AND ta.artist_id IN (`+perfIn+`)) )`,
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	work := significantWorkTokens(q.Work)
	var out []dcPerf
	for rows.Next() {
		var masterID, mainRelease int64
		var year sql.NullInt64
		if err := rows.Scan(&masterID, &mainRelease, &year); err != nil {
			return nil, err
		}
		// Confirm the composer's role-set includes composer (not merely that the artist
		// appears), then loose work-token filter over title ∪ track titles.
		hasComposer, err := m.albumHasArtistWithRole(mainRelease, composerID, core.RoleComposer)
		if err != nil {
			return nil, err
		}
		if !hasComposer {
			continue
		}
		text, err := m.albumText(mainRelease)
		if err != nil {
			return nil, err
		}
		if !albumMatchesWork(work, text) {
			continue
		}
		ids, err := m.albumArtistIDs(mainRelease)
		if err != nil {
			return nil, err
		}
		dp := dcPerf{MasterID: masterID, ArtistIDs: ids}
		if year.Valid {
			dp.Year = int(year.Int64)
		}
		dp.LabelID, dp.Label, dp.Catno, err = m.mainReleaseLabel(mainRelease)
		if err != nil {
			return nil, err
		}
		out = append(out, dp)
	}
	return out, rows.Err()
}

// albumHasArtistWithRole reports whether artistID is credited on the album (release-
// or track-level) with a role whose parsed role-SET contains want (conductor umbrella:
// a wanted conductor also matches chorus_master).
func (m *MirrorDB) albumHasArtistWithRole(releaseID, artistID int64, want core.Role) (bool, error) {
	rows, err := m.DB.Query(
		`SELECT role FROM dc.release_artist WHERE release_id=? AND artist_id=?
		 UNION ALL
		 SELECT ta.role FROM dc.track_artist ta JOIN dc.track t ON t.id=ta.track_id
		  WHERE t.release_id=? AND ta.artist_id=?`, releaseID, artistID, releaseID, artistID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var role sql.NullString
		if err := rows.Scan(&role); err != nil {
			return false, err
		}
		for _, r := range discogsRoles(role.String) {
			if r == want || (want == core.RoleConductor && r == core.RoleChorusMaster) {
				return true, nil
			}
		}
	}
	return false, rows.Err()
}

// albumArtistIDs returns the distinct artist ids credited on the album (release ∪ track).
func (m *MirrorDB) albumArtistIDs(releaseID int64) ([]int64, error) {
	rows, err := m.DB.Query(
		`SELECT artist_id FROM dc.release_artist WHERE release_id=?
		 UNION
		 SELECT ta.artist_id FROM dc.track_artist ta JOIN dc.track t ON t.id=ta.track_id WHERE t.release_id=?`,
		releaseID, releaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// albumText concatenates the release title and its track titles (the work evidence).
func (m *MirrorDB) albumText(releaseID int64) (string, error) {
	var b strings.Builder
	var title sql.NullString
	if err := m.DB.QueryRow(`SELECT title FROM dc.release WHERE id=?`, releaseID).Scan(&title); err != nil && err != sql.ErrNoRows {
		return "", err
	}
	b.WriteString(title.String)
	rows, err := m.DB.Query(`SELECT title FROM dc.track WHERE release_id=? AND parent_track_id IS NULL`, releaseID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var t sql.NullString
		if err := rows.Scan(&t); err != nil {
			return "", err
		}
		b.WriteByte(' ')
		b.WriteString(t.String)
	}
	return b.String(), rows.Err()
}

// reconcile matches each MB performance to at most one Discogs candidate by crossed
// artist-id overlap (the query's performer-constraint Discogs ids) plus year proximity.
// A match is graded ConfidenceHigh when ALL performer constraints are corroborated on
// the reconciled Discogs album (constraintIDs ⊆ dc.ArtistIDs), else ConfidenceMedium
// (work confirmed, performers partially corroborated — a Discogs credit gap); both carry
// dual identity (DiscogsMaster/Label/Catno, Sources=[MusicBrainz,Discogs]). Unmatched MB
// performances stay ConfidenceMedium. Unused Discogs candidates are appended as
// Discogs-only ConfidenceLow performances.
func (m *MirrorDB) reconcile(mb []Performance, dc []dcPerf, q PerformanceQuery) ([]Performance, []string, error) {
	// The query's performer-constraint Discogs ids (shared across all MB performances of
	// this query).
	var constraintIDs []int64
	var warnings []string
	for _, w := range performerCredits(q.Credits) {
		id, viaFallback, ok, err := m.bridgedDiscogsID(w.Name)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			constraintIDs = append(constraintIDs, id)
			if viaFallback {
				warnings = append(warnings, "discogs bridge for "+w.Name+" via name-match fallback (unlinked or dangling id)")
			}
		}
	}

	used := make([]bool, len(dc))
	for i := range mb {
		best := -1
		for j := range dc {
			if used[j] {
				continue
			}
			if !sharesAnyID(constraintIDs, dc[j].ArtistIDs) {
				continue
			}
			if mb[i].FirstYear != 0 && dc[j].Year != 0 && abs(mb[i].FirstYear-dc[j].Year) > 2 {
				continue
			}
			best = j
			break
		}
		if best < 0 {
			continue
		}
		d := dc[best]
		used[best] = true
		mb[i].DiscogsMaster = core.DiscogsMasterID(d.MasterID)
		mb[i].Label = d.Label
		mb[i].Catno = d.Catno
		mb[i].Sources = []core.Source{core.SourceMusicBrainz, core.SourceDiscogs}
		if subsetInt(constraintIDs, d.ArtistIDs) {
			mb[i].Confidence = ConfidenceHigh // all performer constraints corroborated
		} else {
			mb[i].Confidence = ConfidenceMedium // work confirmed, performers partial (Discogs gap)
		}
	}

	// Discogs-only performances (no MB agreement) → surfaced at low confidence.
	for j := range dc {
		if used[j] {
			continue
		}
		mb = append(mb, Performance{
			Work:          WorkRef{}, // no MB work identity on a Discogs-only candidate
			Credits:       dc[j].Credits,
			FirstYear:     dc[j].Year,
			DiscogsMaster: core.DiscogsMasterID(dc[j].MasterID),
			Label:         dc[j].Label,
			Catno:         dc[j].Catno,
			Sources:       []core.Source{core.SourceDiscogs},
			Confidence:    ConfidenceLow,
		})
	}
	return mb, warnings, nil
}

// sharesAnyID reports whether a and b have at least one int64 in common.
func sharesAnyID(a, b []int64) bool {
	set := map[int64]bool{}
	for _, x := range a {
		set[x] = true
	}
	for _, y := range b {
		if set[y] {
			return true
		}
	}
	return false
}

// subsetInt reports whether every id in need appears in have.
func subsetInt(need, have []int64) bool {
	set := make(map[int64]bool, len(have))
	for _, x := range have {
		set[x] = true
	}
	for _, x := range need {
		if !set[x] {
			return false
		}
	}
	return len(need) > 0
}

// abs returns the absolute value of x.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
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
