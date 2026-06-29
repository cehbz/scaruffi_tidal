package main

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestErrorJSON(t *testing.T) {
	raw := errorJSON(errors.New(`bad "quote"`))
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("errorJSON output is not valid JSON: %v — got %q", err, raw)
	}
	if got, want := m["error"], `bad "quote"`; got != want {
		t.Errorf("error field = %q, want %q", got, want)
	}
}
