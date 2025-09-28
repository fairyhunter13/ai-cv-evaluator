package app

import "testing"

func TestParseOrigins(t *testing.T) {
	cases := []struct{ in string; want []string }{
		{"", []string{"*"}},
		{"*", []string{"*"}},
		{"https://a.com, https://b.com", []string{"https://a.com", "https://b.com"}},
		{"  ,  ", []string{"*"}},
	}
	for _, c := range cases {
		got := ParseOrigins(c.in)
		if len(got) != len(c.want) { t.Fatalf("len mismatch for %q: %v vs %v", c.in, got, c.want) }
		for i := range got { if got[i] != c.want[i] { t.Fatalf("mismatch idx %d: %v vs %v", i, got, c.want) } }
	}
}
