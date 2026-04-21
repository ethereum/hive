package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/hive/cmd/hq/internal/api"
	"github.com/ethereum/hive/cmd/hq/internal/display"
)

func statsCommand(args []string) {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: hq stats -sim <name> [flags]")
		fmt.Fprintln(fs.Output(), "\nAggregate pass/fail rates across runs. Without -client, shows a row per")
		fmt.Fprintln(fs.Output(), "client across the last -last runs. With -client, shows a row per run for")
		fmt.Fprintln(fs.Output(), "that client.")
		fs.PrintDefaults()
	}
	addGlobalFlags(fs)
	var (
		sim      = fs.String("sim", "", "Simulator name (required)")
		clientFl = fs.String("client", "", "Filter by client name")
		last     = fs.Int("last", 10, "Number of recent runs to analyze")
	)
	fs.Parse(args)
	applyGlobals()

	if *sim == "" {
		fatalf("-sim flag is required")
	}
	*clientFl = api.ResolveClientAlias(*clientFl)

	client, err := newClient()
	if err != nil {
		fatalf("%v", err)
	}

	// Filter by client up-front so -last N slices the last N runs that
	// actually involve this client, not the last N runs overall.
	entries, err := client.FetchListing(*sim, *clientFl, 0)
	if err != nil {
		fatalf("fetching listing: %v", err)
	}

	api.SortByTime(entries)

	if *last > 0 && len(entries) > *last {
		entries = entries[:*last]
	}

	if *clientFl != "" {
		// Per-run stats for a specific client.
		t := display.NewTable([]string{"Run", "Tests", "Pass", "Fail", "Rate", "When"})
		for _, e := range entries {
			result, err := client.FetchResult(e.FileName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", e.FileName, err)
				continue
			}

			passes, fails := countForClient(result, *clientFl)
			total := passes + fails
			rate := "N/A"
			if total > 0 {
				rate = fmt.Sprintf("%.1f%%", float64(passes)/float64(total)*100)
			}
			t.Append([]string{
				e.FileName,
				fmt.Sprintf("%d", total),
				fmt.Sprintf("%d", passes),
				fmt.Sprintf("%d", fails),
				rate,
				api.FormatTime(e.Start),
			})
		}
		t.Render()
		return
	}

	// Aggregate stats per client across runs.
	type clientStats struct {
		passes int
		fails  int
		runs   int
	}
	stats := make(map[string]*clientStats)

	for _, e := range entries {
		result, err := client.FetchResult(e.FileName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", e.FileName, err)
			continue
		}

		clientCounts := make(map[string][2]int) // [passes, fails]
		for _, tc := range result.TestCases {
			cl := api.ExtractClient(tc.Name)
			if cl == "" {
				continue
			}
			counts := clientCounts[cl]
			if tc.SummaryResult.Pass {
				counts[0]++
			} else {
				counts[1]++
			}
			clientCounts[cl] = counts
		}

		for cl, counts := range clientCounts {
			s, ok := stats[cl]
			if !ok {
				s = &clientStats{}
				stats[cl] = s
			}
			s.passes += counts[0]
			s.fails += counts[1]
			s.runs++
		}
	}

	t := display.NewTable([]string{"Client", "Runs", "Total Tests", "Pass", "Fail", "Rate"})
	for cl, s := range stats {
		total := s.passes + s.fails
		rate := "N/A"
		if total > 0 {
			rate = fmt.Sprintf("%.1f%%", float64(s.passes)/float64(total)*100)
		}
		t.Append([]string{
			cl,
			fmt.Sprintf("%d", s.runs),
			fmt.Sprintf("%d", total),
			fmt.Sprintf("%d", s.passes),
			fmt.Sprintf("%d", s.fails),
			rate,
		})
	}
	t.Render()
}

func countForClient(result *api.TestSuiteResult, clientName string) (passes, fails int) {
	for _, tc := range result.TestCases {
		cl := api.ExtractClient(tc.Name)
		if !strings.Contains(strings.ToLower(cl), strings.ToLower(clientName)) {
			continue
		}
		if tc.SummaryResult.Pass {
			passes++
		} else {
			fails++
		}
	}
	return
}
