package core

import "testing"

func TestNoCompilationRejectsCompilationAlbum(t *testing.T) {
	c := NoCompilation{}
	comp := Album{Title: "X", Traits: []ReleaseTrait{TraitCompilation}}
	if got := c.Violation(comp); got != "compilation" {
		t.Errorf("Violation(compilation album) = %q, want %q", got, "compilation")
	}
	if got := c.Violation(Album{Title: "Y"}); got != "" {
		t.Errorf("Violation(clean album) = %q, want \"\"", got)
	}
}

func TestNoLiveRejectsLiveAlbum(t *testing.T) {
	c := NoLive{}
	if got := c.Violation(Album{Title: "X", Traits: []ReleaseTrait{TraitLive}}); got != "live album" {
		t.Errorf("Violation(live album) = %q, want %q", got, "live album")
	}
	if got := c.Violation(Album{Title: "Y"}); got != "" {
		t.Errorf("Violation(studio album) = %q, want \"\"", got)
	}
}

func TestAlbumCriteriaNoOpOnRecording(t *testing.T) {
	r := Recording{Title: "r"}
	if got := (NoCompilation{}).Violation(r); got != "" {
		t.Errorf("NoCompilation on a recording must no-op; got %q", got)
	}
	if got := (NoLive{}).Violation(r); got != "" {
		t.Errorf("NoLive on a recording must no-op; got %q", got)
	}
}

func TestJudgeAggregatesViolations(t *testing.T) {
	item := Album{Title: "X", Traits: []ReleaseTrait{TraitCompilation, TraitLive}}
	v := Judge(item, []Criterion{NoCompilation{}, NoLive{}})
	if v.Admitted {
		t.Error("a compilation+live album must be rejected")
	}
	if len(v.Violations) != 2 {
		t.Errorf("expected 2 violations, got %v", v.Violations)
	}
}

func TestJudgeAdmitsWhenClean(t *testing.T) {
	v := Judge(Album{Title: "X"}, []Criterion{NoCompilation{}, NoLive{}})
	if !v.Admitted || len(v.Violations) != 0 {
		t.Errorf("clean album should be admitted with no violations; got %+v", v)
	}
}

func TestStudioRejectsLiveRecording(t *testing.T) {
	c := Studio{}
	if got := c.Violation(Recording{Title: "x", Performance: PerfLive}); got != "live recording" {
		t.Errorf("Violation(live take) = %q, want %q", got, "live recording")
	}
	if got := c.Violation(Recording{Title: "x", Performance: PerfStudio}); got != "" {
		t.Errorf("studio take must pass; got %q", got)
	}
	if got := c.Violation(Recording{Title: "x", Performance: PerfUnknown}); got != "" {
		t.Errorf("unknown performance must pass (only explicit live fails); got %q", got)
	}
}

func TestStudioNoOpOnAlbum(t *testing.T) {
	if got := (Studio{}).Violation(Album{Title: "a"}); got != "" {
		t.Errorf("Studio on an album must no-op; got %q", got)
	}
}

func TestPerformedByRejectsCover(t *testing.T) {
	c := PerformedBy{Name: "Traffic"}
	cover := Recording{Title: "x", Credits: Credits{{Role: RoleArtist, Name: "Joe Cocker"}}}
	if got := c.Violation(cover); got == "" {
		t.Error("a recording not by the performer should be rejected as a likely cover")
	}
	by := Recording{Title: "x", Credits: Credits{{Role: RoleArtist, Name: "Traffic"}}}
	if got := c.Violation(by); got != "" {
		t.Errorf("a recording by the performer must pass; got %q", got)
	}
}

func TestPerformedByNoOpOnAlbum(t *testing.T) {
	if got := (PerformedBy{Name: "X"}).Violation(Album{Title: "a"}); got != "" {
		t.Errorf("PerformedBy on an album must no-op; got %q", got)
	}
}
