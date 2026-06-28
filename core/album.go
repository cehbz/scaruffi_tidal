package core

// Album is a golden album: the edition-abstract identity of a release-group, plus
// the credits and canonical tracklist that discriminate it. Its minimum identity
// is the release-group (MB release-group MBID and/or Discogs master id); which
// edition to play is a render-time concern, not part of this identity.
type Album struct {
	Credits       Credits        `json:"credits,omitempty"`
	Title         string         `json:"title"`
	IDs           ExternalIDs    `json:"ids"`
	FirstReleased int            `json:"first_released,omitempty"`
	Traits        []ReleaseTrait `json:"traits,omitempty"`
	Styles        []string       `json:"styles,omitempty"`
	Tracklist     []TrackRef     `json:"tracklist,omitempty"`
}

// HasTrait reports whether the album carries the given release trait.
func (a Album) HasTrait(t ReleaseTrait) bool {
	for _, x := range a.Traits {
		if x == t {
			return true
		}
	}
	return false
}
