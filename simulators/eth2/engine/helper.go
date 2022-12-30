package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	el "github.com/ethereum/hive/simulators/eth2/common/config/execution"
	"github.com/ethereum/hive/simulators/eth2/common/testnet"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

type testSpec struct {
	Name  string
	About string
	Run   func(*hivesim.T, *testnet.Environment, clients.NodeDefinition)
}

// API call names
const (
	EngineForkchoiceUpdatedV1 = "engine_forkchoiceUpdatedV1"
	EngineGetPayloadV1        = "engine_getPayloadV1"
	EngineNewPayloadV1        = "engine_newPayloadV1"
	EthGetBlockByHash         = "eth_getBlockByHash"
	EthGetBlockByNumber       = "eth_getBlockByNumber"
)

// Engine API Types

type PayloadStatus string

const (
	Unknown          = ""
	Valid            = "VALID"
	Invalid          = "INVALID"
	Accepted         = "ACCEPTED"
	Syncing          = "SYNCING"
	InvalidBlockHash = "INVALID_BLOCK_HASH"
)

// Signer for all txs
type Signer struct {
	ChainID    *big.Int
	PrivateKey *ecdsa.PrivateKey
}

func (vs Signer) SignTx(
	baseTx *types.Transaction,
) (*types.Transaction, error) {
	signer := types.NewEIP155Signer(vs.ChainID)
	return types.SignTx(baseTx, signer, vs.PrivateKey)
}

var VaultSigner = Signer{
	ChainID:    CHAIN_ID,
	PrivateKey: VAULT_KEY,
}

// Try to approximate how much time until the merge based on current time, bellatrix fork epoch,
// TTD, execution clients' consensus mechanism, current total difficulty.
// This function is used to calculate timeouts, so it will always return a pessimistic value.
func SlotsUntilMerge(
	parentCtx context.Context,
	t *testnet.Testnet,
	c *testnet.Config,
) beacon.Slot {
	l := make([]beacon.Slot, 0)
	l = append(l, SlotsUntilBellatrix(t.GenesisTime(), t.Spec().Spec))

	for _, e := range t.ExecutionClients().Running() {
		l = append(
			l,
			beacon.Slot(
				TimeUntilTerminalBlock(
					parentCtx,
					e,
					c.Eth1Consensus,
					c.TerminalTotalDifficulty,
				)/uint64(
					t.Spec().SECONDS_PER_SLOT,
				),
			),
		)
	}

	// Return the worst case
	max := beacon.Slot(0)
	for _, s := range l {
		if s > max {
			max = s
		}
	}

	fmt.Printf("INFO: Estimated slots until merge %d\n", max)

	// Add more slots give it some wiggle room
	return max + 5
}

func SlotsUntilBellatrix(
	genesisTime beacon.Timestamp,
	spec *beacon.Spec,
) beacon.Slot {
	currentTimestamp := beacon.Timestamp(time.Now().Unix())
	bellatrixTime, err := spec.TimeAtSlot(
		beacon.Slot(
			spec.BELLATRIX_FORK_EPOCH*beacon.Epoch(spec.SLOTS_PER_EPOCH),
		),
		genesisTime,
	)
	if err != nil {
		panic(err)
	}
	if currentTimestamp >= bellatrixTime {
		return beacon.Slot(0)
	}
	s := beacon.Slot(
		(bellatrixTime-currentTimestamp)/spec.SECONDS_PER_SLOT,
	) + 1
	fmt.Printf(
		"INFO: bellatrixTime:%d, currentTimestamp:%d, slots=%d\n",
		bellatrixTime,
		currentTimestamp,
		s,
	)
	return s
}

func TimeUntilTerminalBlock(
	parentCtx context.Context,
	e *clients.ExecutionClient,
	c el.ExecutionConsensus,
	defaultTTD *big.Int,
) uint64 {
	ttd := defaultTTD
	if e.ConfiguredTTD() != nil {
		ttd = e.ConfiguredTTD()
	}
	if td, err := e.TotalDifficultyByNumber(parentCtx, nil); err != nil ||
		td.Cmp(ttd) >= 0 {
		// TTD already reached
		return 0
	} else {
		fmt.Printf("INFO: ttd:%d, td:%d, diffPerBlock:%d, secondsPerBlock:%d\n", ttd, td, c.DifficultyPerBlock(), c.SecondsPerBlock())
		td.Sub(ttd, td).Div(td, c.DifficultyPerBlock()).Mul(td, big.NewInt(int64(c.SecondsPerBlock())))
		return td.Uint64()
	}
}
