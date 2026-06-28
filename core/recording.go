package core

// RenderingPreference is an optional, intent-expressed preference for which
// rendering of a take to use (e.g. a Steven Wilson remix, a MoFi master). Absent
// by default — most recordings carry no rendering preference.
type RenderingPreference struct {
	Markers []string `json:"markers,omitempty"`
}

// RenderingVariant is one materialized candidate rendering of a take, carried in
// the Golden Master so render needs no catalog read. It is a remix child (linked
// to the take by MB's "remix of" arc) or a remaster/source variant (the same
// recording on a different release), tagged with identifying markers.
type RenderingVariant struct {
	MBID    MBID     `json:"mbid,omitempty"`
	ISRC    ISRC     `json:"isrc,omitempty"`
	Markers []string `json:"markers,omitempty"`
}

// Recording is a golden recording: the edition-abstract identity of a take (MB
// recording MBID + ISRC when present), its credits, and its performance kind.
// Rendering and Renderings are populated only when the intent expressed a
// rendering preference, keeping the common recording small.
type Recording struct {
	Credits     Credits              `json:"credits,omitempty"`
	Title       string               `json:"title"`
	MBID        MBID                 `json:"mbid,omitempty"`
	ISRC        ISRC                 `json:"isrc,omitempty"`
	DurationS   int                  `json:"duration_s,omitempty"`
	Performance Performance          `json:"performance,omitempty"`
	Rendering   *RenderingPreference `json:"rendering,omitempty"`
	Renderings  []RenderingVariant   `json:"renderings,omitempty"`
}
