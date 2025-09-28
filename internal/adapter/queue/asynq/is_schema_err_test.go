package asynqadp

import "testing"

func Test_isSchemaOrJSONErr(t *testing.T) {
	cases := []struct{ in string; want bool }{
		{"invalid json: bad", true},
		{"schema invalid: missing", true},
		{"out of range", true},
		{"empty field", true},
		{"ok", false},
	}
	for _, c := range cases {
		if got := isSchemaOrJSONErr(assertError(c.in)); got != c.want {
			t.Fatalf("%q => %v", c.in, got)
		}
	}
}

type assertError string
func (a assertError) Error() string { return string(a) }
