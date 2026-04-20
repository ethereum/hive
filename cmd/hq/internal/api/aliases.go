package api

import "strings"

// clientAliases maps shorthand names to the canonical client name prefix.
var clientAliases = map[string]string{
	"geth":       "go-ethereum",
	"nimbus":     "nimbus-el",
	"nethermind": "nethermind",
	"besu":       "besu",
	"reth":       "reth",
	"erigon":     "erigon",
	"ethrex":     "ethrex",
}

// ResolveClientAlias returns the canonical client name prefix for the given
// alias. If no alias matches, the input is returned as-is.
func ResolveClientAlias(name string) string {
	if canon, ok := clientAliases[strings.ToLower(name)]; ok {
		return canon
	}
	return name
}

// ClientAliases returns a map of alias -> canonical name for display.
func ClientAliases() map[string]string {
	return clientAliases
}
