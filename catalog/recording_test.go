package catalog

import "testing"

func TestFindRecordingByTitleAndArtist(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindRecording(RecordingQuery{Title: "Dear Mr. Fantasy", ArtistName: "Traffic", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("artist filter should yield exactly the Traffic recording; got %d", len(got))
	}
	c := got[0]
	if c.MBID != "r-dmf" {
		t.Errorf("MBID = %q, want r-dmf", c.MBID)
	}
	if c.ISRC != "GBABC1234567" {
		t.Errorf("ISRC = %q, want GBABC1234567", c.ISRC)
	}
	if c.DurationS != 300 {
		t.Errorf("DurationS = %d, want 300 (ms/1000)", c.DurationS)
	}
	if c.Match.ArtistConfirmed == nil || !*c.Match.ArtistConfirmed {
		t.Error("artist_confirmed should be true when the artist filter applied")
	}
	if c.Match.TitleDistance == nil || *c.Match.TitleDistance != 0 {
		t.Errorf("title_distance should be 0 for an exact title; got %v", c.Match.TitleDistance)
	}
}

func TestFindRecordingTitleOnlyWhenNoArtist(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindRecording(RecordingQuery{Title: "Dear Mr. Fantasy", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("title-only should return both recordings; got %d", len(got))
	}
	if got[0].Match.ArtistConfirmed != nil {
		t.Error("artist_confirmed must be omitted when no artist filter applied")
	}
}

func TestFindRecordingISRCExact(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindRecording(RecordingQuery{Title: "Dear Mr. Fantasy", ArtistName: "Traffic", ISRC: "GBABC1234567", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Match.ISRCExact == nil || !*got[0].Match.ISRCExact {
		t.Error("isrc_exact should be true when the requested ISRC matches")
	}
}

func TestFindRecordingUnresolvedArtistReturnsEmpty(t *testing.T) {
	m := newTestMirror(t)

	got, err := m.FindRecording(RecordingQuery{Title: "Dear Mr. Fantasy", ArtistName: "Nonexistent Band", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("unresolved ArtistName should yield 0 candidates; got %d", len(got))
	}

	got, err = m.FindRecording(RecordingQuery{Title: "Dear Mr. Fantasy", ArtistMBID: "bogus-gid", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("unresolved ArtistMBID should yield 0 candidates; got %d", len(got))
	}
}

func TestFindRecordingByArtistMBID(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindRecording(RecordingQuery{Title: "Dear Mr. Fantasy", ArtistMBID: "a-traffic", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("ArtistMBID filter should yield exactly 1 candidate; got %d", len(got))
	}
	c := got[0]
	if c.MBID != "r-dmf" {
		t.Errorf("MBID = %q, want r-dmf", c.MBID)
	}
	if c.Match.ArtistConfirmed == nil || !*c.Match.ArtistConfirmed {
		t.Error("ArtistConfirmed should be non-nil and true when ArtistMBID resolved")
	}
}

func TestFindRecordingByWork(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindRecording(RecordingQuery{Work: "Missa Papae Marcelli", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 recording of the work, got %d", len(got))
	}
	if got[0].MBID != "r-kyrie" {
		t.Errorf("MBID = %q, want r-kyrie", got[0].MBID)
	}
	if got[0].ISRC != "GBCLASSICAL01" {
		t.Errorf("ISRC = %q, want GBCLASSICAL01", got[0].ISRC)
	}
	if got[0].DurationS != 360 {
		t.Errorf("DurationS = %d, want 360", got[0].DurationS)
	}
}

func TestFindRecordingByWorkUnresolvedEmpty(t *testing.T) {
	m := newTestMirror(t)
	got, err := m.FindRecording(RecordingQuery{Work: "Nonexistent Work", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("an unresolved work must yield no recordings; got %d", len(got))
	}
}
