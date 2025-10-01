// Package textx provides small text utilities used across the project.
package textx

import (
	"strings"
)

// SanitizeText removes control characters except tab/newline/CR and trims spaces.
func SanitizeText(s string) string {
	// strip control chars outside tab/newline/carriage return
	var b strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' || (r >= 32 && r != 127) {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
