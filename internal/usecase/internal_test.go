package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func Test_sanitize(t *testing.T) {
	if got := sanitize("  hello  "); got != "hello" {
		t.Fatalf("sanitize: %q", got)
	}
	if got := sanitize("\n\thello\n"); got != "hello" {
		t.Fatalf("sanitize: %q", got)
	}
}

func Test_mimeFromName(t *testing.T) {
	cases := map[string]string{
		"file.pdf":  "application/pdf",
		"FILE.DOCX": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"note.txt":  "text/plain",
	}
	for in, want := range cases {
		if got := mimeFromName(in); got != want {
			t.Fatalf("%s => %s (got %s)", in, want, got)
		}
	}
}

func Test_hash(t *testing.T) {
	in := "abc"
	ex := sha256.Sum256([]byte(in))
	want := hex.EncodeToString(ex[:])
	if got := hash(in); got != want {
		t.Fatalf("hash mismatch: %s vs %s", got, want)
	}
}
