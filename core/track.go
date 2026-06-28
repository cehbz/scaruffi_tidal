package core

type ReleaseTrait string

const (
	TraitCompilation ReleaseTrait = "compilation"
	TraitLive        ReleaseTrait = "live"
)

// Performance distinguishes takes. Studio vs live is an identity difference, not
// a rendering one: a live take is a different recording from the studio take.
type Performance string

const (
	PerfStudio  Performance = "studio"
	PerfLive    Performance = "live"
	PerfUnknown Performance = "unknown"
)

// TrackRef is one position in an album's canonical tracklist. DurationS is 0 when
// unobserved.
type TrackRef struct {
	Position  int    `json:"position"`
	Title     string `json:"title"`
	MBID      MBID   `json:"mbid,omitempty"`
	ISRC      ISRC   `json:"isrc,omitempty"`
	DurationS int    `json:"duration_s,omitempty"`
}
