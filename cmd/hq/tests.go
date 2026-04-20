package main

import (
	"flag"
	"fmt"
	"sort"

	"github.com/ethereum/hive/cmd/hq/internal/api"
	"github.com/ethereum/hive/cmd/hq/internal/display"
)

func testsCommand(args []string) {
	fs := flag.NewFlagSet("tests", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: hq tests [flags] [run-file]")
		fmt.Fprintln(fs.Output(), "\nList test cases in a run with pass/fail status. If no run-file is given,")
		fmt.Fprintln(fs.Output(), "uses the most recent run matching -sim and -client. With -failed, only")
		fmt.Fprintln(fs.Output(), "failing tests are shown together with their error details.")
		fs.PrintDefaults()
	}
	addGlobalFlags(fs)
	var (
		sim      = fs.String("sim", "", "Filter runs by simulator name")
		clientFl = fs.String("client", "", "Filter by client name")
		testPat  = fs.String("test", "", "Filter by test name (glob pattern)")
		onlyFail = fs.Bool("failed", false, "Only show failing tests (with error details)")
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

	type entry struct {
		name    string
		pass    bool
		details string
	}

	var entries []entry
	for _, tc := range result.TestCases {
		if *onlyFail && tc.SummaryResult.Pass {
			continue
		}
		if _, ok := matchTestCase(tc, *clientFl, testRE); !ok {
			continue
		}
		entries = append(entries, entry{
			name:    tc.Name,
			pass:    tc.SummaryResult.Pass,
			details: tc.SummaryResult.Details,
		})
	}

	if len(entries) == 0 {
		if *onlyFail {
			fmt.Println("No failures found.")
		} else {
			fmt.Println("No matching tests found.")
		}
		return
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})

	if *onlyFail {
		fmt.Printf("Suite: %s\n", result.Name)
		fmt.Printf("Failures: %d\n\n", len(entries))
		t := display.NewTable([]string{"Test", "Details"})
		for _, e := range entries {
			details := e.details
			if len(details) > 80 {
				details = details[:77] + "..."
			}
			t.Append([]string{e.name, details})
		}
		t.Render()
		return
	}

	passes := 0
	t := display.NewTable([]string{"Test", "Status"})
	for _, e := range entries {
		t.Append([]string{e.name, display.PassFail(e.pass)})
		if e.pass {
			passes++
		}
	}
	t.Render()
	fmt.Printf("\n%s passing\n", display.PassFailCount(passes, len(entries)))
}
