package main

import "github.com/ethereum/hive/hivesim"

type ClientDefinitionsByRole struct {
	Beacon    []*hivesim.ClientDefinition `json:"beacon"`
	Validator []*hivesim.ClientDefinition `json:"validator"`
	Eth1      []*hivesim.ClientDefinition `json:"eth1"`
	Other     []*hivesim.ClientDefinition `json:"Other"`
}

func ClientsByRole(available []*hivesim.ClientDefinition) *ClientDefinitionsByRole {
	var out ClientDefinitionsByRole
	for _, client := range available {
		if client.HasRole("beacon") {
			out.Beacon = append(out.Beacon, client)
		}
		if client.HasRole("validator") {
			out.Validator = append(out.Validator, client)
		}
		if client.HasRole("eth1") {
			out.Eth1 = append(out.Eth1, client)
		}
	}
	return &out
}
