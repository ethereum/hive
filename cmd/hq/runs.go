package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/ethereum/hive/cmd/hq/internal/api"
	"github.com/ethereum/hive/cmd/hq/internal/display"
)

func runsCommand(args []string) {
	fs := flag.NewFlagSet("runs", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: hq runs [flags]")
		fmt.Fprintln(fs.Output(), "\nList recent test runs, newest first. Filter by -sim and -client.")
		fs.PrintDefaults()
	}
	addGlobalFlags(fs)
	var (
		sim      = fs.String("sim", "", "Filter by simulator name (substring match)")
		clientFl = fs.String("client", "", "Filter by client name")
		limit    = fs.Int("limit", 20, "Maximum number of runs to show")
	)
	fs.Parse(args)
	applyGlobals()

	*clientFl = api.ResolveClientAlias(*clientFl)

	client, err := newClient()
	if err != nil {
		fatalf("%v", err)
	}

	entries, err := client.FetchListing(*sim, *clientFl, 0)
	if err != nil {
		fatalf("fetching listing: %v", err)
	}

	api.SortByTime(entries)

	if *limit > 0 && len(entries) > *limit {
		entries = entries[:*limit]
	}

	t := display.NewTable([]string{"Name", "Clients", "Tests", "Pass", "Fail", "When", "File"})
	for _, e := range entries {
		t.Append([]string{
			e.Name,
			strings.Join(e.Clients, ","),
			fmt.Sprintf("%d", e.NTests),
			fmt.Sprintf("%d", e.Passes),
			fmt.Sprintf("%d", e.Fails),
			api.FormatTime(e.Start),
			e.FileName,
		})
	}
	t.Render()
}
