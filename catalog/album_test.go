package catalog

import (
	"testing"

	"github.com/cehbz/tidalist/core"
)

func TestFindAlbumMBByArtist(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindAlbum(AlbumQuery{Title: "John Barleycorn Must Die", ArtistName: "Traffic", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	var c *AlbumCandidate
	for i := range got {
		if len(got[i].Sources) > 0 && string(got[i].Sources[0]) == "musicbrainz" {
			c = &got[i]
			break
		}
	}
	if c == nil {
		t.Fatalf("expected MB candidate in results (got %d total): %+v", len(got), got)
	}
	if c.MBID != "rg-jbmd" {
		t.Errorf("MBID = %q, want rg-jbmd", c.MBID)
	}
	if c.DiscogsMasterID != 69017 {
		t.Errorf("DiscogsMasterID = %d, want 69017 (the FK)", c.DiscogsMasterID)
	}
	if c.Year != 1970 {
		t.Errorf("Year = %d, want 1970", c.Year)
	}
	if len(c.Sources) != 1 || c.Sources[0] != "musicbrainz" {
		t.Errorf("Sources = %v, want [musicbrainz]", c.Sources)
	}
	if !c.Credits.Has("artist", "Traffic") {
		t.Errorf("expected an artist credit for Traffic; got %v", c.Credits)
	}
	if c.Match.ArtistConfirmed == nil || !*c.Match.ArtistConfirmed {
		t.Error("artist_confirmed should be true")
	}
}

func TestFindAlbumIncludesDiscogsPeer(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindAlbum(AlbumQuery{Title: "John Barleycorn Must Die", ArtistName: "Traffic", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	var mb, dc *AlbumCandidate
	for i := range got {
		switch got[i].Sources[0] {
		case "musicbrainz":
			mb = &got[i]
		case "discogs":
			dc = &got[i]
		}
	}
	if mb == nil || dc == nil {
		t.Fatalf("expected one MB and one Discogs candidate (peers), got %d: %+v", len(got), got)
	}
	if dc.DiscogsMasterID != 69017 {
		t.Errorf("Discogs master id = %d, want 69017", dc.DiscogsMasterID)
	}
	if dc.Year != 1970 {
		t.Errorf("Discogs year = %d, want 1970", dc.Year)
	}
	if dc.TrackCount != 2 {
		t.Errorf("Discogs track_count = %d, want 2 (main release)", dc.TrackCount)
	}
	if len(dc.Styles) == 0 {
		t.Errorf("expected Discogs styles (genres ∪ styles), got none")
	}
	if dc.MBID != "" {
		t.Errorf("Discogs candidate must not carry an MBID (single-source); got %q", dc.MBID)
	}
}

func TestFindAlbumMBTraits(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindAlbum(AlbumQuery{Title: "Best of Traffic", ArtistName: "Traffic", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(got))
	}
	found := false
	for _, tr := range got[0].Traits {
		if tr == "compilation" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected compilation trait, got %v", got[0].Traits)
	}
}

func TestFindAlbumUnresolvedArtistEmpty(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindAlbum(AlbumQuery{Title: "John Barleycorn Must Die", ArtistName: "Nonexistent", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("a requested-but-unresolved artist must yield no MB candidates; got %d", len(got))
	}
}

// bySource returns the first candidate from each source ("" if absent), guarding
// against empty Sources slices.
func bySource(cands []AlbumCandidate) (mb, dc *AlbumCandidate) {
	for i := range cands {
		if len(cands[i].Sources) == 0 {
			continue
		}
		switch cands[i].Sources[0] {
		case core.SourceMusicBrainz:
			mb = &cands[i]
		case core.SourceDiscogs:
			dc = &cands[i]
		}
	}
	return mb, dc
}

// TestFindAlbumByArtistMBID exercises the artist-MBID branch: the MB lookup
// resolves the gid → artist id, and the Discogs lookup resolves the gid → name.
// Both peers must come back, and the MB candidate must be artist-confirmed.
func TestFindAlbumByArtistMBID(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindAlbum(AlbumQuery{Title: "John Barleycorn Must Die", ArtistMBID: "a-traffic", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	mb, dc := bySource(got)
	if mb == nil || dc == nil {
		t.Fatalf("expected one MB and one Discogs candidate (peers), got %d: %+v", len(got), got)
	}
	if mb.MBID != "rg-jbmd" {
		t.Errorf("MB MBID = %q, want rg-jbmd", mb.MBID)
	}
	if dc.DiscogsMasterID != 69017 {
		t.Errorf("Discogs master id = %d, want 69017", dc.DiscogsMasterID)
	}
	if mb.Match.ArtistConfirmed == nil || !*mb.Match.ArtistConfirmed {
		t.Error("MB candidate artist_confirmed should be true")
	}
}

// TestFindAlbumUnresolvedArtistMBIDEmpty: an unknown artist gid resolves in
// neither mirror (MB: gid not found; Discogs: name lookup ErrNoRows), so there
// are no candidates.
func TestFindAlbumUnresolvedArtistMBIDEmpty(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindAlbum(AlbumQuery{Title: "John Barleycorn Must Die", ArtistMBID: "bogus-gid", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("an unresolvable artist MBID must yield no candidates; got %d: %+v", len(got), got)
	}
}

// TestFindAlbumTitleOnly: with no artist requested, both peers come back as
// title-only matches and neither carries an artist_confirmed signal.
func TestFindAlbumTitleOnly(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindAlbum(AlbumQuery{Title: "John Barleycorn Must Die", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	mb, dc := bySource(got)
	if mb == nil || dc == nil {
		t.Fatalf("expected one MB and one Discogs candidate (peers), got %d: %+v", len(got), got)
	}
	if mb.Match.ArtistConfirmed != nil {
		t.Errorf("MB candidate artist_confirmed should be unset (nil); got %v", *mb.Match.ArtistConfirmed)
	}
	if dc.Match.ArtistConfirmed != nil {
		t.Errorf("Discogs candidate artist_confirmed should be unset (nil); got %v", *dc.Match.ArtistConfirmed)
	}
}

func TestAlbumByRG(t *testing.T) {
	m := newTestMirror(t)
	info, ok, err := m.AlbumByRG("rg-jbmd")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("rg-jbmd must resolve")
	}
	if info.MBID != "rg-jbmd" || info.Title != "John Barleycorn Must Die" {
		t.Errorf("identity = %q %q", info.MBID, info.Title)
	}
	if info.Year != 1970 {
		t.Errorf("year = %d, want 1970", info.Year)
	}
	if info.DiscogsMasterID != 69017 {
		t.Errorf("discogs master = %d, want 69017", info.DiscogsMasterID)
	}
	if !info.ArtistCredits.MatchesRole(core.RoleArtist, "Traffic") {
		t.Errorf("artist credits %v missing Traffic", info.ArtistCredits)
	}
	if len(info.Traits) != 0 {
		t.Errorf("JBMD should carry no traits, got %v", info.Traits)
	}

	comp, ok, err := m.AlbumByRG("rg-best")
	if err != nil || !ok {
		t.Fatalf("rg-best must resolve: ok=%v err=%v", ok, err)
	}
	var hasComp bool
	for _, tr := range comp.Traits {
		if tr == core.TraitCompilation {
			hasComp = true
		}
	}
	if !hasComp {
		t.Errorf("rg-best traits %v missing compilation", comp.Traits)
	}

	if _, ok, err := m.AlbumByRG("rg-nope"); err != nil || ok {
		t.Errorf("unknown rg: ok=%v err=%v, want ok=false err=nil", ok, err)
	}
}

// TestAlbumByMaster: the Discogs-only sibling of AlbumByRG, keyed by master id
// instead of rg gid. Backs the materialize-golden branch for selections that
// carry a discogs_master_id but no rg_mbid (the ~5/267 loss this fixes).
func TestAlbumByMaster(t *testing.T) {
	m := newTestMirror(t)
	info, ok, err := m.AlbumByMaster(69017)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("master 69017 must resolve")
	}
	if info.MBID != "" {
		t.Errorf("Discogs-only album must carry no MBID, got %q", info.MBID)
	}
	if info.DiscogsMasterID != 69017 {
		t.Errorf("discogs master = %d, want 69017", info.DiscogsMasterID)
	}
	if info.Title != "John Barleycorn Must Die" {
		t.Errorf("title = %q", info.Title)
	}
	if info.Year != 1970 {
		t.Errorf("year = %d, want 1970", info.Year)
	}
	if !info.ArtistCredits.MatchesRole(core.RoleArtist, "Traffic") {
		t.Errorf("artist credits %v missing Traffic", info.ArtistCredits)
	}
	if len(info.Traits) != 0 {
		t.Errorf("Discogs masters carry no secondary-type facts, got traits %v", info.Traits)
	}

	if _, ok, err := m.AlbumByMaster(999999); err != nil || ok {
		t.Errorf("unknown master: ok=%v err=%v, want ok=false err=nil", ok, err)
	}
}
