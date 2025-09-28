// Package textx contains tests for the text utilities.
package textx

import "testing"

func TestSanitizeText(t *testing.T) {
	in := "he\x00llo\nwo\x7frld\t!"
	got := SanitizeText(in)
	if got != "hello\nworld\t!" {
		t.Fatalf("unexpected: %q", got)
	}
}
