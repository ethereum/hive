package clients

import (
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/exp/slices"

	"github.com/ethereum/go-ethereum/core/types"
	cg "github.com/ethereum/hive/simulators/eth2/common/chain_generators"
)

// Describe a node setup, which consists of:
// - Execution Client
// - Beacon Client
// - Validator Client
type NodeDefinition struct {
	// Client Types
	ExecutionClient string `json:"execution_client"`
	ConsensusClient string `json:"consensus_client"`
	ValidatorClient string `json:"validator_client"`

	// Execution Config
	ExecutionClientTTD *big.Int          `json:"execution_client_ttd,omitempty"`
	ChainGenerator     cg.ChainGenerator `json:"-"`
	Chain              []*types.Block    `json:"chain,omitempty"`

	// Beacon Config
	BeaconNodeTTD *big.Int `json:"beacon_node_ttd,omitempty"`

	// Validator Config
	ValidatorShares uint64 `json:"validator_shares"`

	// Node Config
	TestVerificationNode bool `json:"test_verification_node"`
	DisableStartup       bool `json:"disable_startup"`

	// Subnet Configuration
	ExecutionSubnet string `json:"execution_subnet"`
	ConsensusSubnet string `json:"consensus_subnet"`
	Subnet          string `json:"subnet"`
}

func (n *NodeDefinition) String() string {
	return fmt.Sprintf("%s-%s", n.ConsensusClient, n.ExecutionClient)
}

func (n *NodeDefinition) ExecutionClientName() string {
	return n.ExecutionClient
}

func (n *NodeDefinition) ConsensusClientName() string {
	return n.ConsensusClient
}

func (n *NodeDefinition) ValidatorClientName() string {
	if n.ValidatorClient == "" {
		return beaconNodeToValidator(n.ConsensusClient)
	}
	return n.ValidatorClient
}

func (n *NodeDefinition) GetExecutionSubnet() string {
	if n.ExecutionSubnet != "" {
		return n.ExecutionSubnet
	}
	if n.Subnet != "" {
		return n.Subnet
	}
	return ""
}

func (n *NodeDefinition) GetConsensusSubnet() string {
	if n.ConsensusSubnet != "" {
		return n.ConsensusSubnet
	}
	if n.Subnet != "" {
		return n.Subnet
	}
	return ""
}

func beaconNodeToValidator(name string) string {
	name, branch, hasBranch := strings.Cut(name, "_")
	name = strings.TrimSuffix(name, "-bn")
	validator := name + "-vc"
	if hasBranch {
		validator += "_" + branch
	}
	return validator
}

type NodeDefinitions []NodeDefinition

func (nodes NodeDefinitions) ClientTypes() []string {
	types := make([]string, 0)
	for _, n := range nodes {
		if !slices.Contains(types, n.ExecutionClient) {
			types = append(types, n.ExecutionClient)
		}
		if !slices.Contains(types, n.ConsensusClient) {
			types = append(types, n.ConsensusClient)
		}
	}
	return types
}

func (nodes NodeDefinitions) Shares() []uint64 {
	shares := make([]uint64, len(nodes))
	for i, n := range nodes {
		shares[i] = n.ValidatorShares
	}
	return shares
}

func (all NodeDefinitions) FilterByCL(filters []string) NodeDefinitions {
	ret := make(NodeDefinitions, 0)
	for _, n := range all {
		for _, filter := range filters {
			if strings.Contains(n.ConsensusClient, filter) {
				ret = append(ret, n)
				break
			}
		}
	}
	return ret
}

func (all NodeDefinitions) FilterByEL(filters []string) NodeDefinitions {
	ret := make(NodeDefinitions, 0)
	for _, n := range all {
		for _, filter := range filters {
			if strings.Contains(n.ExecutionClient, filter) {
				ret = append(ret, n)
				break
			}
		}
	}
	return ret
}
