package real

import (
	"net/http"
	"testing"
	"time"
)

func TestParseRetryAfterHeader_SecondsValue(t *testing.T) {
	d := parseRetryAfterHeader("60")
	if d != 60*time.Second {
		t.Fatalf("expected 60s, got %v", d)
	}
}

func TestParseRetryAfterHeader_HTTPDateFutureAndPast(t *testing.T) {
	future := time.Now().Add(5 * time.Second).UTC().Format(http.TimeFormat)
	d := parseRetryAfterHeader(future)
	if d <= 0 {
		t.Fatalf("expected positive duration for future HTTP-date, got %v", d)
	}

	past := time.Now().Add(-5 * time.Second).UTC().Format(http.TimeFormat)
	if d2 := parseRetryAfterHeader(past); d2 != 0 {
		t.Fatalf("expected 0 duration for past HTTP-date, got %v", d2)
	}
}

func TestParseRetryAfterHeader_Invalid(t *testing.T) {
	if d := parseRetryAfterHeader("not-a-date"); d != 0 {
		t.Fatalf("expected 0 duration for invalid header, got %v", d)
	}
}

func TestTruncateString_Behaviour(t *testing.T) {
	if got := truncateString("short", 10); got != "short" {
		t.Fatalf("truncateString short = %q, want %q", got, "short")
	}

	if got := truncateString("this is long", 4); got != "this..." {
		t.Fatalf("truncateString long = %q, want %q", got, "this...")
	}
}
