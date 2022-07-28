package optimism

import "github.com/ethereum/hive/hivesim"

type ClientsByRole struct {
	Eth1        []*hivesim.ClientDefinition
	OpL2        []*hivesim.ClientDefinition
	OpNode      []*hivesim.ClientDefinition
	OpProposer  []*hivesim.ClientDefinition
	OpBatcher   []*hivesim.ClientDefinition
	OpContracts []*hivesim.ClientDefinition
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
	}
	return &out
}
