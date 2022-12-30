package clients

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/hivesim"
	cg "github.com/ethereum/hive/simulators/eth2/common/chain_generators"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
)

// Describe a node setup, which consists of:
// - Execution Client
// - Beacon Client
// - Validator Client
type NodeDefinition struct {
	ExecutionClient      string
	ConsensusClient      string
	ValidatorClient      string
	ValidatorShares      uint64
	ExecutionClientTTD   *big.Int
	BeaconNodeTTD        *big.Int
	TestVerificationNode bool
	DisableStartup       bool
	ChainGenerator       cg.ChainGenerator
	Chain                []*types.Block
	ExecutionSubnet      string
}

func (n *NodeDefinition) String() string {
	return fmt.Sprintf("%s-%s", n.ConsensusClient, n.ExecutionClient)
}

func (n *NodeDefinition) ExecutionClientName() string {
	return n.ExecutionClient
}

func (n *NodeDefinition) ConsensusClientName() string {
	return fmt.Sprintf("%s-bn", n.ConsensusClient)
}

func (n *NodeDefinition) ValidatorClientName() string {
	if n.ValidatorClient == "" {
		return fmt.Sprintf("%s-vc", n.ConsensusClient)
	}
	return fmt.Sprintf("%s-vc", n.ValidatorClient)
}

type NodeDefinitions []NodeDefinition

func (nodes NodeDefinitions) Shares() []uint64 {
	shares := make([]uint64, len(nodes))
	for i, n := range nodes {
		shares[i] = n.ValidatorShares
	}
	return shares
}

// A node bundles together:
// - Running Execution client
// - Running Beacon client
// - Running Validator client
// Contains a flag that marks a node that can be used to query
// test verification information.
type Node struct {
	T               *hivesim.T
	Index           int
	ExecutionClient *ExecutionClient
	BeaconClient    *BeaconClient
	ValidatorClient *ValidatorClient
	Verification    bool
}

// Starts all clients included in the bundle
func (n *Node) Start(extraOptions ...hivesim.StartOption) error {
	n.T.Logf("Starting validator client bundle %d", n.Index)
	if n.ExecutionClient != nil {
		if err := n.ExecutionClient.Start(extraOptions...); err != nil {
			return err
		}
	} else {
		n.T.Logf("No execution client started")
	}
	if n.BeaconClient != nil {
		if err := n.BeaconClient.Start(extraOptions...); err != nil {
			return err
		}
	} else {
		n.T.Logf("No beacon client started")
	}
	if n.ValidatorClient != nil {
		if err := n.ValidatorClient.Start(extraOptions...); err != nil {
			return err
		}
	} else {
		n.T.Logf("No validator client started")
	}
	return nil
}

func (n *Node) Shutdown() error {
	if err := n.ExecutionClient.Shutdown(); err != nil {
		return err
	}
	if err := n.BeaconClient.Shutdown(); err != nil {
		return err
	}
	if err := n.ValidatorClient.Shutdown(); err != nil {
		return err
	}
	return nil
}

// Validator operations
func (n *Node) SignVoluntaryExit(
	ctx context.Context,
	epoch common.Epoch,
	validatorIndex common.ValidatorIndex,
) (*phase0.SignedVoluntaryExit, error) {
	vc, bn := n.ValidatorClient, n.BeaconClient
	if !vc.ContainsValidatorIndex(validatorIndex) {
		return nil, fmt.Errorf(
			"validator does not contain specified validator index %d",
			validatorIndex,
		)
	}
	if domain, err := bn.ComputeDomain(
		ctx,
		common.DOMAIN_VOLUNTARY_EXIT,
		nil,
	); err != nil {
		return nil, err
	} else {
		return vc.SignVoluntaryExit(domain, epoch, validatorIndex)
	}
}

func (n *Node) SignSubmitVoluntaryExit(
	ctx context.Context,
	epoch common.Epoch,
	validatorIndex common.ValidatorIndex,
) error {
	exit, err := n.SignVoluntaryExit(ctx, epoch, validatorIndex)
	if err != nil {
		return err
	}
	return n.BeaconClient.SubmitVoluntaryExit(ctx, exit)
}

// Node cluster operations
type Nodes []*Node

// Return all execution clients, even the ones not currently running
func (all Nodes) ExecutionClients() ExecutionClients {
	en := make(ExecutionClients, 0)
	for _, n := range all {
		if n.ExecutionClient != nil {
			en = append(en, n.ExecutionClient)
		}
	}
	return en
}

// Return all proxy pointers, even the ones not currently running
func (all Nodes) Proxies() Proxies {
	ps := make(Proxies, 0)
	for _, n := range all {
		if n.ExecutionClient != nil {
			ps = append(ps, n.ExecutionClient.proxy)
		}
	}
	return ps
}

// Return all beacon clients, even the ones not currently running
func (all Nodes) BeaconClients() BeaconClients {
	bn := make(BeaconClients, 0)
	for _, n := range all {
		if n.BeaconClient != nil {
			bn = append(bn, n.BeaconClient)
		}
	}
	return bn
}

// Return all validator clients, even the ones not currently running
func (all Nodes) ValidatorClients() ValidatorClients {
	vc := make(ValidatorClients, 0)
	for _, n := range all {
		if n.ValidatorClient != nil {
			vc = append(vc, n.ValidatorClient)
		}
	}
	return vc
}

// Return subset of nodes which are marked as verification nodes
func (all Nodes) VerificationNodes() Nodes {
	// If none is set as verification, then all are verification nodes
	var any bool
	for _, n := range all {
		if n.Verification {
			any = true
			break
		}
	}
	if !any {
		return all
	}

	res := make(Nodes, 0)
	for _, n := range all {
		if n.Verification {
			res = append(res, n)
		}
	}
	return res
}

func (all Nodes) RemoveNodeAsVerifier(id int) error {
	if id >= len(all) {
		return fmt.Errorf("node %d does not exist", id)
	}
	var any bool
	for _, n := range all {
		if n.Verification {
			any = true
			break
		}
	}
	if any {
		all[id].Verification = false
	} else {
		// If no node is set as verifier, we will set all other nodes as verifiers then
		for i := range all {
			all[i].Verification = (i != id)
		}
	}
	return nil
}

func (all Nodes) ByValidatorIndex(validatorIndex common.ValidatorIndex) *Node {
	for _, n := range all {
		if n.ValidatorClient.ContainsValidatorIndex(validatorIndex) {
			return n
		}
	}
	return nil
}
