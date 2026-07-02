// Package curate is the deterministic half of the curate stage: it materializes
// LLM-chosen identities (a selections document) into the durable Golden Master,
// judging each entry against the type-aware criteria, and emits the curate report
// (dispositions, violations, unverifiable flags, considered alternatives).
//
// The GM is emitted in the Python golden schema (src/tidalist/core/spec.py), the
// contract the render stage consumes: entry kind word "track" (not "recording"),
// criteria tags not_live/not_compilation (not the Go no_* vocabulary), and
// performed_by carries "artist" (not "name").
package curate

import (
	"encoding/json"
	"fmt"

	"github.com/cehbz/tidalist/catalog"
	"github.com/cehbz/tidalist/core"
)

// GoldenDoc is the durable Golden Master document (Python golden schema).
type GoldenDoc struct {
	Name    string            `json:"name"`
	Brief   briefJSON         `json:"brief"`
	Entries []json.RawMessage `json:"entries"`
}

type briefJSON struct {
	Criteria []json.RawMessage `json:"criteria"`
}

// Report is the curate-stage report: the user-facing record of what was resolved,
// what was rejected and why, what could not be verified, and the alternatives
// considered per item.
type Report struct {
	Name  string       `json:"name"`
	Items []ReportItem `json:"items"`
}

// ReportItem is one intent item's curate outcome.
type ReportItem struct {
	Index        int               `json:"index"`
	Kind         string            `json:"kind"`
	ID           string            `json:"id,omitempty"`
	Note         string            `json:"note,omitempty"`
	Disposition  string            `json:"disposition"` // resolved | absent
	Admitted     bool              `json:"admitted"`
	Violations   []string          `json:"violations,omitempty"`
	Unverifiable []string          `json:"unverifiable,omitempty"`
	Marginal     string            `json:"marginal,omitempty"`
	Alternatives []json.RawMessage `json:"alternatives,omitempty"`
}

// Selections is the LLM-authored curate hand-off: the chosen identity per intent
// item, plus judgment metadata (marginal notes, considered alternatives).
type Selections struct {
	Name  string
	Brief []core.Criterion
	Items []Selection
}

// Selection is one chosen identity. Artist/Title are the reviewable-stub identity
// used only when the ids resolve to nothing (the absent case).
type Selection struct {
	Kind          string
	RGMBID        string
	DiscogsMaster int64
	RecordingMBID string
	Artist        string
	Title         string
	Criteria      []core.Criterion
	Edition       *editionJSON
	Provenance    provenanceJSON
	Marginal      string
	Alternatives  []json.RawMessage
}

type editionJSON struct {
	Markers        []string `json:"markers,omitempty"`
	PreferOriginal *bool    `json:"prefer_original,omitempty"`
}

type provenanceJSON struct {
	Source string `json:"source"`
	Note   string `json:"note,omitempty"`
}

// --- selections parsing -------------------------------------------------------

type selectionsWire struct {
	Name  string `json:"name"`
	Brief struct {
		Criteria []json.RawMessage `json:"criteria"`
	} `json:"brief"`
	Selections []selectionWire `json:"selections"`
}

type selectionWire struct {
	Kind          string            `json:"kind"`
	RGMBID        string            `json:"rg_mbid,omitempty"`
	DiscogsMaster int64             `json:"discogs_master_id,omitempty"`
	RecordingMBID string            `json:"recording_mbid,omitempty"`
	Artist        string            `json:"artist,omitempty"`
	Title         string            `json:"title,omitempty"`
	Criteria      []json.RawMessage `json:"criteria,omitempty"`
	Edition       *editionJSON      `json:"edition,omitempty"`
	Provenance    provenanceJSON    `json:"provenance"`
	Marginal      string            `json:"marginal,omitempty"`
	Alternatives  []json.RawMessage `json:"alternatives,omitempty"`
}

// parseCriterion accepts both the Go tags (no_live, no_compilation,
// performed_by{name}) and the Python tags (not_live, not_compilation,
// performed_by{artist}); the closed union is validated by tag, never eval'd.
func parseCriterion(raw json.RawMessage) (core.Criterion, error) {
	var j struct {
		Type   string `json:"type"`
		Name   string `json:"name,omitempty"`
		Artist string `json:"artist,omitempty"`
	}
	if err := json.Unmarshal(raw, &j); err != nil {
		return nil, err
	}
	switch j.Type {
	case "studio":
		return core.Studio{}, nil
	case "no_compilation", "not_compilation":
		return core.NoCompilation{}, nil
	case "no_live", "not_live":
		return core.NoLive{}, nil
	case "performed_by":
		name := j.Name
		if name == "" {
			name = j.Artist
		}
		if name == "" {
			return nil, fmt.Errorf("performed_by needs a name")
		}
		return core.PerformedBy{Name: name}, nil
	default:
		return nil, fmt.Errorf("unknown criterion type: %q", j.Type)
	}
}

func parseCriteria(raws []json.RawMessage) ([]core.Criterion, error) {
	out := make([]core.Criterion, 0, len(raws))
	for _, r := range raws {
		c, err := parseCriterion(r)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// ParseSelections reads a selections JSON document, accepting both the Go and the
// Python criterion tag vocabularies.
func ParseSelections(data []byte) (Selections, error) {
	var w selectionsWire
	if err := json.Unmarshal(data, &w); err != nil {
		return Selections{}, err
	}
	if w.Name == "" {
		return Selections{}, fmt.Errorf("selections document needs a name")
	}
	sel := Selections{Name: w.Name}
	var err error
	if sel.Brief, err = parseCriteria(w.Brief.Criteria); err != nil {
		return Selections{}, fmt.Errorf("brief: %w", err)
	}
	for i, sw := range w.Selections {
		if sw.Kind != "album" && sw.Kind != "track" {
			return Selections{}, fmt.Errorf("selection[%d]: kind must be album or track, got %q", i, sw.Kind)
		}
		s := Selection{
			Kind:          sw.Kind,
			RGMBID:        sw.RGMBID,
			DiscogsMaster: sw.DiscogsMaster,
			RecordingMBID: sw.RecordingMBID,
			Artist:        sw.Artist,
			Title:         sw.Title,
			Edition:       sw.Edition,
			Provenance:    sw.Provenance,
			Marginal:      sw.Marginal,
			Alternatives:  sw.Alternatives,
		}
		if s.Criteria, err = parseCriteria(sw.Criteria); err != nil {
			return Selections{}, fmt.Errorf("selection[%d]: %w", i, err)
		}
		sel.Items = append(sel.Items, s)
	}
	return sel, nil
}

// --- Python-schema emission ----------------------------------------------------

// emitCriterion writes a criterion in the Python golden vocabulary.
func emitCriterion(c core.Criterion) (json.RawMessage, error) {
	switch x := c.(type) {
	case core.Studio:
		return json.Marshal(map[string]string{"type": "studio"})
	case core.NoCompilation:
		return json.Marshal(map[string]string{"type": "not_compilation"})
	case core.NoLive:
		return json.Marshal(map[string]string{"type": "not_live"})
	case core.PerformedBy:
		return json.Marshal(map[string]string{"type": "performed_by", "artist": x.Name})
	default:
		return nil, fmt.Errorf("unserializable criterion: %T", c)
	}
}

func emitCriteria(cs []core.Criterion) ([]json.RawMessage, error) {
	out := make([]json.RawMessage, 0, len(cs))
	for _, c := range cs {
		r, err := emitCriterion(c)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

type verdictJSON struct {
	Admitted   bool     `json:"admitted"`
	Violations []string `json:"violations"`
}

type editionOut struct {
	Markers        []string `json:"markers"`
	PreferOriginal bool     `json:"prefer_original"`
}

type trackRefJSON struct {
	Position  int    `json:"position"`
	Title     string `json:"title"`
	ISRC      string `json:"isrc,omitempty"`
	MBID      string `json:"mbid,omitempty"`
	DurationS int    `json:"duration_s,omitempty"`
}

type albumEntryJSON struct {
	Kind            string         `json:"kind"`
	MBID            string         `json:"mbid,omitempty"`
	Artist          string         `json:"artist"`
	Title           string         `json:"title"`
	Year            int            `json:"year,omitempty"`
	Traits          []string       `json:"traits"`
	Tracklist       []trackRefJSON `json:"tracklist"`
	DiscogsMasterID int64          `json:"discogs_master_id,omitempty"`
	Sources         []string       `json:"sources,omitempty"`
	Provenance      provenanceJSON `json:"provenance"`
	Verdict         verdictJSON    `json:"verdict"`
	Edition         *editionOut    `json:"edition,omitempty"`
}

type creditJSON struct {
	Artist string `json:"artist"`
	Role   string `json:"role"`
}

type trackEntryJSON struct {
	Kind        string         `json:"kind"`
	MBID        string         `json:"mbid,omitempty"`
	ISRC        string         `json:"isrc,omitempty"`
	Artist      string         `json:"artist"`
	Title       string         `json:"title"`
	Album       string         `json:"album,omitempty"`
	Year        int            `json:"year,omitempty"`
	DurationS   int            `json:"duration_s,omitempty"`
	Performance string         `json:"performance"`
	Credits     []creditJSON   `json:"credits"`
	Provenance  provenanceJSON `json:"provenance"`
	Verdict     verdictJSON    `json:"verdict"`
	Edition     *editionOut    `json:"edition,omitempty"`
}

func editionOf(e *editionJSON) *editionOut {
	if e == nil {
		return nil
	}
	out := &editionOut{Markers: e.Markers, PreferOriginal: true}
	if out.Markers == nil {
		out.Markers = []string{}
	}
	if e.PreferOriginal != nil {
		out.PreferOriginal = *e.PreferOriginal
	}
	return out
}

func verdictOf(v core.Verdict) verdictJSON {
	out := verdictJSON{Admitted: v.Admitted, Violations: v.Violations}
	if out.Violations == nil {
		out.Violations = []string{}
	}
	return out
}

// --- materialization -----------------------------------------------------------

// Materialize resolves every selection against the mirrors, judges it, and emits
// the GoldenDoc plus the Report. Absent identities yield rejected, reviewable
// entries — never silent drops.
func Materialize(m *catalog.MirrorDB, sel Selections) (GoldenDoc, Report, error) {
	doc := GoldenDoc{Name: sel.Name}
	rep := Report{Name: sel.Name}
	briefRaw, err := emitCriteria(sel.Brief)
	if err != nil {
		return GoldenDoc{}, Report{}, err
	}
	doc.Brief = briefJSON{Criteria: briefRaw}

	for i, s := range sel.Items {
		criteria := append(append([]core.Criterion{}, sel.Brief...), s.Criteria...)
		var entry json.RawMessage
		var item ReportItem
		switch s.Kind {
		case "album":
			entry, item, err = m2AlbumEntry(m, s, criteria)
		case "track":
			entry, item, err = m2TrackEntry(m, s, criteria)
		}
		if err != nil {
			return GoldenDoc{}, Report{}, fmt.Errorf("selection[%d]: %w", i, err)
		}
		item.Index = i
		item.Kind = s.Kind
		item.Note = s.Provenance.Note
		item.Marginal = s.Marginal
		item.Alternatives = s.Alternatives
		doc.Entries = append(doc.Entries, entry)
		rep.Items = append(rep.Items, item)
	}
	return doc, rep, nil
}

// m2AlbumEntry materializes an album selection: identity, artist credit, vintage,
// traits, and the canonical tracklist, judged by the album-grain criteria.
func m2AlbumEntry(m *catalog.MirrorDB, s Selection, criteria []core.Criterion) (json.RawMessage, ReportItem, error) {
	info, ok, err := m.AlbumByRG(s.RGMBID)
	if err != nil {
		return nil, ReportItem{}, err
	}
	if s.RGMBID == "" || !ok {
		return absentEntry(s, "no album found")
	}

	tracks, err := m.TracklistByReleaseGroup(s.RGMBID)
	if err != nil {
		return nil, ReportItem{}, err
	}
	tl := make([]trackRefJSON, 0, len(tracks))
	for _, t := range tracks {
		tl = append(tl, trackRefJSON{Position: t.Position, Title: t.Title,
			ISRC: string(t.ISRC), MBID: string(t.MBID), DurationS: t.DurationS})
	}

	album := core.Album{
		Credits:       info.ArtistCredits,
		Title:         info.Title,
		FirstReleased: info.Year,
		Traits:        info.Traits,
	}
	verdict := core.Judge(album, criteria)

	dmid := int64(info.DiscogsMasterID)
	if s.DiscogsMaster != 0 {
		dmid = s.DiscogsMaster // the LLM's reconciled master wins over the FK hint
	}
	traits := make([]string, 0, len(info.Traits))
	for _, t := range info.Traits {
		traits = append(traits, string(t))
	}
	entry := albumEntryJSON{
		Kind:            "album",
		MBID:            s.RGMBID,
		Artist:          creditNames(info.ArtistCredits),
		Title:           info.Title,
		Year:            info.Year,
		Traits:          traits,
		Tracklist:       tl,
		DiscogsMasterID: dmid,
		Sources:         []string{"musicbrainz"},
		Provenance:      s.Provenance,
		Verdict:         verdictOf(verdict),
		Edition:         editionOf(s.Edition),
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return nil, ReportItem{}, err
	}
	return raw, ReportItem{
		ID:          s.RGMBID,
		Disposition: "resolved",
		Admitted:    verdict.Admitted,
		Violations:  verdict.Violations,
	}, nil
}

// m2TrackEntry materializes a track selection: recording identity + credits,
// judged by the recording-grain criteria; unknown performance under a studio
// criterion admits with an unverifiable flag (permissive-flagged posture).
func m2TrackEntry(m *catalog.MirrorDB, s Selection, criteria []core.Criterion) (json.RawMessage, ReportItem, error) {
	info, ok, err := m.RecordingByGID(s.RecordingMBID)
	if err != nil {
		return nil, ReportItem{}, err
	}
	if s.RecordingMBID == "" || !ok {
		return absentEntry(s, "no recording found")
	}

	rec := core.Recording{
		Credits:     info.Credits,
		Title:       info.Title,
		MBID:        info.MBID,
		ISRC:        info.ISRC,
		DurationS:   info.DurationS,
		Performance: core.PerfUnknown,
	}
	verdict := core.Judge(rec, criteria)
	unverifiable := unverifiableFlags(rec, criteria)

	credits := make([]creditJSON, 0, len(info.Credits))
	for _, c := range info.Credits {
		credits = append(credits, creditJSON{Artist: c.Name, Role: string(c.Role)})
	}
	entry := trackEntryJSON{
		Kind:        "track",
		MBID:        string(info.MBID),
		ISRC:        string(info.ISRC),
		Artist:      info.ArtistCredit,
		Title:       info.Title,
		Album:       info.Album,
		Year:        info.Year,
		DurationS:   info.DurationS,
		Performance: string(rec.Performance),
		Credits:     credits,
		Provenance:  s.Provenance,
		Verdict:     verdictOf(verdict),
		Edition:     editionOf(s.Edition),
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return nil, ReportItem{}, err
	}
	return raw, ReportItem{
		ID:           s.RecordingMBID,
		Disposition:  "resolved",
		Admitted:     verdict.Admitted,
		Violations:   verdict.Violations,
		Unverifiable: unverifiable,
	}, nil
}

// unverifiableFlags reports the criteria that could not actually be judged against
// observed facts: an unobservable fact admits (permissive) but is flagged.
func unverifiableFlags(r core.Recording, criteria []core.Criterion) []string {
	var out []string
	for _, c := range criteria {
		switch x := c.(type) {
		case core.Studio:
			if r.Performance == core.PerfUnknown {
				out = append(out, "studio: performance unobserved")
			}
		case core.PerformedBy:
			if len(r.Credits) == 0 {
				out = append(out, fmt.Sprintf("performed_by %s: no credits observed", x.Name))
			}
		}
	}
	return out
}

// absentEntry emits the reviewable rejected stub for an identity that resolved to
// nothing (mirrors the Python Curator's no-album/no-recording entries).
func absentEntry(s Selection, reason string) (json.RawMessage, ReportItem, error) {
	verdict := verdictJSON{Admitted: false, Violations: []string{reason}}
	var raw json.RawMessage
	var err error
	if s.Kind == "album" {
		raw, err = json.Marshal(albumEntryJSON{
			Kind: "album", Artist: s.Artist, Title: s.Title,
			Traits: []string{}, Tracklist: []trackRefJSON{},
			Provenance: s.Provenance, Verdict: verdict, Edition: editionOf(s.Edition),
		})
	} else {
		raw, err = json.Marshal(trackEntryJSON{
			Kind: "track", Artist: s.Artist, Title: s.Title,
			Performance: string(core.PerfUnknown), Credits: []creditJSON{},
			Provenance: s.Provenance, Verdict: verdict, Edition: editionOf(s.Edition),
		})
	}
	if err != nil {
		return nil, ReportItem{}, err
	}
	return raw, ReportItem{Disposition: "absent", Admitted: false, Violations: []string{reason}}, nil
}

// creditNames joins credit names into the display artist string.
func creditNames(cs core.Credits) string {
	s := ""
	for i, c := range cs {
		if i > 0 {
			s += ", "
		}
		s += c.Name
	}
	return s
}
