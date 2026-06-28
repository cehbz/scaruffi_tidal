package core

import (
	"reflect"
	"testing"
)

func TestCriterionJSONRoundTrip(t *testing.T) {
	for _, c := range []Criterion{Studio{}, NoCompilation{}, NoLive{}, PerformedBy{Name: "Traffic"}} {
		b, err := MarshalCriterion(c)
		if err != nil {
			t.Fatalf("marshal %T: %v", c, err)
		}
		got, err := UnmarshalCriterion(b)
		if err != nil {
			t.Fatalf("unmarshal %s: %v", b, err)
		}
		if !reflect.DeepEqual(c, got) {
			t.Errorf("round-trip %T: %+v -> %s -> %+v", c, c, b, got)
		}
	}
}

func TestCriterionJSONShapes(t *testing.T) {
	b, _ := MarshalCriterion(Studio{})
	if got := string(b); got != `{"type":"studio"}` {
		t.Errorf("Studio JSON = %s", got)
	}
	b, _ = MarshalCriterion(PerformedBy{Name: "Traffic"})
	if got := string(b); got != `{"type":"performed_by","name":"Traffic"}` {
		t.Errorf("PerformedBy JSON = %s", got)
	}
}

func TestUnmarshalUnknownTagErrors(t *testing.T) {
	if _, err := UnmarshalCriterion([]byte(`{"type":"bogus"}`)); err == nil {
		t.Error("an unknown criterion tag must error, never be eval'd")
	}
}

func TestUnmarshalCriteriaList(t *testing.T) {
	cs, err := UnmarshalCriteria([]byte(`[{"type":"studio"},{"type":"no_live"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 {
		t.Fatalf("expected 2 criteria, got %d", len(cs))
	}
	if _, ok := cs[0].(Studio); !ok {
		t.Errorf("first criterion = %T, want Studio", cs[0])
	}
	if _, ok := cs[1].(NoLive); !ok {
		t.Errorf("second criterion = %T, want NoLive", cs[1])
	}
}
