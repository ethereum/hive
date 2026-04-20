package main

import (
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/ethereum/hive/cmd/hq/internal/api"
	"github.com/ethereum/hive/cmd/hq/internal/display"
)

func clientsCommand(args []string) {
	fs := flag.NewFlagSet("clients", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: hq clients [flags]")
		fmt.Fprintln(fs.Output(), "\nList clients seen in recent runs, along with any known short aliases.")
		fs.PrintDefaults()
	}
	addGlobalFlags(fs)
	fs.Parse(args)
	applyGlobals()

	client, err := newClient()
	if err != nil {
		fatalf("%v", err)
	}

	// Fetch recent runs to discover active clients.
	entries, err := client.FetchListing("", "", 0)
	if err != nil {
		fatalf("fetching listing: %v", err)
	}

	seen := make(map[string]bool)
	for _, e := range entries {
		for _, c := range e.Clients {
			seen[c] = true
		}
	}

	// Build reverse map: canonical prefix -> list of aliases.
	aliases := api.ClientAliases()
	reverseAliases := make(map[string][]string)
	for alias, canon := range aliases {
		if alias != canon {
			reverseAliases[canon] = append(reverseAliases[canon], alias)
		}
	}
	for _, v := range reverseAliases {
		sort.Strings(v)
	}

	var clients []string
	for c := range seen {
		clients = append(clients, c)
	}
	sort.Strings(clients)

	t := display.NewTable([]string{"Client", "Alias"})
	for _, c := range clients {
		prefix := strings.TrimSuffix(c, "_default")
		prefix = strings.Split(prefix, "_")[0]
		aliasStr := ""
		if shortNames, ok := reverseAliases[prefix]; ok {
			aliasStr = strings.Join(shortNames, ", ")
		}
		t.Append([]string{c, aliasStr})
	}
	t.Render()
}
