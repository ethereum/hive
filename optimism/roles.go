package optimism

import (
	"fmt"
	"github.com/ethereum/hive/hivesim"
	"strings"
)

type ClientsByRole struct {
	Eth1        []*hivesim.ClientDefinition
	OpL2        []*hivesim.ClientDefinition
	OpNode      []*hivesim.ClientDefinition
	OpProposer  []*hivesim.ClientDefinition
	OpBatcher   []*hivesim.ClientDefinition
	OpContracts []*hivesim.ClientDefinition
}

func stringifyClientDefs(clientDefs []*hivesim.ClientDefinition) string {
	var out []string
	for _, c := range clientDefs {
		out = append(out, c.Name)
	}
	return strings.Join(out, ", ")
}

func (cr *ClientsByRole) String() string {
	out := "eth1: " + stringifyClientDefs(cr.Eth1)
	out += ", op-l2: " + stringifyClientDefs(cr.OpL2)
	out += ", op-node: " + stringifyClientDefs(cr.OpNode)
	out += ", op-proposer: " + stringifyClientDefs(cr.OpProposer)
	out += ", op-batcher: " + stringifyClientDefs(cr.OpBatcher)
	out += ", op-contracts: " + stringifyClientDefs(cr.OpContracts)
	return out
}

func Roles(clientDefs []*hivesim.ClientDefinition) *ClientsByRole {
	var out ClientsByRole
	for _, client := range clientDefs {
		if client.HasRole("eth1") {
			out.Eth1 = append(out.Eth1, client)
		}
		if client.HasRole("op-l2") {
			out.OpL2 = append(out.OpL2, client)
		}
		if client.HasRole("op-node") {
			out.OpNode = append(out.OpNode, client)
		}
		if client.HasRole("op-proposer") {
			out.OpProposer = append(out.OpProposer, client)
		}
		if client.HasRole("op-batcher") {
			out.OpBatcher = append(out.OpBatcher, client)
		}
		if client.HasRole("op-contracts") {
			out.OpContracts = append(out.OpContracts, client)
		}
		fmt.Println("client: ", client.Name, " meta: ", client.Meta, " version: ", client.Version)
	}
	return &out
}
