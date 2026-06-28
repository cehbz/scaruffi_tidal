// Package core holds tidalist's platform-agnostic golden value objects: the
// abstract identity of albums and recordings, independent of any backend.
package core

type MBID string
type ISRC string
type DiscogsMasterID int64
type DiscogsReleaseID int64
type Source string

const (
	SourceMusicBrainz Source = "musicbrainz"
	SourceDiscogs     Source = "discogs"
)

type ExternalIDs struct {
	MBID             MBID             `json:"mbid,omitempty"`
	DiscogsMasterID  DiscogsMasterID  `json:"discogs_master_id,omitempty"`
	DiscogsReleaseID DiscogsReleaseID `json:"discogs_release_id,omitempty"`
	Sources          []Source         `json:"sources,omitempty"`
}

// Empty reports whether no catalog identity is present.
func (e ExternalIDs) Empty() bool {
	return e.MBID == "" && e.DiscogsMasterID == 0 && e.DiscogsReleaseID == 0
}
