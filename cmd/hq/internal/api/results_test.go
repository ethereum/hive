package api

import "testing"

func TestExtractClient(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "eth_getBalance/balance-0 (geth)", "geth"},
		{"underscored", "test (nethermind_default)", "nethermind_default"},
		{"trailing space", "foo (besu) ", "besu"},
		{"no client", "some-test-name", ""},
		{"empty", "", ""},
		// The extractor's [^)]+ stops at any ')', so nested parens don't resolve.
		// Real test names don't hit this, but documenting the behavior.
		{"nested parens", "foo (bar (baz))", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractClient(tc.in); got != tc.want {
				t.Errorf("ExtractClient(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCompileGlob(t *testing.T) {
	t.Run("empty pattern returns nil regex", func(t *testing.T) {
		re, err := CompileGlob("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if re != nil {
			t.Fatalf("expected nil regex for empty pattern, got %v", re)
		}
	})

	tests := []struct {
		pattern string
		name    string
		want    bool
	}{
		// * matches anything including /.
		{"eth_getBalance*", "eth_getBalance/balance-0", true},
		{"eth_getBalance/*", "eth_getBalance/balance-0", true},
		{"*getBalance*", "eth_getBalance/x", true},
		{"*", "anything", true},

		// Anchored: partial matches fail without wildcards.
		{"eth_getBalance", "eth_getBalance/balance-0", false},
		{"balance-0", "eth_getBalance/balance-0", false},

		// ? matches one character.
		{"eth_?etBalance", "eth_getBalance", true},
		{"eth_?etBalance", "eth_getgBalance", false},

		// Regex metacharacters in the pattern are taken literally.
		{"foo.bar", "foo.bar", true},
		{"foo.bar", "fooXbar", false},
		{"a+b", "a+b", true},
		{"a+b", "aab", false},

		// Exact match.
		{"specific-name", "specific-name", true},
		{"specific-name", "specific-name-suffix", false},
	}
	for _, tc := range tests {
		t.Run(tc.pattern+"_vs_"+tc.name, func(t *testing.T) {
			re, err := CompileGlob(tc.pattern)
			if err != nil {
				t.Fatalf("CompileGlob(%q) error: %v", tc.pattern, err)
			}
			if got := re.MatchString(tc.name); got != tc.want {
				t.Errorf("match(%q, %q) = %v, want %v", tc.pattern, tc.name, got, tc.want)
			}
		})
	}
}
