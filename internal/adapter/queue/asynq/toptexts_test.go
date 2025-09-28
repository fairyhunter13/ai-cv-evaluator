package asynqadp

import "testing"

func Test_topTextsByWeight(t *testing.T) {
	in := []map[string]any{
		{"payload": map[string]any{"text": "a", "weight": 0.9}},
		{"payload": map[string]any{"text": "b"}},
		{"payload": map[string]any{"text": "c", "weight": 1.2}},
		{"payload": map[string]any{"text": "a", "weight": 0.5}}, // duplicate
	}
	out := topTextsByWeight(in, 3)
	if len(out) != 3 { t.Fatalf("want 3, got %d", len(out)) }
	if out[0] != "c" || out[1] != "a" { t.Fatalf("unexpected order: %#v", out) }
}
