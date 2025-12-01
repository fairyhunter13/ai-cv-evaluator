package httpserver

import "testing"

func TestValidateJobID(t *testing.T) {
	cases := []struct {
		name  string
		id    string
		valid bool
		code  string
	}{
		{"empty", "", false, "REQUIRED"},
		{"too_long", makeString(101, 'a'), false, "TOO_LONG"},
		{"invalid_chars", "abc$%", false, "INVALID_FORMAT"},
		{"valid", "job-123_ABC", true, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := ValidateJobID(tc.id)
			if res.Valid != tc.valid {
				t.Fatalf("Valid=%v, want %v", res.Valid, tc.valid)
			}
			if !tc.valid {
				if len(res.Errors) != 1 || res.Errors[0].Code != tc.code {
					t.Fatalf("unexpected error: %+v", res.Errors)
				}
			}
		})
	}
}

func TestValidateSearchQuery(t *testing.T) {
	if !ValidateSearchQuery("").Valid {
		t.Fatalf("empty query should be valid")
	}

	long := makeString(201, 'a')
	res := ValidateSearchQuery(long)
	if res.Valid || res.Errors[0].Code != "TOO_LONG" {
		t.Fatalf("expected TOO_LONG error, got %+v", res)
	}

	res = ValidateSearchQuery("ok query")
	if !res.Valid {
		t.Fatalf("simple query should be valid")
	}

	res = ValidateSearchQuery("bad!query")
	if res.Valid || res.Errors[0].Code != "INVALID_FORMAT" {
		t.Fatalf("expected INVALID_FORMAT error, got %+v", res)
	}
}

func TestValidateStatus(t *testing.T) {
	if !ValidateStatus("").Valid {
		t.Fatalf("empty status should be valid")
	}
	for _, s := range []string{"queued", "processing", "completed", "failed"} {
		if !ValidateStatus(s).Valid {
			t.Fatalf("status %q should be valid", s)
		}
	}
	res := ValidateStatus("other")
	if res.Valid || res.Errors[0].Code != "INVALID_VALUE" {
		t.Fatalf("expected INVALID_VALUE error, got %+v", res)
	}
}

func TestSanitizeString(t *testing.T) {
	in := "  hello\x00world  "
	out := SanitizeString(in)
	if out != "helloworld" {
		t.Fatalf("SanitizeString output=%q", out)
	}

	// Long string should be truncated
	long := makeString(1500, 'a')
	out = SanitizeString(long)
	if len(out) != 1000 {
		t.Fatalf("expected length 1000, got %d", len(out))
	}
}

func TestSanitizeJobID(t *testing.T) {
	id := " job$%id-123_ABC "
	out := SanitizeJobID(id)
	if out != "jobid-123_ABC" {
		t.Fatalf("SanitizeJobID output=%q", out)
	}

	long := makeString(150, 'b')
	out = SanitizeJobID(long)
	if len(out) != 100 {
		t.Fatalf("expected length 100, got %d", len(out))
	}
}

func makeString(n int, ch rune) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = ch
	}
	return string(b)
}
