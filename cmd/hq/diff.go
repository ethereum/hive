package main

import (
	"flag"
	"fmt"

	"github.com/ethereum/hive/cmd/hq/internal/api"
	"github.com/ethereum/hive/cmd/hq/internal/display"
)

func diffCommand(args []string) {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: hq diff [flags] [run-file]")
		fmt.Fprintln(fs.Output(), "\nShow colorized diff for failing tests. If no run-file is given, uses the")
		fmt.Fprintln(fs.Output(), "most recent run matching the -sim and -client filters.")
		fs.PrintDefaults()
	}
	addGlobalFlags(fs)
	var (
		sim      = fs.String("sim", "", "Filter runs by simulator name")
		testPat  = fs.String("test", "", "Filter by test name (glob pattern)")
		clientFl = fs.String("client", "", "Filter by client name")
		full     = fs.Bool("full", false, "Show full output instead of only differences")
	)
	fs.Parse(args)
	applyGlobals()

	*clientFl = api.ResolveClientAlias(*clientFl)
	testRE, err := api.CompileGlob(*testPat)
	if err != nil {
		fatalf("invalid -test pattern: %v", err)
	}

	client, err := newClient()
	if err != nil {
		fatalf("%v", err)
	}

	var fileName string
	if fs.NArg() > 0 {
		fileName = fs.Arg(0)
	} else {
		entries, err := client.FetchListing(*sim, *clientFl, 0)
		if err != nil {
			fatalf("fetching listing: %v", err)
		}
		if len(entries) == 0 {
			fatalf("no runs found matching filters")
		}
		api.SortByTime(entries)
		fileName = entries[0].FileName
		fmt.Printf("Using most recent run: %s\n\n", fileName)
	}

	result, err := client.FetchResult(fileName)
	if err != nil {
		fatalf("fetching result: %v", err)
	}

	if result.TestDetailsLog == "" {
		fatalf("no test details log available for this run")
	}

	matched := 0
	for _, tc := range result.TestCases {
		if tc.SummaryResult.Pass {
			continue
		}

		if _, ok := matchTestCase(tc, *clientFl, testRE); !ok {
			continue
		}

		begin := tc.SummaryResult.Log.Begin
		end := tc.SummaryResult.Log.End
		if begin == 0 && end == 0 {
			continue
		}

		log, err := client.FetchTestLog(result.TestDetailsLog, begin, end)
		if err != nil {
			fmt.Printf("Error fetching log for %s: %v\n", tc.Name, err)
			continue
		}

		matched++
		display.Bold.Printf("=== %s ===\n", tc.Name)
		if *full {
			display.ColorizeDiff(log, noColor)
		} else {
			display.CompactDiff(log, 3, noColor)
		}
		fmt.Println()
	}

	if matched == 0 {
		fmt.Println("No matching failing tests with log data found.")
	}
}
