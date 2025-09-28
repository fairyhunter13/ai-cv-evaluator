package textx

import "testing"

func TestSanitizeText(t *testing.T) {
	in := "he\x00llo\nwo\x7frld\t!"
	got := SanitizeText(in)
	if got != "he\nwo\trld!" {
		t.Fatalf("unexpected: %q", got)
	}
}
