package clients

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/exp/slices"

	"github.com/ethereum/go-ethereum/core/types"
	cg "github.com/ethereum/hive/simulators/eth2/common/chain_generators"
	"github.com/ethereum/hive/simulators/eth2/common/utils"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
)

// Describe a node setup, which consists of:
// - Execution Client
// - Beacon Client
// - Validator Client
type NodeDefinition struct {
	// Client Types
	ExecutionClient string
	ConsensusClient string
	ValidatorClient string

	// Execution Config
	ExecutionClientTTD *big.Int
	ChainGenerator     cg.ChainGenerator
	Chain              []*types.Block

	// Beacon Config
	BeaconNodeTTD *big.Int

	// Validator Config
	ValidatorShares uint64

	// Node Config
	TestVerificationNode bool
	DisableStartup       bool

	// Subnet Configuration
	ExecutionSubnet string
	ConsensusSubnet string
	Subnet          string
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

// A node bundles together:
// - Running Execution client
// - Running Beacon client
// - Running Validator client
// Contains a flag that marks a node that can be used to query
// test verification information.
type Node struct {
	Logging         utils.Logging
	Index           int
	ExecutionClient *ExecutionClient
	BeaconClient    *BeaconClient
	ValidatorClient *ValidatorClient
	Verification    bool
}

func (n *Node) Logf(format string, values ...interface{}) {
	if l := n.Logging; l != nil {
		l.Logf(format, values...)
	}
}

// Starts all clients included in the bundle
func (n *Node) Start() error {
	n.Logf("Starting validator client bundle %d", n.Index)
	if n.ExecutionClient != nil {
		if err := n.ExecutionClient.Start(); err != nil {
			return err
		}
	} else {
		n.Logf("No execution client started")
	}
	if n.BeaconClient != nil {
		if err := n.BeaconClient.Start(); err != nil {
			return err
		}
	} else {
		n.Logf("No beacon client started")
	}
	if n.ValidatorClient != nil {
		if err := n.ValidatorClient.Start(); err != nil {
			return err
		}
	} else {
		n.Logf("No validator client started")
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

func (n *Node) ClientNames() string {
	var name string
	if n.ExecutionClient != nil {
		name = n.ExecutionClient.ClientType()
	}
	if n.BeaconClient != nil {
		name = fmt.Sprintf("%s/%s", name, n.BeaconClient.ClientName())
	}
	return name
}

func (n *Node) IsRunning() bool {
	return n.ExecutionClient.IsRunning() && n.BeaconClient.IsRunning()
}

// Validator operations
func (n *Node) SignBLSToExecutionChange(
	ctx context.Context,
	blsToExecutionChangeInfo BLSToExecutionChangeInfo,
) (*common.SignedBLSToExecutionChange, error) {
	vc, bn := n.ValidatorClient, n.BeaconClient
	if !vc.ContainsValidatorIndex(blsToExecutionChangeInfo.ValidatorIndex) {
		return nil, fmt.Errorf(
			"validator does not contain specified validator index %d",
			blsToExecutionChangeInfo.ValidatorIndex,
		)
	}
	if domain, err := bn.ComputeDomain(
		ctx,
		common.DOMAIN_BLS_TO_EXECUTION_CHANGE,
		&bn.Config.Spec.GENESIS_FORK_VERSION,
	); err != nil {
		return nil, err
	} else {
		return vc.SignBLSToExecutionChange(domain, blsToExecutionChangeInfo)
	}
}

func (n *Node) SignSubmitBLSToExecutionChanges(
	ctx context.Context,
	blsToExecutionChangesInfo []BLSToExecutionChangeInfo,
) error {
	l := make(common.SignedBLSToExecutionChanges, 0)
	for _, c := range blsToExecutionChangesInfo {
		blsToExecChange, err := n.SignBLSToExecutionChange(
			ctx,
			c,
		)
		if err != nil {
			return err
		}
		l = append(l, *blsToExecChange)
	}

	return n.BeaconClient.SubmitPoolBLSToExecutionChange(ctx, l)
}

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
			ps = append(ps, n.ExecutionClient)
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

// Return subset of nodes that are currently running
func (all Nodes) Running() Nodes {
	res := make(Nodes, 0)
	for _, n := range all {
		if n.IsRunning() {
			res = append(res, n)
		}
	}
	return res
}

func (all Nodes) FilterByCL(filters []string) Nodes {
	ret := make(Nodes, 0)
	for _, n := range all {
		for _, filter := range filters {
			if strings.Contains(n.BeaconClient.ClientName(), filter) {
				ret = append(ret, n)
				break
			}
		}
	}
	return ret
}

func (all Nodes) FilterByEL(filters []string) Nodes {
	ret := make(Nodes, 0)
	for _, n := range all {
		for _, filter := range filters {
			if strings.Contains(n.ExecutionClient.ClientType(), filter) {
				ret = append(ret, n)
				break
			}
		}
	}
	return ret
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

func (all Nodes) SignSubmitBLSToExecutionChanges(
	ctx context.Context,
	blsToExecutionChanges []BLSToExecutionChangeInfo,
) error {
	// First gather all signed changes
	l := make(common.SignedBLSToExecutionChanges, 0)
	for _, c := range blsToExecutionChanges {
		n := all.ByValidatorIndex(c.ValidatorIndex)
		if n == nil {
			return fmt.Errorf(
				"validator index %d not found",
				c.ValidatorIndex,
			)
		}
		blsToExecChange, err := n.SignBLSToExecutionChange(
			ctx,
			c,
		)
		if err != nil {
			return err
		}
		l = append(l, *blsToExecChange)
	}
	// Then send the signed changes
	for _, n := range all {
		if err := n.BeaconClient.SubmitPoolBLSToExecutionChange(ctx, l); err != nil {
			return err
		}
	}
	return nil
}
