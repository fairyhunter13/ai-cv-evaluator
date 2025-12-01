package redpanda

import "testing"

func TestClassifyFailureCode(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		want string
	}{
		{name: "empty", msg: "", want: "INTERNAL"},
		{name: "whitespace", msg: "   \n\t", want: "INTERNAL"},
		{name: "schema_invalid_phrase", msg: "schema invalid: payload", want: "SCHEMA_INVALID"},
		{name: "invalid_json", msg: "Invalid JSON body", want: "SCHEMA_INVALID"},
		{name: "out_of_range", msg: "value OUT OF RANGE", want: "SCHEMA_INVALID"},
		{name: "empty_field", msg: "field is empty", want: "SCHEMA_INVALID"},
		{name: "rate_limit", msg: "upstream rate limit exceeded", want: "UPSTREAM_RATE_LIMIT"},
		{name: "timeout", msg: "request timeout from upstream", want: "UPSTREAM_TIMEOUT"},
		{name: "deadline_exceeded", msg: "context deadline exceeded while calling provider", want: "UPSTREAM_TIMEOUT"},
		{name: "not_found", msg: "resource not found in store", want: "NOT_FOUND"},
		{name: "invalid_argument", msg: "invalid argument provided", want: "INVALID_ARGUMENT"},
		{name: "ids_required", msg: "ids required for this operation", want: "INVALID_ARGUMENT"},
		{name: "default_internal", msg: "some unexpected provider error", want: "INTERNAL"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyFailureCode(tc.msg)
			if got != tc.want {
				t.Fatalf("classifyFailureCode(%q) = %q, want %q", tc.msg, got, tc.want)
			}
		})
	}
}
