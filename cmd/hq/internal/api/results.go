package api

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// FetchResult fetches a test suite result file. These are immutable (content-addressed).
func (c *Client) FetchResult(fileName string) (*TestSuiteResult, error) {
	url := fmt.Sprintf("%s/%s/results/%s", c.BaseURL, c.Suite, fileName)
	data, err := c.fetch(url, 0) // immutable, cache forever
	if err != nil {
		return nil, err
	}
	var result TestSuiteResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing result %s: %w", fileName, err)
	}
	return &result, nil
}

// FetchTestLog fetches the byte range for a specific test case's log.
func (c *Client) FetchTestLog(detailsLog string, begin, end int64) (string, error) {
	if begin == 0 && end == 0 {
		return "", nil
	}
	url := fmt.Sprintf("%s/%s/results/%s", c.BaseURL, c.Suite, detailsLog)
	data, err := c.fetchRange(url, begin, end)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

var clientRegex = regexp.MustCompile(`\(([^)]+)\)$`)

// ExtractClient extracts the client name from a test case name. Test names
// follow the pattern "method/test-name (client_name)".
func ExtractClient(testName string) string {
	m := clientRegex.FindStringSubmatch(strings.TrimSpace(testName))
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

// CompileGlob compiles a shell-style glob pattern into an anchored regular
// expression. The wildcards * and ? match any run of characters (including
// the path separator) and any single character respectively. An empty pattern
// returns (nil, nil), meaning "match everything".
func CompileGlob(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, nil
	}
	var b strings.Builder
	b.Grow(len(pattern) + 4)
	b.WriteByte('^')
	for _, r := range pattern {
		switch r {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteByte('.')
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	b.WriteByte('$')
	return regexp.Compile(b.String())
}
