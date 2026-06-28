package catalog

import "strings"

// Match carries the structured signals a candidate matched on; each tool sets only
// the applicable fields (pointers, omitempty), per the grammar spec. No conflated score.
type Match struct {
	FTSRank         *float64 `json:"fts_rank,omitempty"`
	ArtistConfirmed *bool    `json:"artist_confirmed,omitempty"`
	MBIDExact       *bool    `json:"mbid_exact,omitempty"`
	ISRCExact       *bool    `json:"isrc_exact,omitempty"`
	TitleDistance   *float64 `json:"title_distance,omitempty"`
	YearMatch       *bool    `json:"year_match,omitempty"`
}

func boolPtr(b bool) *bool        { return &b }
func floatPtr(f float64) *float64 { return &f }

func tokenSet(s string) map[string]bool {
	set := map[string]bool{}
	for _, tok := range strings.Fields(strings.ToLower(s)) {
		set[tok] = true
	}
	return set
}

// titleDistance is the token-set Jaccard distance between two titles (lowercased,
// whitespace-split): 1 - |A∩B| / |A∪B|. 0 = identical token sets, 1 = disjoint.
func titleDistance(a, b string) float64 {
	sa, sb := tokenSet(a), tokenSet(b)
	if len(sa) == 0 && len(sb) == 0 {
		return 0
	}
	inter := 0
	for tok := range sa {
		if sb[tok] {
			inter++
		}
	}
	union := len(sa) + len(sb) - inter
	if union == 0 {
		return 0
	}
	return 1 - float64(inter)/float64(union)
}
