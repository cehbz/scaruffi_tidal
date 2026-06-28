package core

import (
	"encoding/json"
	"fmt"
)

// criterionJSON is the wire form of the closed criterion union: a `type` tag plus
// the fields that tag carries. Validated by tag on read, never eval'd.
type criterionJSON struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// MarshalCriterion writes a criterion as its tagged JSON object.
func MarshalCriterion(c Criterion) ([]byte, error) {
	switch x := c.(type) {
	case Studio:
		return json.Marshal(criterionJSON{Type: "studio"})
	case NoCompilation:
		return json.Marshal(criterionJSON{Type: "no_compilation"})
	case NoLive:
		return json.Marshal(criterionJSON{Type: "no_live"})
	case PerformedBy:
		return json.Marshal(criterionJSON{Type: "performed_by", Name: x.Name})
	default:
		return nil, fmt.Errorf("unserializable criterion: %T", c)
	}
}

// UnmarshalCriterion reads a tagged criterion object, validating the tag against
// the closed union.
func UnmarshalCriterion(data []byte) (Criterion, error) {
	var j criterionJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, err
	}
	switch j.Type {
	case "studio":
		return Studio{}, nil
	case "no_compilation":
		return NoCompilation{}, nil
	case "no_live":
		return NoLive{}, nil
	case "performed_by":
		return PerformedBy{Name: j.Name}, nil
	default:
		return nil, fmt.Errorf("unknown criterion type: %q", j.Type)
	}
}

// UnmarshalCriteria reads a JSON array of tagged criteria.
func UnmarshalCriteria(data []byte) ([]Criterion, error) {
	var raws []json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return nil, err
	}
	out := make([]Criterion, 0, len(raws))
	for _, raw := range raws {
		c, err := UnmarshalCriterion(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}
