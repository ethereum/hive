package hivesim

import (
	"regexp"
	"strings"
)

type testMatcher struct {
	suite   *regexp.Regexp
	test    *regexp.Regexp
	pattern string
}

func parseTestPattern(p string) (m testMatcher, err error) {
	parts := splitRegexp(p)
	m.suite, err = regexp.Compile("(?i:" + parts[0] + ")")
	if err != nil {
		return m, err
	}
	if len(parts) > 1 {
		m.test, err = regexp.Compile("(?i:" + strings.Join(parts[1:], "/") + ")")
		if err != nil {
			return m, err
		}
	}
	m.pattern = p
	return m, nil
}

// match checks whether the pattern matches suite and test name.
func (m *testMatcher) match(suite, test string) bool {
	if m.suite != nil && !m.suite.MatchString(suite) {
		return false
	}
	if test != "" && m.test != nil && !m.test.MatchString(test) {
		return false
	}
	return true
}

// splitRegexp splits the expression s into /-separated parts.
//
// This is borrowed from package testing.
func splitRegexp(s string) []string {
	a := make([]string, 0, strings.Count(s, "/"))
	cs := 0
	cp := 0
	for i := 0; i < len(s); {
		switch s[i] {
		case '[':
			cs++
		case ']':
			if cs--; cs < 0 { // An unmatched ']' is legal.
				cs = 0
			}
		case '(':
			if cs == 0 {
				cp++
			}
		case ')':
			if cs == 0 {
				cp--
			}
		case '\\':
			i++
		case '/':
			if cs == 0 && cp == 0 {
				a = append(a, s[:i])
				s = s[i+1:]
				i = 0
				continue
			}
		}
		i++
	}
	return append(a, s)
}
