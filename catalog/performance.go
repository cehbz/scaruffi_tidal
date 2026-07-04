package catalog

import (
	"database/sql"
	"sort"
	"strconv"
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
	// MBOnly skips Discogs discovery/corroboration entirely (the composer+performer
	// Discogs path is minutes-scale for a prolific composer); resolution stays on the
	// MB spine and grades at most Medium.
	MBOnly bool
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
	// WorkResolution is WorkGroup.Resolution ("title"|"alias"|"performer-fallback"),
	// threaded through for the curate protocol's mandatory cross-check rule (see
	// CURATE.md) — a performer-fallback resolution can silently land on a
	// same-composer sibling work.
	WorkResolution string `json:"work_resolution,omitempty"`
}

// Performance is a resolved (or candidate) performance: the movement recordings
// satisfying the conjunctive credit key, plus edition attributes and confidence.
type Performance struct {
	Work           WorkRef                `json:"work"`
	Recordings     []PerformanceRecording `json:"recordings"`
	Credits        core.Credits           `json:"matched_credits,omitempty"`
	FirstYear      int                    `json:"first_year,omitempty"`
	RGMBID         core.MBID              `json:"rg_mbid,omitempty"`
	RGTitle        string                 `json:"rg_title,omitempty"`
	RGArtistCredit string                 `json:"rg_artist_credit,omitempty"`
	DiscogsMaster  core.DiscogsMasterID   `json:"discogs_master_id,omitempty"`
	Label          string                 `json:"label,omitempty"`
	LabelID        int64                  `json:"discogs_label_id,omitempty"`
	Catno          string                 `json:"catno,omitempty"`
	Sources        []core.Source          `json:"sources"`
	Confidence     Confidence             `json:"confidence"`

	clusterKey int64 // unexported: the earliest release-group id (cluster identity)
}

// PerformanceResult is the ResolvePerformance return value.
type PerformanceResult struct {
	Outcome      Outcome       `json:"outcome"`
	Performances []Performance `json:"performances"`
	Warnings     []string      `json:"warnings,omitempty"`
}

// ResolvePerformance is the capstone primitive: orchestrate work-group resolution,
// MB+Discogs candidate discovery, cross-source reconciliation, within-block selection
// (year nearest ±2, then label family), and the three-outcome gate. It never
// substitutes a performance or vintage: a selector that excludes everything surfaces
// the unselected set plus a warning.
func (m *MirrorDB) ResolvePerformance(q PerformanceQuery) (PerformanceResult, error) {
	if q.Limit <= 0 {
		q.Limit = 25
	}
	composer := ""
	if names := q.Credits.Names(core.RoleComposer); len(names) > 0 {
		composer = names[0]
	}
	groups, groupWarnings, err := m.resolveWorkGroups(q.Work, composer)
	if err != nil {
		return PerformanceResult{}, err
	}
	if len(groups) == 0 {
		return PerformanceResult{Outcome: OutcomeAbsent, Warnings: groupWarnings}, nil
	}

	// Multi-root candidacy (see resolveWorkGroups' doc comment / rr-task-5-report.md
	// FINDING 1): try each candidate root's performances in order — title-sourced
	// first, then alias-sourced — and use the first one that actually carries a
	// matching performance. g defaults to the title-priority candidate (groups[0])
	// so the "no candidate matched at all" path below still blames/reports the same
	// group it always has when every candidate comes up empty.
	g := groups[0]
	var mb []Performance
	for _, cand := range groups {
		found, err := m.mbPerformances(cand, q)
		if err != nil {
			return PerformanceResult{}, err
		}
		if len(found) > 0 {
			g, mb = cand, found
			break
		}
	}
	var fallbackWarnings []string
	if len(mb) == 0 && len(performerCredits(q.Credits)) > 0 {
		// Title-twin family trap: the title-resolved group holds none of the
		// requested performers' recordings. Rediscover the group from the
		// performer's discography (see workGroupFromPerformers).
		g2, ok, err := m.workGroupFromPerformers(q, composer)
		if err != nil {
			return PerformanceResult{}, err
		}
		if ok && g2.RootID != g.RootID {
			if mb, err = m.mbPerformances(g2, q); err != nil {
				return PerformanceResult{}, err
			}
			if len(mb) > 0 {
				fallbackWarnings = append(fallbackWarnings,
					"work-group "+string(g.RootMBID)+" ("+g.RootName+") had no matching performances; resolved via the performer's discography to "+string(g2.RootMBID)+" ("+g2.RootName+")",
					"work resolved via performer-discography fallback; cross-check the work identity (same-composer sibling works can't be discriminated by this path)")
			}
		}
	}
	// Bridge every performer constraint to a Discogs id ONCE: discogsPerformances
	// (candidate discovery) and reconcile (corroboration grading) both need it, and
	// bridging touches the mirror per credit — a single shared pass, not two.
	armIDs, bridgeWarnings, err := m.bridgedConstraintIDs(q.Credits)
	if err != nil {
		return PerformanceResult{}, err
	}
	var dc []dcPerf
	if !q.MBOnly {
		if dc, err = m.discogsPerformances(q, armIDs); err != nil {
			return PerformanceResult{}, err
		}
	}
	perfs, warnings, err := m.reconcile(mb, dc, q, armIDs, bridgeWarnings)
	warnings = append(append(append([]string{}, groupWarnings...), fallbackWarnings...), warnings...)
	if err != nil {
		return PerformanceResult{}, err
	}
	if len(perfs) == 0 {
		return PerformanceResult{Outcome: OutcomeAbsent, Warnings: warnings}, nil
	}

	// Within-block selection — year (nearest, ±2) then label-family. Selectors NEVER
	// fabricate: a selector that excludes everything surfaces the unselected set + warns.
	selected := perfs
	if q.Year != 0 {
		var within []Performance
		for _, p := range selected {
			if p.FirstYear != 0 && abs(p.FirstYear-q.Year) <= 2 {
				within = append(within, p)
			}
		}
		if len(within) == 0 {
			warnings = append(warnings, "no performance matches year "+strconv.Itoa(q.Year)+" (±2); surfacing all candidates without substituting")
		} else {
			selected = nearestByYear(within, q.Year)
		}
	}
	if q.Label != "" && len(selected) > 1 {
		filtered, err := m.filterByLabel(selected, q.Label, q.Catno)
		if err != nil {
			return PerformanceResult{}, err
		}
		if len(filtered) == 0 {
			warnings = append(warnings, "no performance matches label "+q.Label+"; surfacing candidates without substituting")
		} else {
			selected = filtered
		}
	}

	if len(selected) > q.Limit {
		selected = selected[:q.Limit]
	}

	// Gate: captured iff exactly one performance AND the full requested credit set was
	// matched on it (Sources includes MB, i.e. it carries recordings/ISRCs).
	gateWants, err := m.expandWants(performerCredits(q.Credits))
	if err != nil {
		return PerformanceResult{}, err
	}
	fullCredits := allCreditsMatched(selected, gateWants)
	switch {
	case len(selected) == 1 && fullCredits && len(selected[0].Recordings) > 0:
		return PerformanceResult{Outcome: OutcomeCaptured, Performances: selected, Warnings: warnings}, nil
	default:
		return PerformanceResult{Outcome: OutcomeCandidates, Performances: selected, Warnings: warnings}, nil
	}
}

// nearestByYear keeps only the performance(s) with the minimal |FirstYear-year| gap.
func nearestByYear(ps []Performance, year int) []Performance {
	best := -1
	for _, p := range ps {
		d := abs(p.FirstYear - year)
		if best < 0 || d < best {
			best = d
		}
	}
	var out []Performance
	for _, p := range ps {
		if abs(p.FirstYear-year) == best {
			out = append(out, p)
		}
	}
	return out
}

// filterByLabel keeps performances whose label shares a Discogs label family with
// the requested label: the wanted name is resolved to a Discogs id once, and each
// performance with both ids present (its reconciled LabelID and the resolved want
// id) is kept via m.sameLabelFamily — so a query for "Columbia" keeps a take
// reconciled to its sublabel "CBS". When either id is missing (the wanted name has
// no dc.label row, or the performance carries no reconciled LabelID — MB-only, never
// reconciled with Discogs), the filter falls back to core.NormalizeName equality on
// the label name. catno, when given, must also match casefold. A performance with no
// label at all is dropped by a label selector (it cannot be confirmed).
func (m *MirrorDB) filterByLabel(ps []Performance, label, catno string) ([]Performance, error) {
	wantKey := core.NormalizeName(label)
	wantID, wantOK, err := m.labelIDByName(label)
	if err != nil {
		return nil, err
	}
	var out []Performance
	for _, p := range ps {
		if p.Label == "" {
			continue
		}
		matched := core.NormalizeName(p.Label) == wantKey
		if wantOK && p.LabelID != 0 {
			if matched, err = m.sameLabelFamily(wantID, p.LabelID); err != nil {
				return nil, err
			}
		}
		if !matched {
			continue
		}
		if catno != "" && core.NormalizeName(p.Catno) != core.NormalizeName(catno) {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// allCreditsMatched reports whether every performance in ps has the full requested
// performer credit set (used to decide captured vs candidates).
func allCreditsMatched(ps []Performance, want []wantCredit) bool {
	for _, p := range ps {
		if !creditsSatisfy(p.Credits, want) {
			return false
		}
	}
	return len(ps) > 0
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
	want, err := m.expandWants(performerCredits(q.Credits))
	if err != nil {
		return nil, err
	}
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

	work := WorkRef{MBID: g.RootMBID, Name: g.RootName, Composers: g.Composers, WorkResolution: g.Resolution}
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
		if key > 0 { // a real release-group (not a no-release singleton) → album identity
			if err := m.releaseGroupIdentity(key, &p); err != nil {
				return nil, err
			}
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

// releaseGroupIdentity fills p's RGMBID/RGTitle/RGArtistCredit from the cluster's
// release-group row (the performance's album identity for GM materialization).
func (m *MirrorDB) releaseGroupIdentity(rgID int64, p *Performance) error {
	var gid, name string
	var credit sql.NullString
	err := m.DB.QueryRow(
		`SELECT rg.gid, rg.name,
		        (SELECT GROUP_CONCAT(a.name, ', ')
		           FROM artist_credit_name acn JOIN artist a ON a.id = acn.artist
		          WHERE acn.artist_credit = rg.artist_credit)
		   FROM release_group rg WHERE rg.id = ?`, rgID).Scan(&gid, &name, &credit)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	p.RGMBID = core.MBID(gid)
	p.RGTitle = name
	p.RGArtistCredit = credit.String
	return nil
}

// wantCredit is one requested credit expanded to every name its artist is known
// by (MB primary name + aliases) — a Latin query matches a Cyrillic credit.
type wantCredit struct {
	Role  core.Role
	Names []string
}

// wantsOf lifts plain credits to single-variant wants (no alias expansion).
func wantsOf(cs core.Credits) []wantCredit {
	out := make([]wantCredit, 0, len(cs))
	for _, c := range cs {
		out = append(out, wantCredit{Role: c.Role, Names: []string{c.Name}})
	}
	return out
}

// expandWants lifts credits to alias-expanded wants via the mirror's alias
// table, resolved role-aware (nameVariantsForRole): each credit's own role
// disambiguates same-FTS-rank artist-type ties (e.g. an ensemble "Trio" vs
// "Orchestra"), so the variant set targets the artist the credit actually means.
func (m *MirrorDB) expandWants(cs core.Credits) ([]wantCredit, error) {
	out := make([]wantCredit, 0, len(cs))
	for _, c := range cs {
		names, err := m.nameVariantsForRole(c.Name, c.Role)
		if err != nil {
			return nil, err
		}
		out = append(out, wantCredit{Role: c.Role, Names: names})
	}
	return out, nil
}

// matchesAny reports whether have carries role (or an umbrella'd role) under ANY
// of the requested name variants.
func matchesAny(have core.Credits, role core.Role, names []string) bool {
	for _, n := range names {
		if have.MatchesRole(role, n) {
			return true
		}
		switch role {
		case core.RoleConductor:
			if have.MatchesRole(core.RoleChorusMaster, n) {
				return true
			}
		case core.RoleOrchestra:
			if have.MatchesRole(core.RoleChorus, n) || have.MatchesRole(core.RoleArtist, n) {
				return true
			}
		case core.RoleChorus, core.RoleSoloist:
			if have.MatchesRole(core.RoleArtist, n) {
				return true
			}
		}
	}
	return false
}

// creditsSatisfy reports whether have matches EVERY requested credit (AND), with
// two role umbrellas (identity stays name-exact per variant; only the role widens):
//   - conductor also matches chorus_master (the a-cappella director);
//   - the ensemble/performer roles (orchestra, chorus, soloist) also match the
//     bare credited-artist role, and orchestra additionally matches chorus —
//     the intent cannot know whether MB typed an ensemble's arc as performing
//     orchestra, choir vocals, or left it as the artist credit only.
//
// Empty want → true.
func creditsSatisfy(have core.Credits, want []wantCredit) bool {
	for _, req := range want {
		if !matchesAny(have, req.Role, req.Names) {
			return false
		}
	}
	return true
}

// matchedForces returns the subset of a recording's credits whose role is one of the
// requested roles OR one of their umbrella roles (the same widening creditsSatisfy/
// matchesAny apply when deciding whether a recording qualifies at all) — otherwise a
// recording that only qualified via e.g. the conductor->chorus_master umbrella would
// surface with the very credit that satisfied it missing from the captured set.
func matchedForces(have core.Credits, want []wantCredit) core.Credits {
	if len(want) == 0 {
		return nil
	}
	roles := map[core.Role]bool{}
	for _, w := range want {
		roles[w.Role] = true
		switch w.Role {
		case core.RoleConductor:
			roles[core.RoleChorusMaster] = true
		case core.RoleOrchestra:
			roles[core.RoleChorus] = true
			roles[core.RoleArtist] = true
		case core.RoleChorus, core.RoleSoloist:
			roles[core.RoleArtist] = true
		}
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
// form word(s) + number, not on filler. "no" ("No. 5") is the numbering
// abbreviation's connector word, not a content token; it must not itself
// satisfy albumMatchesWork's shared-word requirement independent of the digit.
var workStop = map[string]bool{"in": true, "of": true, "the": true, "for": true, "a": true, "and": true, "on": true, "de": true, "no": true}

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

// dcCandidate is a (master, credited release) pair from the performer intersection.
type dcCandidate struct {
	MasterID      int64
	ReleaseID     int64 // the credited release (work-group evidence is read here)
	MainReleaseID int64 // label/edition attributes come from the main release
	Year          int
}

// performerIntersectionCandidates finds (master, credited release) pairs whose
// release-level credits contain EVERY arm id — the performers are the album's
// identity, and each arm is an indexed release_artist scan (typical: 10^2–10^3 rows
// per arm, intersection in seconds; measured 274 for Kleiber ∩ VPO). Masterless
// releases are not returned (the primitive's unit is the master).
func (m *MirrorDB) performerIntersectionCandidates(armIDs []int64) ([]dcCandidate, error) {
	if len(armIDs) == 0 {
		return nil, nil
	}
	var b strings.Builder
	args := make([]any, len(armIDs))
	for i, id := range armIDs {
		if i > 0 {
			b.WriteString(" INTERSECT ")
		}
		b.WriteString("SELECT release_id FROM dc.release_artist WHERE artist_id = ?")
		args[i] = id
	}
	rows, err := m.DB.Query(
		`SELECT r.master_id, r.id, m.main_release_id, m.year
		   FROM (`+b.String()+`) c
		   JOIN dc.release r ON r.id = c.release_id
		   JOIN dc.master m ON m.id = r.master_id
		  ORDER BY r.master_id, (r.id = m.main_release_id) DESC, r.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []dcCandidate
	seen := map[int64]bool{}
	for rows.Next() {
		var c dcCandidate
		var year sql.NullInt64
		if err := rows.Scan(&c.MasterID, &c.ReleaseID, &c.MainReleaseID, &year); err != nil {
			return nil, err
		}
		if seen[c.MasterID] {
			continue
		}
		seen[c.MasterID] = true
		if year.Valid {
			c.Year = int(year.Int64)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// dcTrack is a Discogs release track: its title and the ID used to key
// track-level credits.
type dcTrack struct {
	ID    int64
	Title string
}

// dcCredit is a Discogs release_artist/track credit: an artist and its free-text role.
type dcCredit struct {
	ArtistID int64
	Role     string
}

// tracksFor returns the top-level tracks of each release, seq-ordered — one query.
func (m *MirrorDB) tracksFor(releaseIDs []int64) (map[int64][]dcTrack, error) {
	in, args := intInClause(releaseIDs)
	// +parent_track_id disqualifies idx_track_parent (SQLite's documented unary-+
	// mechanism). Its NULL bucket (~120M of 178M rows) is inexpressible in stat1's
	// single per-index average (any ANALYZE records ~4-13 rows/key), so with or
	// without stats the planner costs IS NULL as selective and walks the index
	// (~6 min for 16 releases). Guarded by the integration latency tests.
	rows, err := m.DB.Query(
		`SELECT release_id, id, title FROM dc.track
		  WHERE release_id IN (`+in+`) AND +parent_track_id IS NULL
		  ORDER BY release_id, seq`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]dcTrack{}
	for rows.Next() {
		var rid int64
		var tr dcTrack
		var title sql.NullString
		if err := rows.Scan(&rid, &tr.ID, &title); err != nil {
			return nil, err
		}
		tr.Title = title.String
		out[rid] = append(out[rid], tr)
	}
	return out, rows.Err()
}

// trackArtistsFor returns each track's credits for all tracks of the releases — one query.
func (m *MirrorDB) trackArtistsFor(releaseIDs []int64) (map[int64][]dcCredit, error) {
	in, args := intInClause(releaseIDs)
	rows, err := m.DB.Query(
		`SELECT ta.track_id, ta.artist_id, ta.role
		   FROM dc.track_artist ta JOIN dc.track t ON t.id = ta.track_id
		  WHERE t.release_id IN (`+in+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]dcCredit{}
	for rows.Next() {
		var tid int64
		var c dcCredit
		var role sql.NullString
		if err := rows.Scan(&tid, &c.ArtistID, &role); err != nil {
			return nil, err
		}
		c.Role = role.String
		out[tid] = append(out[tid], c)
	}
	return out, rows.Err()
}

// releaseArtistsFor returns each release's release-level credits — one query.
func (m *MirrorDB) releaseArtistsFor(releaseIDs []int64) (map[int64][]dcCredit, error) {
	in, args := intInClause(releaseIDs)
	rows, err := m.DB.Query(
		`SELECT release_id, artist_id, role FROM dc.release_artist
		  WHERE release_id IN (`+in+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]dcCredit{}
	for rows.Next() {
		var rid int64
		var c dcCredit
		var role sql.NullString
		if err := rows.Scan(&rid, &c.ArtistID, &role); err != nil {
			return nil, err
		}
		c.Role = role.String
		out[rid] = append(out[rid], c)
	}
	return out, rows.Err()
}

// workGroupTracks reconstructs the work from track evidence: the tracks whose titles
// match the work tokens. When no track title matches but the release title does, the
// whole album is the work (movement-titled tracks) and all tracks are the group.
// nil = this album is not this work.
func workGroupTracks(work map[string]bool, releaseTitle string, tracks []dcTrack) []dcTrack {
	var group []dcTrack
	for _, t := range tracks {
		if albumMatchesWork(work, t.Title) {
			group = append(group, t)
		}
	}
	if group != nil {
		return group
	}
	if albumMatchesWork(work, releaseTitle) && len(tracks) > 0 {
		return tracks
	}
	return nil
}

// groupComposerConfirmed ties the composer to the reconstructed work-group: a track-
// level composer credit on a group track confirms; a release-level composer credit
// confirms only when NO group track carries any composer-role credit (the guard that
// rejects a release-level filler credit on a multi-composer album whose matched group
// belongs to another composer).
func groupComposerConfirmed(group []dcTrack, trackCredits map[int64][]dcCredit, releaseCredits []dcCredit, composerID int64) bool {
	groupHasAnyComposer := false
	for _, tr := range group {
		for _, c := range trackCredits[tr.ID] {
			for _, r := range discogsRoles(c.Role) {
				if r != core.RoleComposer {
					continue
				}
				if c.ArtistID == composerID {
					return true
				}
				groupHasAnyComposer = true
			}
		}
	}
	if groupHasAnyComposer {
		return false
	}
	for _, c := range releaseCredits {
		if c.ArtistID != composerID {
			continue
		}
		for _, r := range discogsRoles(c.Role) {
			if r == core.RoleComposer {
				return true
			}
		}
	}
	return false
}

// bridgedConstraintIDs bridges every performer credit (composer excluded — it is
// not a within-block identity constraint) to a Discogs artist id, ONE pass shared by
// discogsPerformances (candidate discovery) and reconcile (corroboration grading) —
// see ResolvePerformance, which calls this once and threads the result to both.
// warnings flags each credit bridged via the lower-confidence name-match fallback.
func (m *MirrorDB) bridgedConstraintIDs(credits core.Credits) (ids []int64, warnings []string, err error) {
	for _, w := range performerCredits(credits) {
		id, viaFallback, ok, err := m.bridgedDiscogsID(w.Name)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			ids = append(ids, id)
			if viaFallback {
				warnings = append(warnings, "discogs bridge for "+w.Name+" via name-match fallback (unlinked or dangling id)")
			}
		}
	}
	return ids, warnings, nil
}

// dcPerf is a Discogs-side performance candidate: a master from the performer-id
// intersection whose reconstructed work-group confirmed the composer.
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

// discogsPerformances finds Discogs masters that are a performance of the work by
// these forces — PERFORMER-DRIVEN: candidates are the releases credited with ALL
// bridged performer-constraint ids (the forces are the album's identity and are far
// more selective than a prolific composer); per candidate, the WORK is reconstructed
// from track evidence and THAT group's composer is confirmed (a release-level filler
// credit on a multi-composer album never false-matches). All candidate reads are
// batched. Returns nil when the composer or every performer lacks a Discogs identity
// (no anchor -> no claim). A composer or performer credited only on releases outside
// the master's credited release is a documented miss (build-time concordance scope).
// armIDs is the caller's already-bridged performer-constraint ids (bridgedConstraintIDs,
// shared with reconcile so a query bridges its performer set once); the composer bridge
// stays local here (discogsPerformances is the only caller that needs it, to bound a
// lone performer arm and to confirm the reconstructed work-group's composer).
func (m *MirrorDB) discogsPerformances(q PerformanceQuery, armIDs []int64) ([]dcPerf, error) {
	composerNames := q.Credits.Names(core.RoleComposer)
	if len(composerNames) == 0 {
		return nil, nil
	}
	composerID, _, ok, err := m.bridgedDiscogsID(composerNames[0])
	if err != nil || !ok {
		return nil, err
	}
	if len(armIDs) == 0 {
		return nil, nil // a performance is identified by its performers
	}
	if len(armIDs) == 1 {
		// A lone orchestra arm can span 10^4+ releases; the composer arm bounds it.
		// Copy rather than append in place: armIDs is shared with reconcile's caller.
		armIDs = append(append([]int64{}, armIDs...), composerID)
	}
	cands, err := m.performerIntersectionCandidates(armIDs)
	if err != nil || len(cands) == 0 {
		return nil, err
	}

	releaseIDs := make([]int64, len(cands))
	for i, c := range cands {
		releaseIDs[i] = c.ReleaseID
	}
	tracks, err := m.tracksFor(releaseIDs)
	if err != nil {
		return nil, err
	}
	trackCredits, err := m.trackArtistsFor(releaseIDs)
	if err != nil {
		return nil, err
	}
	releaseCredits, err := m.releaseArtistsFor(releaseIDs)
	if err != nil {
		return nil, err
	}

	work := significantWorkTokens(q.Work)
	var out []dcPerf
	for _, c := range cands {
		group := workGroupTracks(work, "", tracks[c.ReleaseID])
		if group == nil {
			// Movement-titled tracks: fall back to the release title.
			var title sql.NullString
			if err := m.DB.QueryRow(`SELECT title FROM dc.release WHERE id=?`, c.ReleaseID).Scan(&title); err != nil && err != sql.ErrNoRows {
				return nil, err
			}
			group = workGroupTracks(work, title.String, tracks[c.ReleaseID])
		}
		if group == nil {
			continue
		}
		if !groupComposerConfirmed(group, trackCredits, releaseCredits[c.ReleaseID], composerID) {
			continue
		}
		ids := creditedArtistIDs(releaseCredits[c.ReleaseID], tracks[c.ReleaseID], trackCredits)
		dp := dcPerf{MasterID: c.MasterID, Year: c.Year, ArtistIDs: ids}
		dp.LabelID, dp.Label, dp.Catno, err = m.mainReleaseLabel(c.MainReleaseID)
		if err != nil {
			return nil, err
		}
		out = append(out, dp)
	}
	return out, nil
}

// creditedArtistIDs collects the distinct artist ids credited on the release
// (release-level plus its tracks' track-level).
func creditedArtistIDs(release []dcCredit, tracks []dcTrack, trackCredits map[int64][]dcCredit) []int64 {
	seen := map[int64]bool{}
	var out []int64
	add := func(id int64) {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, c := range release {
		add(c.ArtistID)
	}
	for _, tr := range tracks {
		for _, c := range trackCredits[tr.ID] {
			add(c.ArtistID)
		}
	}
	return out
}

// reconcile matches each MB performance to at most one Discogs candidate: every (mb, dc)
// pair sharing a constraint id is scored by year proximity (known-year pairs by
// |mbYear-dcYear|, hard-capped at ±2; either-year-unknown pairs rank after every
// known-year pair) and assigned greedily over that globally sorted order, each side at
// most once — a year-unknown MB cluster can no longer steal a known-year dc master from
// an exact-match cluster. A match is graded ConfidenceHigh only when EVERY performer
// constraint bridged to a Discogs id (allBridged) AND all of them are corroborated on the
// reconciled Discogs album (constraintIDs ⊆ dc.ArtistIDs); else ConfidenceMedium (work
// confirmed, performers partially corroborated — an unbridged or uncorroborated
// constraint); both carry dual identity (DiscogsMaster/Label/Catno,
// Sources=[MusicBrainz,Discogs]). Unmatched MB performances stay ConfidenceMedium. Unused
// Discogs candidates are appended as Discogs-only ConfidenceLow performances. constraintIDs
// and warnings are the caller's already-bridged performer-constraint ids
// (bridgedConstraintIDs, shared with discogsPerformances so a query bridges its performer
// set once, not twice).
func (m *MirrorDB) reconcile(mb []Performance, dc []dcPerf, q PerformanceQuery, constraintIDs []int64, warnings []string) ([]Performance, []string, error) {
	allBridged := len(constraintIDs) == len(performerCredits(q.Credits))

	// Pairing is globally year-proximate, not first-match-wins: score every (mb, dc)
	// pair that shares a constraint id (known-year pairs by |mbYear-dcYear|, hard-capped
	// at ±2; either-unknown pairs rank after every known-year pair), sort ascending, and
	// assign greedily over that order — each side at most once. This prevents a year-0 MB
	// cluster from stealing a known-year dc master that rightfully belongs to an
	// exact-match cluster considered later in mb's original order.
	type pair struct{ i, j, score int }
	var pairs []pair
	for i := range mb {
		for j := range dc {
			if !sharesAnyID(constraintIDs, dc[j].ArtistIDs) {
				continue
			}
			score := 1000 // either year unknown: after every known-year pair
			if mb[i].FirstYear != 0 && dc[j].Year != 0 {
				d := abs(mb[i].FirstYear - dc[j].Year)
				if d > 2 {
					continue
				}
				score = d
			}
			pairs = append(pairs, pair{i, j, score})
		}
	}
	sort.Slice(pairs, func(a, b int) bool {
		if pairs[a].score != pairs[b].score {
			return pairs[a].score < pairs[b].score
		}
		if pairs[a].i != pairs[b].i {
			return pairs[a].i < pairs[b].i
		}
		return pairs[a].j < pairs[b].j
	})
	usedMB := make([]bool, len(mb))
	used := make([]bool, len(dc))
	for _, p := range pairs {
		if usedMB[p.i] || used[p.j] {
			continue
		}
		usedMB[p.i], used[p.j] = true, true
		d := dc[p.j]
		mb[p.i].DiscogsMaster = core.DiscogsMasterID(d.MasterID)
		mb[p.i].Label = d.Label
		mb[p.i].LabelID = d.LabelID
		mb[p.i].Catno = d.Catno
		mb[p.i].Sources = []core.Source{core.SourceMusicBrainz, core.SourceDiscogs}
		// High requires every performer constraint to be bridged to a Discogs id (not just
		// every bridged id corroborated): a constraint that can't even be checked is not
		// evidence of agreement. subsetInt holds by construction under intersection
		// candidates once allBridged; it stays as an explicit guard.
		if allBridged && subsetInt(constraintIDs, d.ArtistIDs) {
			mb[p.i].Confidence = ConfidenceHigh // all performer constraints corroborated
		} else {
			mb[p.i].Confidence = ConfidenceMedium // work confirmed, performers partial (Discogs gap)
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
