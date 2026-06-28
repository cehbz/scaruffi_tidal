package core

// Kind discriminates the two first-class golden units. Both are fully modeled;
// albums are more frequent but recordings are not an exception.
type Kind string

const (
	KindAlbum     Kind = "album"
	KindRecording Kind = "recording"
)

// GoldenItem is the golden unit: an Album or a Recording. Type-aware behavior
// (criteria, fidelity facets) dispatches on the concrete type.
type GoldenItem interface {
	Kind() Kind
}

func (Album) Kind() Kind     { return KindAlbum }
func (Recording) Kind() Kind { return KindRecording }
