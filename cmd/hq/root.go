package main

import (
	"flag"
	"fmt"
	"regexp"
	"strings"

	"github.com/ethereum/hive/cmd/hq/internal/api"
	"github.com/ethereum/hive/cmd/hq/internal/cache"
	"github.com/fatih/color"
)

// Global flags shared by all subcommands.
var (
	baseURL  string
	suite    string
	noCache  bool
	noColor  bool
	cacheDir string
)

// addGlobalFlags registers the global flags on fs. Each subcommand calls this
// so globals can appear after the subcommand name, e.g.
//
//	hq runs -sim rpc-compat -base-url https://...
func addGlobalFlags(fs *flag.FlagSet) {
	fs.StringVar(&baseURL, "base-url", "https://hive.ethpandaops.io", "Hive server base URL")
	fs.StringVar(&suite, "suite", "generic", "Test suite name")
	fs.BoolVar(&noCache, "no-cache", false, "Bypass cache reads")
	fs.BoolVar(&noColor, "no-color", false, "Disable colored output")
	fs.StringVar(&cacheDir, "cache-dir", "", "Cache directory (default ~/.cache/hq)")
}

// applyGlobals propagates parsed globals into shared state. Must be called
// after fs.Parse.
func applyGlobals() {
	if noColor {
		color.NoColor = true
	}
}

func newClient() (*api.Client, error) {
	c, err := cache.New(cacheDir, !noCache)
	if err != nil {
		return nil, fmt.Errorf("initializing cache: %w", err)
	}
	return api.NewClient(baseURL, suite, c), nil
}

// matchTestCase reports whether tc passes the client and test-name filters.
// An empty clientFilter and a nil testRE both match everything. The extracted
// client name is returned as a convenience for display.
//
// When a test name carries a trailing " (client)" suffix, matching is retried
// against the name with that suffix stripped, so patterns that don't account
// for the suffix still work.
func matchTestCase(tc api.TestCase, clientFilter string, testRE *regexp.Regexp) (clientName string, ok bool) {
	clientName = api.ExtractClient(tc.Name)
	if clientFilter != "" && !strings.Contains(strings.ToLower(clientName), strings.ToLower(clientFilter)) {
		return clientName, false
	}
	if testRE == nil {
		return clientName, true
	}
	if testRE.MatchString(tc.Name) {
		return clientName, true
	}
	if idx := strings.LastIndex(tc.Name, " ("); idx > 0 && testRE.MatchString(tc.Name[:idx]) {
		return clientName, true
	}
	return clientName, false
}
