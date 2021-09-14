package main

import "github.com/ethereum/hive/hivesim"

type ClientDefinitionsByRole struct {
	beacon    []*hivesim.ClientDefinition
	validator []*hivesim.ClientDefinition
	eth1      []*hivesim.ClientDefinition
	other     []*hivesim.ClientDefinition
}

func ClientsByRole(available []*hivesim.ClientDefinition) *ClientDefinitionsByRole {
	var out ClientDefinitionsByRole
	for _, client := range available {
		if client.HasRole("beacon") {
			out.beacon = append(out.beacon, client)
		}
		if client.HasRole("validator") {
			out.validator = append(out.validator, client)
		}
		if client.HasRole("eth1") {
			out.eth1 = append(out.eth1, client)
		}
	}
	return &out
}
