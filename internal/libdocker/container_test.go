package libdocker

import "testing"

func TestShortID(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"abc", "abc"},
		{"e6476c86", "e6476c86"},
		{"e6476c8687d049b96fe0d3e34127b468c7c84616858fba75d253b7b199d5215b", "e6476c86"},
	}
	for _, test := range tests {
		if got := shortID(test.in); got != test.want {
			t.Errorf("shortID(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}
