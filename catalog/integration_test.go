//go:build integration

package catalog

// Integration smoke tests for the catalog package.
//
// These tests open the real local MusicBrainz + Discogs SQLite mirrors and
// exercise one query per catalog tool, verifying that the ported SQL matches
// the real schema.  They are excluded from the normal `go test ./...` run and
// must be invoked explicitly:
//
//	go test -tags integration ./catalog/ -run Integration -v
//
// The test skips itself on any machine that lacks the mirror files.
//
// Anchors (verified live 2026-06-29):
//   - Traffic MB gid:            9fadfba9-ecae-4383-a4d8-47b043cea19a
//   - John Barleycorn Must Die   MB release-group gid: 3770d5ce-e0e1-3389-9acf-cd38f0722baf
//   - John Barleycorn Must Die   Discogs master id: 69017 (main release 583800)
//   - Recording "Glad" (Traffic) MB gid: 53bb54ac-1020-4cd4-83ce-362a58e1ec17, ISRC: GBUM71030667
//   - Work "Dear Mr. Fantasy"    MB gid: fa1ba832-28bd-3b2c-ad70-4f32f0e65b21 (125 recordings, composers: Steve Winwood, Chris Wood)
//   - JBMD canonical tracklist:  6 tracks; position 1 = "Glad"

import (
	"os"
	"testing"

	"github.com/cehbz/tidalist/core"
)

const (
	defaultMBPath = "/Volumes/Crucial X10/musicbrainz/musicbrainz.db"
	defaultDCPath = "/Volumes/Crucial X10/discogs/discogs.db"

	trafficGID = "9fadfba9-ecae-4383-a4d8-47b043cea19a"
	jbmdRGGID  = "3770d5ce-e0e1-3389-9acf-cd38f0722baf"
	jbmdDCMID  = int64(69017)

	gladRecordingGID = "53bb54ac-1020-4cd4-83ce-362a58e1ec17"

	dmfWorkGID = "fa1ba832-28bd-3b2c-ad70-4f32f0e65b21"
)

// openRealMirror opens the real mirrors, skipping the test if either file is absent.
func openRealMirror(t *testing.T) *MirrorDB {
	t.Helper()
	mbPath := os.Getenv("TIDALIST_MUSICBRAINZ_DB")
	if mbPath == "" {
		mbPath = defaultMBPath
	}
	dcPath := os.Getenv("TIDALIST_DISCOGS_DB")
	if dcPath == "" {
		dcPath = defaultDCPath
	}
	if _, err := os.Stat(mbPath); err != nil {
		t.Skipf("MusicBrainz mirror not found at %s: %v", mbPath, err)
	}
	if _, err := os.Stat(dcPath); err != nil {
		t.Skipf("Discogs mirror not found at %s: %v", dcPath, err)
	}
	m, err := Open(mbPath, dcPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { m.Close() })
	return m
}

// TestIntegrationResolveArtist checks that "Traffic" resolves to the canonical
// Traffic artist (gid 9fadfba9-…) as one of the top candidates.
func TestIntegrationResolveArtist(t *testing.T) {
	m := openRealMirror(t)
	got, err := m.ResolveArtist("Traffic", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one artist candidate for 'Traffic'")
	}
	// The canonical Traffic band must appear in the results.
	var found bool
	for _, c := range got {
		if c.MBID == core.MBID(trafficGID) {
			found = true
			if c.Name == "" {
				t.Error("Traffic candidate has empty Name")
			}
			if c.Match.FTSRank == nil {
				t.Error("Traffic candidate missing fts_rank signal")
			}
			break
		}
	}
	if !found {
		t.Errorf("Traffic gid %s not found in %d candidates", trafficGID, len(got))
	}
}

// TestIntegrationFindRecordingByTitleArtist checks that searching "Glad" by
// Traffic returns the canonical recording with a known ISRC.
func TestIntegrationFindRecordingByTitleArtist(t *testing.T) {
	m := openRealMirror(t)
	got, err := m.FindRecording(RecordingQuery{Title: "Glad", ArtistName: "Traffic", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one recording of 'Glad' by Traffic")
	}
	// The canonical JBMD Glad recording must appear.
	var found bool
	for _, c := range got {
		if c.MBID == core.MBID(gladRecordingGID) {
			found = true
			if c.ISRC == "" {
				t.Error("Glad recording missing ISRC")
			}
			if c.Match.ArtistConfirmed == nil || !*c.Match.ArtistConfirmed {
				t.Error("artist_confirmed should be true when artist filter applied")
			}
			break
		}
	}
	if !found {
		t.Errorf("Glad recording gid %s not found in %d candidates", gladRecordingGID, len(got))
	}
}

// TestIntegrationFindRecordingByWork checks that querying recordings linked to
// the "Dear Mr. Fantasy" work returns ≥1 result.
func TestIntegrationFindRecordingByWork(t *testing.T) {
	m := openRealMirror(t)
	got, err := m.FindRecording(RecordingQuery{Work: "Dear Mr. Fantasy", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one recording linked to work 'Dear Mr. Fantasy'")
	}
	// Every result must have a gid and a title.
	for i, c := range got {
		if c.MBID == "" {
			t.Errorf("result[%d] missing MBID", i)
		}
		if c.Title == "" {
			t.Errorf("result[%d] missing Title", i)
		}
	}
}

// TestIntegrationResolveWork checks that "Dear Mr. Fantasy" resolves to a work
// with the expected gid and composers Steve Winwood and Chris Wood.
func TestIntegrationResolveWork(t *testing.T) {
	m := openRealMirror(t)
	got, err := m.ResolveWork("Dear Mr. Fantasy", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one work for 'Dear Mr. Fantasy'")
	}
	var found bool
	for _, w := range got {
		if w.MBID != core.MBID(dmfWorkGID) {
			continue
		}
		found = true
		if len(w.Composers) == 0 {
			t.Error("Dear Mr. Fantasy work has no composers")
		}
		hasWinwood, hasWood := false, false
		for _, c := range w.Composers {
			if c == "Steve Winwood" {
				hasWinwood = true
			}
			if c == "Chris Wood" {
				hasWood = true
			}
		}
		if !hasWinwood {
			t.Errorf("composers %v missing Steve Winwood", w.Composers)
		}
		if !hasWood {
			t.Errorf("composers %v missing Chris Wood", w.Composers)
		}
		break
	}
	if !found {
		t.Errorf("Dear Mr. Fantasy work gid %s not found in %d results", dmfWorkGID, len(got))
	}
}

// TestIntegrationFindAlbum checks that searching "John Barleycorn Must Die" by
// Traffic returns both a MusicBrainz peer (with the rg gid) and a Discogs peer
// (with master id 69017).
func TestIntegrationFindAlbum(t *testing.T) {
	m := openRealMirror(t)
	got, err := m.FindAlbum(AlbumQuery{Title: "John Barleycorn Must Die", ArtistName: "Traffic", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one album candidate")
	}
	var foundMB, foundDC bool
	for _, c := range got {
		for _, src := range c.Sources {
			if src == core.SourceMusicBrainz && c.MBID == core.MBID(jbmdRGGID) {
				foundMB = true
			}
			if src == core.SourceDiscogs && c.DiscogsMasterID == core.DiscogsMasterID(jbmdDCMID) {
				foundDC = true
			}
		}
	}
	if !foundMB {
		t.Errorf("MB peer with rg gid %s not found among %d candidates", jbmdRGGID, len(got))
	}
	if !foundDC {
		t.Errorf("Discogs peer with master id %d not found among %d candidates", jbmdDCMID, len(got))
	}
}

// TestIntegrationTracklistByReleaseGroup checks that the canonical MB tracklist
// for JBMD has exactly 6 tracks and the first is "Glad".
func TestIntegrationTracklistByReleaseGroup(t *testing.T) {
	m := openRealMirror(t)
	tracks, err := m.TracklistByReleaseGroup(jbmdRGGID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 6 {
		t.Errorf("expected 6 canonical JBMD tracks, got %d", len(tracks))
	}
	if len(tracks) > 0 {
		if tracks[0].Title != "Glad" {
			t.Errorf("track 1 title = %q, want Glad", tracks[0].Title)
		}
		if tracks[0].MBID == "" {
			t.Error("track 1 missing MBID")
		}
	}
	for i, tk := range tracks {
		if tk.Position != i+1 {
			t.Errorf("track[%d].Position = %d, want %d", i, tk.Position, i+1)
		}
	}
}

// TestIntegrationTracklistByMaster checks that the Discogs main-release
// tracklist for JBMD master 69017 has ≥6 tracks.
func TestIntegrationTracklistByMaster(t *testing.T) {
	m := openRealMirror(t)
	tracks, err := m.TracklistByMaster(jbmdDCMID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) < 6 {
		t.Errorf("expected ≥6 Discogs JBMD tracks, got %d", len(tracks))
	}
	if len(tracks) > 0 {
		if tracks[0].Title == "" {
			t.Error("first Discogs track has empty title")
		}
		// The Discogs main release opens with "Glad".
		if tracks[0].Title != "Glad" {
			t.Errorf("Discogs track 1 title = %q, want Glad", tracks[0].Title)
		}
	}
}

// TestIntegrationAlbumEditionsMB checks that JBMD has multiple MB editions with
// key fields populated.
func TestIntegrationAlbumEditionsMB(t *testing.T) {
	m := openRealMirror(t)
	eds, err := m.AlbumEditionsMB(jbmdRGGID)
	if err != nil {
		t.Fatal(err)
	}
	if len(eds) < 5 {
		t.Errorf("expected ≥5 MB editions for JBMD, got %d", len(eds))
	}
	for i, e := range eds {
		if e.MBID == "" {
			t.Errorf("edition[%d] missing MBID", i)
		}
		if e.Title == "" {
			t.Errorf("edition[%d] missing Title", i)
		}
		if e.Source != core.SourceMusicBrainz {
			t.Errorf("edition[%d] source = %q, want musicbrainz", i, e.Source)
		}
	}
}

// TestIntegrationAlbumEditionsDiscogs checks that JBMD master 69017 has
// multiple Discogs editions and that exactly one is flagged as the main release.
func TestIntegrationAlbumEditionsDiscogs(t *testing.T) {
	m := openRealMirror(t)
	eds, err := m.AlbumEditionsDiscogs(jbmdDCMID)
	if err != nil {
		t.Fatal(err)
	}
	if len(eds) < 10 {
		t.Errorf("expected ≥10 Discogs editions for JBMD master 69017, got %d", len(eds))
	}
	mainCount := 0
	for i, e := range eds {
		if e.Title == "" {
			t.Errorf("edition[%d] missing Title", i)
		}
		if e.Source != core.SourceDiscogs {
			t.Errorf("edition[%d] source = %q, want discogs", i, e.Source)
		}
		if e.IsMainRelease {
			mainCount++
		}
	}
	if mainCount != 1 {
		t.Errorf("expected exactly 1 main release among %d editions, got %d", len(eds), mainCount)
	}
}
