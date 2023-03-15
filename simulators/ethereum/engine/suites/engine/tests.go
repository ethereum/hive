package suite_engine

import (
	"context"
	"encoding/json"
	"math/big"
	"math/rand"
	"time"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
	"github.com/ethereum/hive/simulators/ethereum/engine/client/node"
	client_types "github.com/ethereum/hive/simulators/ethereum/engine/client/types"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Execution specification reference:
// https://github.com/ethereum/execution-apis/blob/main/src/engine/specification.md

var (
	big0      = new(big.Int)
	big1      = big.NewInt(1)
	Head      *big.Int // Nil
	Pending   = big.NewInt(-2)
	Finalized = big.NewInt(-3)
	Safe      = big.NewInt(-4)
	ZeroAddr  = common.Address{}
)

var Tests = []test.Spec{
	// Engine API Negative Test Cases
	{
		Name: "Invalid Terminal Block in ForkchoiceUpdated",
		Run:  invalidTerminalBlockForkchoiceUpdated,
		TTD:  1000000,
	},
	{
		Name: "Invalid GetPayload Under PoW",
		Run:  invalidGetPayloadUnderPoW,
		TTD:  1000000,
	},
	{
		Name: "Invalid Terminal Block in NewPayload",
		Run:  invalidTerminalBlockNewPayload,
		TTD:  1000000,
	},
	{
		Name: "Inconsistent Head in ForkchoiceState",
		Run:  inconsistentForkchoiceStateGen("Head"),
	},
	{
		Name: "Inconsistent Safe in ForkchoiceState",
		Run:  inconsistentForkchoiceStateGen("Safe"),
	},
	{
		Name: "Inconsistent Finalized in ForkchoiceState",
		Run:  inconsistentForkchoiceStateGen("Finalized"),
	},
	{
		Name: "Unknown HeadBlockHash",
		Run:  unknownHeadBlockHash,
	},
	{
		Name: "Unknown SafeBlockHash",
		Run:  unknownSafeBlockHash,
	},
	{
		Name: "Unknown FinalizedBlockHash",
		Run:  unknownFinalizedBlockHash,
	},
	{
		Name: "ForkchoiceUpdated Invalid Payload Attributes",
		Run:  invalidPayloadAttributesGen(false),
	},
	{
		Name: "ForkchoiceUpdated Invalid Payload Attributes (Syncing)",
		Run:  invalidPayloadAttributesGen(true),
	},
	{
		Name: "Pre-TTD ForkchoiceUpdated After PoS Switch",
		Run:  preTTDFinalizedBlockHash,
		TTD:  2,
	},
	// Invalid Payload Tests
	{
		Name: "Bad Hash on NewPayload",
		Run:  badHashOnNewPayloadGen(false, false),
	},
	{
		Name: "Bad Hash on NewPayload Syncing",
		Run:  badHashOnNewPayloadGen(true, false),
	},
	{
		Name: "Bad Hash on NewPayload Side Chain",
		Run:  badHashOnNewPayloadGen(false, true),
	},
	{
		Name: "Bad Hash on NewPayload Side Chain Syncing",
		Run:  badHashOnNewPayloadGen(true, true),
	},
	{
		Name: "ParentHash==BlockHash on NewPayload",
		Run:  parentHashOnExecPayload,
	},
	{
		Name:      "Invalid Transition Payload",
		Run:       invalidTransitionPayload,
		TTD:       393504,
		ChainFile: "blocks_2_td_393504.rlp",
	},
	{
		Name: "Invalid ParentHash NewPayload",
		Run:  invalidPayloadTestCaseGen(helper.InvalidParentHash, false, false),
	},
	{
		Name: "Invalid ParentHash NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen(helper.InvalidParentHash, true, false),
	},
	{
		Name: "Invalid StateRoot NewPayload",
		Run:  invalidPayloadTestCaseGen(helper.InvalidStateRoot, false, false),
	},
	{
		Name: "Invalid StateRoot NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen(helper.InvalidStateRoot, true, false),
	},
	{
		Name: "Invalid StateRoot NewPayload, Empty Transactions",
		Run:  invalidPayloadTestCaseGen(helper.InvalidStateRoot, false, true),
	},
	{
		Name: "Invalid StateRoot NewPayload, Empty Transactions (Syncing)",
		Run:  invalidPayloadTestCaseGen(helper.InvalidStateRoot, true, true),
	},
	{
		Name: "Invalid ReceiptsRoot NewPayload",
		Run:  invalidPayloadTestCaseGen(helper.InvalidReceiptsRoot, false, false),
	},
	{
		Name: "Invalid ReceiptsRoot NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen(helper.InvalidReceiptsRoot, true, false),
	},
	{
		Name: "Invalid Number NewPayload",
		Run:  invalidPayloadTestCaseGen(helper.InvalidNumber, false, false),
	},
	{
		Name: "Invalid Number NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen(helper.InvalidNumber, true, false),
	},
	{
		Name: "Invalid GasLimit NewPayload",
		Run:  invalidPayloadTestCaseGen(helper.InvalidGasLimit, false, false),
	},
	{
		Name: "Invalid GasLimit NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen(helper.InvalidGasLimit, true, false),
	},
	{
		Name: "Invalid GasUsed NewPayload",
		Run:  invalidPayloadTestCaseGen(helper.InvalidGasUsed, false, false),
	},
	{
		Name: "Invalid GasUsed NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen(helper.InvalidGasUsed, true, false),
	},
	{
		Name: "Invalid Timestamp NewPayload",
		Run:  invalidPayloadTestCaseGen(helper.InvalidTimestamp, false, false),
	},
	{
		Name: "Invalid Timestamp NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen(helper.InvalidTimestamp, true, false),
	},
	{
		Name: "Invalid PrevRandao NewPayload",
		Run:  invalidPayloadTestCaseGen(helper.InvalidPrevRandao, false, false),
	},
	{
		Name: "Invalid PrevRandao NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen(helper.InvalidPrevRandao, true, false),
	},
	{
		Name: "Invalid Incomplete Transactions NewPayload",
		Run:  invalidPayloadTestCaseGen(helper.RemoveTransaction, false, false),
	},
	{
		Name: "Invalid Incomplete Transactions NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen(helper.RemoveTransaction, true, false),
	},
	{
		Name:                "Invalid Transaction Signature NewPayload",
		Run:                 invalidPayloadTestCaseGen(helper.InvalidTransactionSignature, false, false),
		TestTransactionType: helper.LegacyTxOnly,
	},
	{
		Name:                "Invalid Transaction Signature NewPayload (EIP-1559)",
		Run:                 invalidPayloadTestCaseGen(helper.InvalidTransactionSignature, false, false),
		TestTransactionType: helper.DynamicFeeTxOnly,
	},
	{
		Name:                "Invalid Transaction Signature NewPayload (Syncing)",
		Run:                 invalidPayloadTestCaseGen(helper.InvalidTransactionSignature, true, false),
		TestTransactionType: helper.LegacyTxOnly,
	},
	{
		Name:                "Invalid Transaction Nonce NewPayload",
		Run:                 invalidPayloadTestCaseGen(helper.InvalidTransactionNonce, false, false),
		TestTransactionType: helper.LegacyTxOnly,
	},
	{
		Name:                "Invalid Transaction Nonce NewPayload (EIP-1559)",
		Run:                 invalidPayloadTestCaseGen(helper.InvalidTransactionNonce, false, false),
		TestTransactionType: helper.DynamicFeeTxOnly,
	},
	{
		Name:                "Invalid Transaction Nonce NewPayload (Syncing)",
		Run:                 invalidPayloadTestCaseGen(helper.InvalidTransactionNonce, true, false),
		TestTransactionType: helper.LegacyTxOnly,
	},
	{
		Name:                "Invalid Transaction GasPrice NewPayload",
		Run:                 invalidPayloadTestCaseGen(helper.InvalidTransactionGasPrice, false, false),
		TestTransactionType: helper.LegacyTxOnly,
	},
	{
		Name:                "Invalid Transaction GasPrice NewPayload (EIP-1559)",
		Run:                 invalidPayloadTestCaseGen(helper.InvalidTransactionGasPrice, false, false),
		TestTransactionType: helper.DynamicFeeTxOnly,
	},
	{
		Name:                "Invalid Transaction GasPrice NewPayload (Syncing)",
		Run:                 invalidPayloadTestCaseGen(helper.InvalidTransactionGasPrice, true, false),
		TestTransactionType: helper.LegacyTxOnly,
	},
	{
		Name:                "Invalid Transaction Gas Tip NewPayload (EIP-1559)",
		Run:                 invalidPayloadTestCaseGen(helper.InvalidTransactionGasTipPrice, false, false),
		TestTransactionType: helper.DynamicFeeTxOnly,
	},
	{
		Name:                "Invalid Transaction Gas Tip NewPayload (EIP-1559, Syncing)",
		Run:                 invalidPayloadTestCaseGen(helper.InvalidTransactionGasTipPrice, true, false),
		TestTransactionType: helper.DynamicFeeTxOnly,
	},
	{
		Name:                "Invalid Transaction Gas NewPayload",
		Run:                 invalidPayloadTestCaseGen(helper.InvalidTransactionGas, false, false),
		TestTransactionType: helper.LegacyTxOnly,
	},
	{
		Name:                "Invalid Transaction Gas NewPayload (EIP-1559)",
		Run:                 invalidPayloadTestCaseGen(helper.InvalidTransactionGas, false, false),
		TestTransactionType: helper.DynamicFeeTxOnly,
	},
	{
		Name:                "Invalid Transaction Gas NewPayload (Syncing)",
		Run:                 invalidPayloadTestCaseGen(helper.InvalidTransactionGas, true, false),
		TestTransactionType: helper.LegacyTxOnly,
	},
	{
		Name:                "Invalid Transaction Value NewPayload",
		Run:                 invalidPayloadTestCaseGen(helper.InvalidTransactionValue, false, false),
		TestTransactionType: helper.LegacyTxOnly,
	},
	{
		Name:                "Invalid Transaction Value NewPayload (EIP-1559)",
		Run:                 invalidPayloadTestCaseGen(helper.InvalidTransactionValue, false, false),
		TestTransactionType: helper.DynamicFeeTxOnly,
	},
	{
		Name:                "Invalid Transaction Value NewPayload (Syncing)",
		Run:                 invalidPayloadTestCaseGen(helper.InvalidTransactionValue, true, false),
		TestTransactionType: helper.LegacyTxOnly,
	},
	{
		Name:                "Invalid Transaction ChainID NewPayload",
		Run:                 invalidPayloadTestCaseGen(helper.InvalidTransactionChainID, false, false),
		TestTransactionType: helper.LegacyTxOnly,
	},
	{
		Name:                "Invalid Transaction ChainID NewPayload (EIP-1559)",
		Run:                 invalidPayloadTestCaseGen(helper.InvalidTransactionChainID, false, false),
		TestTransactionType: helper.DynamicFeeTxOnly,
	},
	{
		Name:                "Invalid Transaction ChainID NewPayload (Syncing)",
		Run:                 invalidPayloadTestCaseGen(helper.InvalidTransactionChainID, true, false),
		TestTransactionType: helper.LegacyTxOnly,
	},

	// Invalid Ancestor Re-Org Tests (Reveal via newPayload)
	{
		Name:             "Invalid Ancestor Chain Re-Org, Invalid StateRoot, Invalid P1', Reveal using newPayload",
		SlotsToFinalized: big.NewInt(20),
		Run:              invalidMissingAncestorReOrgGen(1, helper.InvalidStateRoot, true),
	},
	{
		Name:             "Invalid Ancestor Chain Re-Org, Invalid StateRoot, Invalid P9', Reveal using newPayload",
		SlotsToFinalized: big.NewInt(20),
		Run:              invalidMissingAncestorReOrgGen(9, helper.InvalidStateRoot, true),
	},
	{
		Name:             "Invalid Ancestor Chain Re-Org, Invalid StateRoot, Invalid P10', Reveal using newPayload",
		SlotsToFinalized: big.NewInt(20),
		Run:              invalidMissingAncestorReOrgGen(10, helper.InvalidStateRoot, true),
	},

	// Invalid Ancestor Re-Org/Sync Tests (Reveal via sync through secondary client)
	{
		Name:             "Invalid Ancestor Chain Re-Org, Invalid StateRoot, Invalid P9', Reveal using sync",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 9,
			PayloadField:        helper.InvalidStateRoot,
			ReOrgFromCanonical:  true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Sync, Invalid StateRoot, Invalid P9'",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 9,
			PayloadField:        helper.InvalidStateRoot,
			ReOrgFromCanonical:  false,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Re-Org, Invalid StateRoot, Empty Txs, Invalid P9', Reveal using sync",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 9,
			PayloadField:        helper.InvalidStateRoot,
			EmptyTransactions:   true,
			ReOrgFromCanonical:  true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Sync, Invalid StateRoot, Empty Txs, Invalid P9'",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 9,
			PayloadField:        helper.InvalidStateRoot,
			EmptyTransactions:   true,
			ReOrgFromCanonical:  false,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Re-Org, Invalid ReceiptsRoot, Invalid P8', Reveal using sync",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 8,
			PayloadField:        helper.InvalidReceiptsRoot,
			ReOrgFromCanonical:  true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Sync, Invalid ReceiptsRoot, Invalid P8'",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 8,
			PayloadField:        helper.InvalidReceiptsRoot,
			ReOrgFromCanonical:  false,
		}.GenerateSync(),
	},
	/*
		TODO, RE-ENABLE: Test is causing a panic on the secondary node, disabling for now.
		{
			Name:             "Invalid Ancestor Chain Re-Org, Invalid Number, Invalid P9', Reveal using sync",
			TimeoutSeconds:   30,
			SlotsToFinalized: big.NewInt(20),
			Run: InvalidMissingAncestorReOrgSpec{
				PayloadInvalidIndex: 9,
				PayloadField:        helper.InvalidNumber,
				ReOrgFromCanonical:  true,
			}.GenerateSync(),
		},
		{
			Name:             "Invalid Ancestor Chain Sync, Invalid Number, Invalid P9'",
			TimeoutSeconds:   30,
			SlotsToFinalized: big.NewInt(20),
			Run: InvalidMissingAncestorReOrgSpec{
				PayloadInvalidIndex: 9,
				PayloadField:        helper.InvalidNumber,
				ReOrgFromCanonical:  false,
			}.GenerateSync(),
		},
	*/
	{
		Name:             "Invalid Ancestor Chain Re-Org, Invalid GasLimit, Invalid P9', Reveal using sync",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 8,
			PayloadField:        helper.InvalidGasLimit,
			ReOrgFromCanonical:  true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Sync, Invalid GasLimit, Invalid P9'",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 8,
			PayloadField:        helper.InvalidGasLimit,
			ReOrgFromCanonical:  false,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Re-Org, Invalid GasUsed, Invalid P9', Reveal using sync",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 8,
			PayloadField:        helper.InvalidGasUsed,
			ReOrgFromCanonical:  true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Sync, Invalid GasUsed, Invalid P9'",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 8,
			PayloadField:        helper.InvalidGasUsed,
			ReOrgFromCanonical:  false,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Re-Org, Invalid Timestamp, Invalid P9', Reveal using sync",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 8,
			PayloadField:        helper.InvalidTimestamp,
			ReOrgFromCanonical:  true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Sync, Invalid Timestamp, Invalid P9'",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 8,
			PayloadField:        helper.InvalidTimestamp,
			ReOrgFromCanonical:  false,
		}.GenerateSync(),
	},
	/*
			TODO, RE-ENABLE: Test consistently fails with Failed to set invalid block: missing trie node.
		{
			Name:             "Invalid Ancestor Chain Re-Org, Invalid PrevRandao, Invalid P9', Reveal using sync",
			TimeoutSeconds:   30,
			SlotsToFinalized: big.NewInt(20),
			Run: InvalidMissingAncestorReOrgSpec{
				PayloadInvalidIndex: 8,
				PayloadField:        InvalidPrevRandao,
				ReOrgFromCanonical:  true,
			}.GenerateSync(),
		},
		{
			Name:             "Invalid Ancestor Chain Sync, Invalid PrevRandao, Invalid P9'",
			TimeoutSeconds:   30,
			SlotsToFinalized: big.NewInt(20),
			Run: InvalidMissingAncestorReOrgSpec{
				PayloadInvalidIndex: 8,
				PayloadField:        InvalidPrevRandao,
				ReOrgFromCanonical:  false,
			}.GenerateSync(),
		},
	*/
	{
		Name:             "Invalid Ancestor Chain Re-Org, Incomplete Transactions, Invalid P9', Reveal using sync",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 9,
			PayloadField:        helper.RemoveTransaction,
			ReOrgFromCanonical:  true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Sync, Incomplete Transactions, Invalid P9'",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 9,
			PayloadField:        helper.RemoveTransaction,
			ReOrgFromCanonical:  false,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Re-Org, Invalid Transaction Signature, Invalid P9', Reveal using sync",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 9,
			PayloadField:        helper.InvalidTransactionSignature,
			ReOrgFromCanonical:  true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Sync, Invalid Transaction Signature, Invalid P9'",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 9,
			PayloadField:        helper.InvalidTransactionSignature,
			ReOrgFromCanonical:  false,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Re-Org, Invalid Transaction Nonce, Invalid P9', Reveal using sync",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 9,
			PayloadField:        helper.InvalidTransactionNonce,
			ReOrgFromCanonical:  true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Sync, Invalid Transaction Nonce, Invalid P9'",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 9,
			PayloadField:        helper.InvalidTransactionNonce,
			ReOrgFromCanonical:  false,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Re-Org, Invalid Transaction Gas, Invalid P9', Reveal using sync",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 9,
			PayloadField:        helper.InvalidTransactionGas,
			ReOrgFromCanonical:  true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Sync, Invalid Transaction Gas, Invalid P9'",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 9,
			PayloadField:        helper.InvalidTransactionGas,
			ReOrgFromCanonical:  false,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Re-Org, Invalid Transaction GasPrice, Invalid P9', Reveal using sync",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 9,
			PayloadField:        helper.InvalidTransactionGasPrice,
			ReOrgFromCanonical:  true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Sync, Invalid Transaction GasPrice, Invalid P9'",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 9,
			PayloadField:        helper.InvalidTransactionGasPrice,
			ReOrgFromCanonical:  false,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Re-Org, Invalid Transaction Value, Invalid P9', Reveal using sync",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 9,
			PayloadField:        helper.InvalidTransactionValue,
			ReOrgFromCanonical:  true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Sync, Invalid Transaction Value, Invalid P9'",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 9,
			PayloadField:        helper.InvalidTransactionValue,
			ReOrgFromCanonical:  false,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Re-Org, Invalid Ommers, Invalid P9', Reveal using sync",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 9,
			PayloadField:        helper.InvalidOmmers,
			ReOrgFromCanonical:  true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Ancestor Chain Sync, Invalid Ommers, Invalid P9'",
		TimeoutSeconds:   30,
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex: 9,
			PayloadField:        helper.InvalidOmmers,
			ReOrgFromCanonical:  false,
		}.GenerateSync(),
	},

	// Invalid Transition Payload Re-Org/Sync Tests (Reveal via sync through secondary client)
	{
		Name:             "Invalid Transition Payload Re-Org, Invalid StateRoot, Reveal using sync",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(2),
			PayloadField:          helper.InvalidStateRoot,
			ReOrgFromCanonical:    true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Sync, Invalid StateRoot",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(2),
			PayloadField:          helper.InvalidStateRoot,
			ReOrgFromCanonical:    false,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Re-Org, Invalid StateRoot, Empty Txs, Reveal using sync",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(2),
			PayloadField:          helper.InvalidStateRoot,
			EmptyTransactions:     true,
			ReOrgFromCanonical:    true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Sync, Invalid StateRoot, Empty Txs",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(2),
			PayloadField:          helper.InvalidStateRoot,
			EmptyTransactions:     true,
			ReOrgFromCanonical:    false,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Re-Org, Invalid ReceiptsRoot, Reveal using sync",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(3),
			PayloadField:          helper.InvalidReceiptsRoot,
			ReOrgFromCanonical:    true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Sync, Invalid ReceiptsRoot",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(3),
			PayloadField:          helper.InvalidReceiptsRoot,
			ReOrgFromCanonical:    false,
		}.GenerateSync(),
	},
	/*
		TODO, RE-ENABLE: Test is causing a panic on the secondary node, disabling for now.
		{
			Name:             "Invalid Transition Payload Re-Org, Invalid Number, Reveal using sync",
			TimeoutSeconds:   30,
			TTD:              393504,
			ChainFile:        "blocks_2_td_393504.rlp",
			SlotsToFinalized: big.NewInt(20),
			Run: InvalidMissingAncestorReOrgSpec{
				PayloadInvalidIndex:     1, // Transition payload
				CommonAncestorHeight:  big.NewInt(0),
				DeviatingPayloadCount: big.NewInt(2),
				PayloadField:          helper.InvalidNumber,
				ReOrgFromCanonical:    true,
			}.GenerateSync(),
		},
		{
			Name:             "Invalid Transition Payload Sync, Invalid Number",
			TimeoutSeconds:   30,
			TTD:              393504,
			ChainFile:        "blocks_2_td_393504.rlp",
			SlotsToFinalized: big.NewInt(20),
			Run: InvalidMissingAncestorReOrgSpec{
				PayloadInvalidIndex:     1, // Transition payload
				CommonAncestorHeight:  big.NewInt(0),
				DeviatingPayloadCount: big.NewInt(2),
				PayloadField:          helper.InvalidNumber,
				ReOrgFromCanonical:    false,
			}.GenerateSync(),
		},
	*/
	{
		Name:             "Invalid Transition Payload Re-Org, Invalid GasLimit, Reveal using sync",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(3),
			PayloadField:          helper.InvalidGasLimit,
			ReOrgFromCanonical:    true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Sync, Invalid GasLimit",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(3),
			PayloadField:          helper.InvalidGasLimit,
			ReOrgFromCanonical:    false,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Re-Org, Invalid GasUsed, Reveal using sync",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(3),
			PayloadField:          helper.InvalidGasUsed,
			ReOrgFromCanonical:    true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Sync, Invalid GasUsed",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(3),
			PayloadField:          helper.InvalidGasUsed,
			ReOrgFromCanonical:    false,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Re-Org, Invalid Timestamp, Reveal using sync",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(3),
			PayloadField:          helper.InvalidTimestamp,
			ReOrgFromCanonical:    true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Sync, Invalid Timestamp",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(3),
			PayloadField:          helper.InvalidTimestamp,
			ReOrgFromCanonical:    false,
		}.GenerateSync(),
	},
	/*
		TODO, RE-ENABLE: Test consistently fails with Failed to set invalid block: missing trie node.
		{
			Name:             "Invalid Transition Payload Re-Org, Invalid PrevRandao, Reveal using sync",
			TimeoutSeconds:   30,
			TTD:              393504,
			ChainFile:        "blocks_2_td_393504.rlp",
			SlotsToFinalized: big.NewInt(20),
			Run: InvalidMissingAncestorReOrgSpec{
				PayloadInvalidIndex:     1, // Transition payload
				CommonAncestorHeight:  big.NewInt(0),
				DeviatingPayloadCount: big.NewInt(3),
				PayloadField:          helper.InvalidPrevRandao,
				ReOrgFromCanonical:    true,
			}.GenerateSync(),
		},
		{
			Name:             "Invalid Transition Payload Sync, Invalid PrevRandao",
			TimeoutSeconds:   30,
			TTD:              393504,
			ChainFile:        "blocks_2_td_393504.rlp",
			SlotsToFinalized: big.NewInt(20),
			Run: InvalidMissingAncestorReOrgSpec{
				PayloadInvalidIndex:     1, // Transition payload
				CommonAncestorHeight:  big.NewInt(0),
				DeviatingPayloadCount: big.NewInt(3),
				PayloadField:          helper.InvalidPrevRandao,
				ReOrgFromCanonical:    false,
			}.GenerateSync(),
		},
	*/
	{
		Name:             "Invalid Transition Payload Re-Org, Incomplete Transactions, Reveal using sync",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(2),
			PayloadField:          helper.RemoveTransaction,
			ReOrgFromCanonical:    true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Sync, Incomplete Transactions",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(2),
			PayloadField:          helper.RemoveTransaction,
			ReOrgFromCanonical:    false,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Re-Org, Invalid Transaction Signature, Reveal using sync",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(2),
			PayloadField:          helper.InvalidTransactionSignature,
			ReOrgFromCanonical:    true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Sync, Invalid Transaction Signature",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(2),
			PayloadField:          helper.InvalidTransactionSignature,
			ReOrgFromCanonical:    false,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Re-Org, Invalid Transaction Nonce, Reveal using sync",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(2),
			PayloadField:          helper.InvalidTransactionNonce,
			ReOrgFromCanonical:    true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Sync, Invalid Transaction Nonce",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(2),
			PayloadField:          helper.InvalidTransactionNonce,
			ReOrgFromCanonical:    false,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Re-Org, Invalid Transaction Gas, Reveal using sync",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(2),
			PayloadField:          helper.InvalidTransactionGas,
			ReOrgFromCanonical:    true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Sync, Invalid Transaction Gas",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(2),
			PayloadField:          helper.InvalidTransactionGas,
			ReOrgFromCanonical:    false,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Re-Org, Invalid Transaction GasPrice, Reveal using sync",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(2),
			PayloadField:          helper.InvalidTransactionGasPrice,
			ReOrgFromCanonical:    true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Sync, Invalid Transaction GasPrice",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(2),
			PayloadField:          helper.InvalidTransactionGasPrice,
			ReOrgFromCanonical:    false,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Re-Org, Invalid Transaction Value, Reveal using sync",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(2),
			PayloadField:          helper.InvalidTransactionValue,
			ReOrgFromCanonical:    true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Sync, Invalid Transaction Value",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(2),
			PayloadField:          helper.InvalidTransactionValue,
			ReOrgFromCanonical:    false,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Re-Org, Invalid Ommers, Reveal using sync",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(2),
			PayloadField:          helper.InvalidOmmers,
			ReOrgFromCanonical:    true,
		}.GenerateSync(),
	},
	{
		Name:             "Invalid Transition Payload Sync, Invalid Ommers",
		TimeoutSeconds:   30,
		TTD:              393504,
		ChainFile:        "blocks_2_td_393504.rlp",
		SlotsToFinalized: big.NewInt(20),
		Run: InvalidMissingAncestorReOrgSpec{
			PayloadInvalidIndex:   1, // Transition payload
			CommonAncestorHeight:  big.NewInt(0),
			DeviatingPayloadCount: big.NewInt(2),
			PayloadField:          helper.InvalidOmmers,
			ReOrgFromCanonical:    false,
		}.GenerateSync(),
	},

	// Eth RPC Status on ForkchoiceUpdated Events
	{
		Name: "latest Block after NewPayload",
		Run:  blockStatusExecPayloadGen(false),
	},
	{
		Name: "latest Block after NewPayload (Transition Block)",
		Run:  blockStatusExecPayloadGen(true),
		TTD:  5,
	},
	{
		Name: "latest Block after New HeadBlockHash",
		Run:  blockStatusHeadBlockGen(false),
	},
	{
		Name: "latest Block after New HeadBlockHash (Transition Block)",
		Run:  blockStatusHeadBlockGen(true),
		TTD:  5,
	},
	{
		Name: "safe Block after New SafeBlockHash",
		Run:  blockStatusSafeBlock,
		TTD:  5,
	},
	{
		Name: "finalized Block after New FinalizedBlockHash",
		Run:  blockStatusFinalizedBlock,
		TTD:  5,
	},
	{
		Name:             "safe, finalized on Canonical Chain",
		Run:              safeFinalizedCanonicalChain,
		SlotsToSafe:      big.NewInt(1),
		SlotsToFinalized: big.NewInt(2),
	},
	{
		Name: "latest Block after Reorg",
		Run:  blockStatusReorg,
	},

	// Payload Tests
	{
		Name: "Re-Execute Payload",
		Run:  reExecPayloads,
	},
	{
		Name: "Multiple New Payloads Extending Canonical Chain",
		Run:  multipleNewCanonicalPayloads,
	},
	{
		Name: "Consecutive Payload Execution",
		Run:  inOrderPayloads,
	},
	{
		Name: "Valid NewPayload->ForkchoiceUpdated on Syncing Client",
		Run:  validPayloadFcUSyncingClient,
	},
	{
		Name:      "NewPayload with Missing ForkchoiceUpdated",
		Run:       missingFcu,
		TTD:       393120,
		ChainFile: "blocks_2_td_393504.rlp",
	},
	{
		Name: "Payload Build after New Invalid Payload",
		Run:  payloadBuildAfterNewInvalidPayload,
	},
	{
		Name:                "Build Payload with Invalid ChainID Transaction (Legacy Tx)",
		Run:                 buildPayloadWithInvalidChainIDTx,
		TestTransactionType: helper.LegacyTxOnly,
	},
	{
		Name:                "Build Payload with Invalid ChainID Transaction (EIP-1559)",
		Run:                 buildPayloadWithInvalidChainIDTx,
		TestTransactionType: helper.DynamicFeeTxOnly,
	},

	// Re-org using Engine API
	{
		Name: "Transaction Reorg",
		Run:  transactionReorg,
	},
	{
		Name: "Transaction Reorg - Check Blockhash",
		Run:  transactionReorgBlockhash(false),
	},
	{
		Name: "Transaction Reorg - Check Blockhash with NP on revert",
		Run:  transactionReorgBlockhash(true),
	},
	{
		Name: "Sidechain Reorg",
		Run:  sidechainReorg,
	},
	{
		Name: "Re-Org Back into Canonical Chain",
		Run:  reorgBack,
	},
	{
		Name: "Re-Org Back to Canonical Chain From Syncing Chain",
		Run:  reorgBackFromSyncing,
	},
	{
		Name:             "Import and re-org to previously validated payload on a side chain",
		SlotsToSafe:      big.NewInt(15),
		SlotsToFinalized: big.NewInt(20),
		Run:              reorgPrevValidatedPayloadOnSideChain,
	},
	{
		Name:             "Safe Re-Org to Side Chain",
		Run:              safeReorgToSideChain,
		SlotsToSafe:      big.NewInt(1),
		SlotsToFinalized: big.NewInt(2),
	},

	// Suggested Fee Recipient in Payload creation
	{
		Name:                "Suggested Fee Recipient Test",
		Run:                 suggestedFeeRecipient,
		TestTransactionType: helper.LegacyTxOnly,
	},
	{
		Name:                "Suggested Fee Recipient Test (EIP-1559 Transactions)",
		Run:                 suggestedFeeRecipient,
		TestTransactionType: helper.DynamicFeeTxOnly,
	},

	// PrevRandao opcode tests
	{
		Name:                "PrevRandao Opcode Transactions",
		Run:                 prevRandaoOpcodeTx,
		TTD:                 10,
		TestTransactionType: helper.LegacyTxOnly,
	},
	{
		Name:                "PrevRandao Opcode Transactions (EIP-1559 Transactions)",
		Run:                 prevRandaoOpcodeTx,
		TTD:                 10,
		TestTransactionType: helper.DynamicFeeTxOnly,
	},
}

// Invalid Terminal Block in ForkchoiceUpdated: Client must reject ForkchoiceUpdated directives if the referenced HeadBlockHash does not meet the TTD requirement.
func invalidTerminalBlockForkchoiceUpdated(t *test.Env) {
	gblock := t.Genesis.ToBlock()

	forkchoiceState := api.ForkchoiceStateV1{
		HeadBlockHash:      gblock.Hash(),
		SafeBlockHash:      gblock.Hash(),
		FinalizedBlockHash: gblock.Hash(),
	}

	// Execution specification:
	// {payloadStatus: {status: INVALID, latestValidHash=0x00..00}, payloadId: null}
	// either obtained from the Payload validation process or as a result of validating a PoW block referenced by forkchoiceState.headBlockHash
	r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&forkchoiceState, nil)
	r.ExpectPayloadStatus(test.Invalid)
	r.ExpectLatestValidHash(&(common.Hash{}))
	// ValidationError is not validated since it can be either null or a string message

	// Check that PoW chain progresses
	t.VerifyPoWProgress(gblock.Hash())
}

// Invalid GetPayload Under PoW: Client must reject GetPayload directives under PoW.
func invalidGetPayloadUnderPoW(t *test.Env) {
	gblock := t.Genesis.ToBlock()
	// We start in PoW and try to get an invalid Payload, which should produce an error but nothing should be disrupted.
	r := t.TestEngine.TestEngineGetPayloadV1(&api.PayloadID{1, 2, 3, 4, 5, 6, 7, 8})
	r.ExpectError()

	// Check that PoW chain progresses
	t.VerifyPoWProgress(gblock.Hash())
}

// Invalid Terminal Block in NewPayload: Client must reject NewPayload directives if the referenced ParentHash does not meet the TTD requirement.
func invalidTerminalBlockNewPayload(t *test.Env) {
	gblock := t.Genesis.ToBlock()

	// Create a dummy payload to send in the NewPayload call
	payload := api.ExecutableData{
		ParentHash:    gblock.Hash(),
		FeeRecipient:  common.Address{},
		StateRoot:     gblock.Root(),
		ReceiptsRoot:  types.EmptyUncleHash,
		LogsBloom:     types.CreateBloom(types.Receipts{}).Bytes(),
		Random:        common.Hash{},
		Number:        1,
		GasLimit:      gblock.GasLimit(),
		GasUsed:       0,
		Timestamp:     gblock.Time() + 1,
		ExtraData:     []byte{},
		BaseFeePerGas: gblock.BaseFee(),
		BlockHash:     common.Hash{},
		Transactions:  [][]byte{},
	}
	hashedPayload, err := helper.CustomizePayload(&payload, &helper.CustomPayloadData{})
	if err != nil {
		t.Fatalf("FAIL (%s): Error while constructing PoW payload: %v", t.TestName, err)
	}

	// Execution specification:
	// {status: INVALID, latestValidHash=0x00..00}
	// if terminal block conditions are not satisfied
	r := t.TestEngine.TestEngineNewPayloadV1(hashedPayload)
	r.ExpectStatus(test.Invalid)
	r.ExpectLatestValidHash(&(common.Hash{}))
	// ValidationError is not validated since it can be either null or a string message

	// Check that PoW chain progresses
	t.VerifyPoWProgress(gblock.Hash())
}

// Verify that a forkchoiceUpdated with a valid HeadBlock (previously sent using NewPayload) and unknown SafeBlock
// results in error
func unknownSafeBlockHash(t *test.Env) {
	// Wait until TTD is reached by this client
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		// Run test after a new payload has been broadcast
		OnNewPayloadBroadcast: func() {

			// Generate a random SafeBlock hash
			randomSafeBlockHash := common.Hash{}
			rand.Read(randomSafeBlockHash[:])

			// Send forkchoiceUpdated with random SafeBlockHash
			// Execution specification:
			// - This value MUST be either equal to or an ancestor of headBlockHash
			r := t.TestEngine.TestEngineForkchoiceUpdatedV1(
				&api.ForkchoiceStateV1{
					HeadBlockHash:      t.CLMock.LatestExecutedPayload.BlockHash,
					SafeBlockHash:      randomSafeBlockHash,
					FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
				}, nil)
			r.ExpectError()

		},
	})

}

// Verify that a forkchoiceUpdated with a valid HeadBlock (previously sent using NewPayload) and unknown
// FinalizedBlockHash results in error
func unknownFinalizedBlockHash(t *test.Env) {
	// Wait until TTD is reached by this client
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		// Run test after a new payload has been broadcast
		OnNewPayloadBroadcast: func() {

			// Generate a random FinalizedBlockHash hash
			randomFinalizedBlockHash := common.Hash{}
			rand.Read(randomFinalizedBlockHash[:])

			// Send forkchoiceUpdated with random FinalizedBlockHash
			forkchoiceStateUnknownFinalizedHash := api.ForkchoiceStateV1{
				HeadBlockHash:      t.CLMock.LatestExecutedPayload.BlockHash,
				SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
				FinalizedBlockHash: randomFinalizedBlockHash,
			}
			r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&forkchoiceStateUnknownFinalizedHash, nil)
			r.ExpectError()

			// Test again using PayloadAttributes, should also return INVALID and no PayloadID
			r = t.TestEngine.TestEngineForkchoiceUpdatedV1(&forkchoiceStateUnknownFinalizedHash,
				&api.PayloadAttributes{
					Timestamp:             t.CLMock.LatestExecutedPayload.Timestamp + 1,
					Random:                common.Hash{},
					SuggestedFeeRecipient: common.Address{},
				})
			r.ExpectError()

		},
	})

}

// Verify that an unknown hash at HeadBlock in the forkchoice results in client returning Syncing state
func unknownHeadBlockHash(t *test.Env) {
	// Wait until TTD is reached by this client
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	// Generate a random HeadBlock hash
	randomHeadBlockHash := common.Hash{}
	rand.Read(randomHeadBlockHash[:])

	forkchoiceStateUnknownHeadHash := api.ForkchoiceStateV1{
		HeadBlockHash:      randomHeadBlockHash,
		SafeBlockHash:      t.CLMock.LatestForkchoice.FinalizedBlockHash,
		FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
	}

	t.Logf("INFO (%v) forkchoiceStateUnknownHeadHash: %v\n", t.TestName, forkchoiceStateUnknownHeadHash)

	// Execution specification::
	// - {payloadStatus: {status: SYNCING, latestValidHash: null, validationError: null}, payloadId: null}
	//   if forkchoiceState.headBlockHash references an unknown payload or a payload that can't be validated
	//   because requisite data for the validation is missing
	r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&forkchoiceStateUnknownHeadHash, nil)
	r.ExpectPayloadStatus(test.Syncing)

	// Test again using PayloadAttributes, should also return SYNCING and no PayloadID
	r = t.TestEngine.TestEngineForkchoiceUpdatedV1(&forkchoiceStateUnknownHeadHash,
		&api.PayloadAttributes{
			Timestamp:             t.CLMock.LatestExecutedPayload.Timestamp + 1,
			Random:                common.Hash{},
			SuggestedFeeRecipient: common.Address{},
		})
	r.ExpectPayloadStatus(test.Syncing)
	r.ExpectPayloadID(nil)

}

// Send an inconsistent ForkchoiceState with a known payload that belongs to a side chain as head, safe or finalized.
func inconsistentForkchoiceStateGen(inconsistency string) func(t *test.Env) {
	return func(t *test.Env) {
		// Wait until TTD is reached by this client
		t.CLMock.WaitForTTD()

		canonicalPayloads := make([]*api.ExecutableData, 0)
		alternativePayloads := make([]*api.ExecutableData, 0)
		// Produce blocks before starting the test
		t.CLMock.ProduceBlocks(3, clmock.BlockProcessCallbacks{
			OnGetPayload: func() {
				// Generate and send an alternative side chain
				customData := helper.CustomPayloadData{}
				customData.ExtraData = &([]byte{0x01})
				if len(alternativePayloads) > 0 {
					customData.ParentHash = &alternativePayloads[len(alternativePayloads)-1].BlockHash
				}
				alternativePayload, err := helper.CustomizePayload(&t.CLMock.LatestPayloadBuilt, &customData)
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to construct alternative payload: %v", t.TestName, err)
				}
				alternativePayloads = append(alternativePayloads, alternativePayload)
				latestCanonicalPayload := t.CLMock.LatestPayloadBuilt
				canonicalPayloads = append(canonicalPayloads, &latestCanonicalPayload)

				// Send the alternative payload
				r := t.TestEngine.TestEngineNewPayloadV1(alternativePayload)
				r.ExpectStatusEither(test.Valid, test.Accepted)
			},
		})
		// Send the invalid ForkchoiceStates
		inconsistentFcU := api.ForkchoiceStateV1{
			HeadBlockHash:      canonicalPayloads[len(alternativePayloads)-1].BlockHash,
			SafeBlockHash:      canonicalPayloads[len(alternativePayloads)-2].BlockHash,
			FinalizedBlockHash: canonicalPayloads[len(alternativePayloads)-3].BlockHash,
		}
		switch inconsistency {
		case "Head":
			inconsistentFcU.HeadBlockHash = alternativePayloads[len(alternativePayloads)-1].BlockHash
		case "Safe":
			inconsistentFcU.SafeBlockHash = alternativePayloads[len(canonicalPayloads)-2].BlockHash
		case "Finalized":
			inconsistentFcU.FinalizedBlockHash = alternativePayloads[len(canonicalPayloads)-3].BlockHash
		}
		r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&inconsistentFcU, nil)
		r.ExpectError()

		// Return to the canonical chain
		r = t.TestEngine.TestEngineForkchoiceUpdatedV1(&t.CLMock.LatestForkchoice, nil)
		r.ExpectPayloadStatus(test.Valid)
	}
}

// Verify behavior on a forkchoiceUpdated with invalid payload attributes
func invalidPayloadAttributesGen(syncing bool) func(*test.Env) {

	return func(t *test.Env) {
		// Wait until TTD is reached by this client
		t.CLMock.WaitForTTD()

		// Produce blocks before starting the test
		t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

		// Send a forkchoiceUpdated with invalid PayloadAttributes
		t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
			OnNewPayloadBroadcast: func() {
				// Try to apply the new payload with invalid attributes
				var blockHash common.Hash
				if syncing {
					// Setting a random hash will put the client into `SYNCING`
					rand.Read(blockHash[:])
				} else {
					// Set the block hash to the next payload that was broadcasted
					blockHash = t.CLMock.LatestPayloadBuilt.BlockHash
				}
				t.Logf("INFO (%s): Sending EngineForkchoiceUpdatedV1 (Syncing=%s) with invalid payload attributes", t.TestName, syncing)
				fcu := api.ForkchoiceStateV1{
					HeadBlockHash:      blockHash,
					SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
					FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
				}
				attr := api.PayloadAttributes{
					Timestamp:             0,
					Random:                common.Hash{},
					SuggestedFeeRecipient: common.Address{},
				}
				// 0) Check headBlock is known and there is no missing data, if not respond with SYNCING
				// 1) Check headBlock is VALID, if not respond with INVALID
				// 2) Apply forkchoiceState
				// 3) Check payloadAttributes, if invalid respond with error: code: Invalid payload attributes
				// 4) Start payload build process and respond with VALID
				if syncing {
					// If we are SYNCING, the outcome should be SYNCING regardless of the validity of the payload atttributes
					r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&fcu, &attr)
					r.ExpectPayloadStatus(test.Syncing)
					r.ExpectPayloadID(nil)
				} else {
					r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&fcu, &attr)
					r.ExpectError()

					// Check that the forkchoice was applied, regardless of the error
					s := t.TestEngine.TestHeaderByNumber(Head)
					s.ExpectHash(blockHash)
				}
			},
		})
	}

}

// Verify that a forkchoiceUpdated fails on hash being set to a pre-TTD block after PoS change
func preTTDFinalizedBlockHash(t *test.Env) {
	// Wait until TTD is reached by this client
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	// Send the Genesis block as forkchoice
	gblock := t.Genesis.ToBlock()

	r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&api.ForkchoiceStateV1{
		HeadBlockHash:      gblock.Hash(),
		SafeBlockHash:      gblock.Hash(),
		FinalizedBlockHash: gblock.Hash(),
	}, nil)
	r.ExpectPayloadStatus(test.Invalid)
	r.ExpectLatestValidHash(&(common.Hash{}))

	r = t.TestEngine.TestEngineForkchoiceUpdatedV1(&t.CLMock.LatestForkchoice, nil)
	r.ExpectPayloadStatus(test.Valid)

}

// Corrupt the hash of a valid payload, client should reject the payload.
// All possible scenarios:
//    (fcU)
//	┌────────┐        ┌────────────────────────┐
//	│  HEAD  │◄───────┤ Bad Hash (!Sync,!Side) │
//	└────┬───┘        └────────────────────────┘
//		 │
//		 │
//	┌────▼───┐        ┌────────────────────────┐
//	│ HEAD-1 │◄───────┤ Bad Hash (!Sync, Side) │
//	└────┬───┘        └────────────────────────┘
//		 │
//
//
//	  (fcU)
//	********************  ┌───────────────────────┐
//	*  (Unknown) HEAD  *◄─┤ Bad Hash (Sync,!Side) │
//	********************  └───────────────────────┘
//		 │
//		 │
//	┌────▼───┐            ┌───────────────────────┐
//	│ HEAD-1 │◄───────────┤ Bad Hash (Sync, Side) │
//	└────┬───┘            └───────────────────────┘
//		 │
//

func badHashOnNewPayloadGen(syncing bool, sidechain bool) func(*test.Env) {

	return func(t *test.Env) {
		// Wait until TTD is reached by this client
		t.CLMock.WaitForTTD()

		// Produce blocks before starting the test
		t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

		var (
			alteredPayload     api.ExecutableData
			invalidPayloadHash common.Hash
		)

		t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
			// Run test after the new payload has been obtained
			OnGetPayload: func() {
				// Alter hash on the payload and send it to client, should produce an error
				alteredPayload = t.CLMock.LatestPayloadBuilt
				invalidPayloadHash = alteredPayload.BlockHash
				invalidPayloadHash[common.HashLength-1] = byte(255 - invalidPayloadHash[common.HashLength-1])
				alteredPayload.BlockHash = invalidPayloadHash

				if !syncing && sidechain {
					// We alter the payload by setting the parent to a known past block in the
					// canonical chain, which makes this payload a side chain payload, and also an invalid block hash
					// (because we did not update the block hash appropriately)
					alteredPayload.ParentHash = t.CLMock.LatestHeader.ParentHash
				} else if syncing {
					// We need to send an fcU to put the client in SYNCING state.
					randomHeadBlock := common.Hash{}
					rand.Read(randomHeadBlock[:])
					fcU := api.ForkchoiceStateV1{
						HeadBlockHash:      randomHeadBlock,
						SafeBlockHash:      t.CLMock.LatestHeader.Hash(),
						FinalizedBlockHash: t.CLMock.LatestHeader.Hash(),
					}
					r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&fcU, nil)
					r.ExpectPayloadStatus(test.Syncing)

					if sidechain {
						// Syncing and sidechain, the caonincal head is an unknown payload to us,
						// but this specific bad hash payload is in theory part of a side chain.
						// Therefore the parent we use is the head hash.
						alteredPayload.ParentHash = t.CLMock.LatestHeader.Hash()
					} else {
						// The invalid bad-hash payload points to the unknown head, but we know it is
						// indeed canonical because the head was set using forkchoiceUpdated.
						alteredPayload.ParentHash = randomHeadBlock
					}
				}

				// Execution specification::
				// - {status: INVALID_BLOCK_HASH, latestValidHash: null, validationError: null} if the blockHash validation has failed
				// Starting from Shanghai, INVALID should be returned instead (https://github.com/ethereum/execution-apis/pull/338)
				r := t.TestEngine.TestEngineNewPayloadV1(&alteredPayload)
				r.ExpectStatusEither(test.InvalidBlockHash, test.Invalid)
				r.ExpectLatestValidHash(nil)
			},
		})

		// Lastly, attempt to build on top of the invalid payload
		t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
			// Run test after the new payload has been obtained
			OnGetPayload: func() {
				alteredPayload, err := helper.CustomizePayload(&t.CLMock.LatestPayloadBuilt, &helper.CustomPayloadData{
					ParentHash: &invalidPayloadHash,
				})
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to modify payload: %v", t.TestName, err)
				}

				// Response status can be ACCEPTED (since parent payload could have been thrown out by the client)
				// or INVALID (client still has the payload and can verify that this payload is incorrectly building on top of it),
				// but a VALID response is incorrect.
				r := t.TestEngine.TestEngineNewPayloadV1(alteredPayload)
				r.ExpectStatusEither(test.Accepted, test.Invalid, test.Syncing)

			},
		})

	}
}

// Copy the parentHash into the blockHash, client should reject the payload
// (from Kintsugi Incident Report: https://notes.ethereum.org/@ExXcnR0-SJGthjz1dwkA1A/BkkdHWXTY)
func parentHashOnExecPayload(t *test.Env) {
	// Wait until TTD is reached by this client
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		// Run test after the new payload has been obtained
		OnGetPayload: func() {
			// Alter hash on the payload and send it to client, should produce an error
			alteredPayload := t.CLMock.LatestPayloadBuilt
			alteredPayload.BlockHash = alteredPayload.ParentHash
			// Execution specification::
			// - {status: INVALID_BLOCK_HASH, latestValidHash: null, validationError: null} if the blockHash validation has failed
			// Starting from Shanghai, INVALID should be returned instead (https://github.com/ethereum/execution-apis/pull/338)
			r := t.TestEngine.TestEngineNewPayloadV1(&alteredPayload)
			r.ExpectStatusEither(test.InvalidBlockHash, test.Invalid)
			r.ExpectLatestValidHash(nil)
		},
	})

}

// Attempt to re-org to a chain containing an invalid transition payload
func invalidTransitionPayload(t *test.Env) {
	// Wait until TTD is reached by main client
	t.CLMock.WaitForTTD()

	// Produce two blocks before trying to re-org
	t.CLMock.ProduceBlocks(2, clmock.BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {
			_, err := helper.SendNextTransaction(
				t.TestContext,
				t.CLMock.NextBlockProducer,
				&helper.BaseTransactionCreator{
					Recipient: &globals.PrevRandaoContractAddr,
					Amount:    big1,
					Payload:   nil,
					TxType:    t.TestTransactionType,
					GasLimit:  75000,
				},
			)
			if err != nil {
				t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
			}
		},
	})

	// Introduce the invalid transition payload
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		// This is being done in the middle of the block building
		// process simply to be able to re-org back.
		OnGetPayload: func() {
			basePayload := t.CLMock.ExecutedPayloadHistory[t.CLMock.FirstPoSBlockNumber.Uint64()]
			alteredPayload, err := helper.GenerateInvalidPayload(basePayload, helper.InvalidStateRoot)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to modify payload: %v", t.TestName, err)
			}
			p := t.TestEngine.TestEngineNewPayloadV1(alteredPayload)
			p.ExpectStatusEither(test.Invalid, test.Accepted)
			if p.Status.Status == test.Invalid {
				p.ExpectLatestValidHash(&(common.Hash{}))
			} else if p.Status.Status == test.Accepted {
				p.ExpectLatestValidHash(nil)
			}
			r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&api.ForkchoiceStateV1{
				HeadBlockHash:      alteredPayload.BlockHash,
				SafeBlockHash:      common.Hash{},
				FinalizedBlockHash: common.Hash{},
			}, nil)
			r.ExpectPayloadStatus(test.Invalid)
			r.ExpectLatestValidHash(&(common.Hash{}))

			s := t.TestEngine.TestBlockByNumber(Head)
			s.ExpectHash(t.CLMock.LatestExecutedPayload.BlockHash)
		},
	})
}

// Generate test cases for each field of NewPayload, where the payload contains a single invalid field and a valid hash.
func invalidPayloadTestCaseGen(payloadField helper.InvalidPayloadBlockField, syncing bool, emptyTxs bool) func(*test.Env) {
	return func(t *test.Env) {

		if syncing {
			// To allow sending the primary engine client into SYNCING state, we need a secondary client to guide the payload creation
			secondaryClient, err := hive_rpc.HiveRPCEngineStarter{}.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
			}
			t.CLMock.AddEngineClient(secondaryClient)
		}

		// Wait until TTD is reached by all clients
		t.CLMock.WaitForTTD()

		txFunc := func() {
			if !emptyTxs {
				// Function to send at least one transaction each block produced
				// Send the transaction to the globals.PrevRandaoContractAddr
				_, err := helper.SendNextTransaction(
					t.TestContext,
					t.CLMock.NextBlockProducer,
					&helper.BaseTransactionCreator{
						Recipient: &globals.PrevRandaoContractAddr,
						Amount:    big1,
						Payload:   nil,
						TxType:    t.TestTransactionType,
						GasLimit:  75000,
					},
				)
				if err != nil {
					t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
				}
			}
		}

		// Produce blocks before starting the test
		t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{
			// Make sure at least one transaction is included in each block
			OnPayloadProducerSelected: txFunc,
		})

		if syncing {
			// Disconnect the main engine client from the CL Mocker and produce a block
			t.CLMock.RemoveEngineClient(t.Engine)
			t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
				OnPayloadProducerSelected: txFunc,
			})

			// This block is now unknown to the main client, sending an fcU will set it to syncing mode
			r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&t.CLMock.LatestForkchoice, nil)
			r.ExpectPayloadStatus(test.Syncing)
		}

		var (
			alteredPayload *api.ExecutableData
			err            error
		)

		t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
			// Make sure at least one transaction is included in the payload
			OnPayloadProducerSelected: txFunc,
			// Run test after the new payload has been obtained
			OnGetPayload: func() {
				// Alter the payload while maintaining a valid hash and send it to the client, should produce an error

				// We need at least one transaction for most test cases to work
				if !emptyTxs && len(t.CLMock.LatestPayloadBuilt.Transactions) == 0 {
					// But if the payload has no transactions, the test is invalid
					t.Fatalf("FAIL (%s): No transactions in the base payload", t.TestName)
				}

				alteredPayload, err = helper.GenerateInvalidPayload(&t.CLMock.LatestPayloadBuilt, payloadField)
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to modify payload (%v): %v", t.TestName, payloadField, err)
				}

				// Depending on the field we modified, we expect a different status
				r := t.TestEngine.TestEngineNewPayloadV1(alteredPayload)
				if syncing || payloadField == helper.InvalidParentHash {
					// Execution specification::
					// {status: ACCEPTED, latestValidHash: null, validationError: null} if the following conditions are met:
					//  - the blockHash of the payload is valid
					//  - the payload doesn't extend the canonical chain
					//  - the payload hasn't been fully validated
					// {status: SYNCING, latestValidHash: null, validationError: null}
					// if the payload extends the canonical chain and requisite data for its validation is missing
					// (the client can assume the payload extends the canonical because the linking payload could be missing)
					r.ExpectStatusEither(test.Accepted, test.Syncing)
					r.ExpectLatestValidHash(nil)
				} else {
					r.ExpectStatus(test.Invalid)
					r.ExpectLatestValidHash(&alteredPayload.ParentHash)
				}

				// Send the forkchoiceUpdated with a reference to the invalid payload.
				fcState := api.ForkchoiceStateV1{
					HeadBlockHash:      alteredPayload.BlockHash,
					SafeBlockHash:      alteredPayload.BlockHash,
					FinalizedBlockHash: alteredPayload.BlockHash,
				}
				payloadAttrbutes := api.PayloadAttributes{
					Timestamp:             alteredPayload.Timestamp + 1,
					Random:                common.Hash{},
					SuggestedFeeRecipient: common.Address{},
				}

				// Execution specification:
				//  {payloadStatus: {status: INVALID, latestValidHash: null, validationError: errorMessage | null}, payloadId: null}
				//  obtained from the Payload validation process if the payload is deemed INVALID
				s := t.TestEngine.TestEngineForkchoiceUpdatedV1(&fcState, &payloadAttrbutes)
				if !syncing {
					// Execution specification:
					//  {payloadStatus: {status: INVALID, latestValidHash: null, validationError: errorMessage | null}, payloadId: null}
					//  obtained from the Payload validation process if the payload is deemed INVALID
					// Note: SYNCING/ACCEPTED is acceptable here as long as the block produced after this test is produced successfully
					s.ExpectAnyPayloadStatus(test.Syncing, test.Accepted, test.Invalid)
				} else {
					// At this moment the response should be SYNCING
					s.ExpectPayloadStatus(test.Syncing)

					// When we send the previous payload, the client must now be capable of determining that the invalid payload is actually invalid
					p := t.TestEngine.TestEngineNewPayloadV1(&t.CLMock.LatestExecutedPayload)
					p.ExpectStatus(test.Valid)
					p.ExpectLatestValidHash(&t.CLMock.LatestExecutedPayload.BlockHash)

					/*
						// Another option here could be to send an fcU to the previous payload,
						// but this does not seem like something the CL would do.
						s := t.TestEngine.TestEngineForkchoiceUpdatedV1(&api.ForkchoiceStateV1{
							HeadBlockHash:      previousPayload.BlockHash,
							SafeBlockHash:      previousPayload.BlockHash,
							FinalizedBlockHash: previousPayload.BlockHash,
						}, nil)
						s.ExpectPayloadStatus(Valid)
					*/

					q := t.TestEngine.TestEngineNewPayloadV1(alteredPayload)
					if payloadField == helper.InvalidParentHash {
						// There is no invalid parentHash, if this value is incorrect,
						// it is assumed that the block is missing and we need to sync.
						// ACCEPTED also valid since the CLs normally use these interchangeably
						q.ExpectStatusEither(test.Syncing, test.Accepted)
						q.ExpectLatestValidHash(nil)
					} else if payloadField == helper.InvalidNumber {
						// A payload with an invalid number can force us to start a sync cycle
						// as we don't know if that block might be a valid future block.
						q.ExpectStatusEither(test.Invalid, test.Syncing)
						if q.Status.Status == test.Invalid {
							q.ExpectLatestValidHash(&t.CLMock.LatestExecutedPayload.BlockHash)
						} else {
							q.ExpectLatestValidHash(nil)
						}
					} else {
						// Otherwise the response should be INVALID.
						q.ExpectStatus(test.Invalid)
						q.ExpectLatestValidHash(&t.CLMock.LatestExecutedPayload.BlockHash)
					}

					// Try sending the fcU again, this time we should get the proper invalid response.
					// At this moment the response should be INVALID
					if payloadField != helper.InvalidParentHash {
						s := t.TestEngine.TestEngineForkchoiceUpdatedV1(&fcState, nil)
						// Note: SYNCING is acceptable here as long as the block produced after this test is produced successfully
						s.ExpectAnyPayloadStatus(test.Syncing, test.Invalid)
					}
				}

				// Finally, attempt to fetch the invalid payload using the JSON-RPC endpoint
				p := t.TestEngine.TestBlockByHash(alteredPayload.BlockHash)
				p.ExpectError()
			},
		})

		if syncing {
			// Send the valid payload and its corresponding forkchoiceUpdated
			r := t.TestEngine.TestEngineNewPayloadV1(&t.CLMock.LatestExecutedPayload)
			r.ExpectStatus(test.Valid)
			r.ExpectLatestValidHash(&t.CLMock.LatestExecutedPayload.BlockHash)

			s := t.TestEngine.TestEngineForkchoiceUpdatedV1(&t.CLMock.LatestForkchoice, nil)
			s.ExpectPayloadStatus(test.Valid)
			s.ExpectLatestValidHash(&t.CLMock.LatestExecutedPayload.BlockHash)

			// Add main client again to the CL Mocker
			t.CLMock.AddEngineClient(t.Engine)
		}

		// Lastly, attempt to build on top of the invalid payload
		t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
			// Run test after the new payload has been obtained
			OnGetPayload: func() {
				followUpAlteredPayload, err := helper.CustomizePayload(&t.CLMock.LatestPayloadBuilt, &helper.CustomPayloadData{
					ParentHash: &alteredPayload.BlockHash,
				})
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to modify payload: %v", t.TestName, err)
				}

				t.Logf("INFO (%s): Sending customized NewPayload: ParentHash %v -> %v", t.TestName, t.CLMock.LatestPayloadBuilt.ParentHash, alteredPayload.BlockHash)
				// Response status can be ACCEPTED (since parent payload could have been thrown out by the client)
				// or SYNCING (parent payload is thrown out and also client assumes that the parent is part of canonical chain)
				// or INVALID (client still has the payload and can verify that this payload is incorrectly building on top of it),
				// but a VALID response is incorrect.
				r := t.TestEngine.TestEngineNewPayloadV1(followUpAlteredPayload)
				r.ExpectStatusEither(test.Accepted, test.Invalid, test.Syncing)
				if r.Status.Status == test.Accepted || r.Status.Status == test.Syncing {
					r.ExpectLatestValidHash(nil)
				} else if r.Status.Status == test.Invalid {
					r.ExpectLatestValidHash(&alteredPayload.ParentHash)
				}
			},
		})
	}
}

// Attempt to re-org to a chain which at some point contains an unknown payload which is also invalid.
// Then reveal the invalid payload and expect that the client rejects it and rejects forkchoice updated calls to this chain.
// The invalid_index parameter determines how many payloads apart is the common ancestor from the block that invalidates the chain,
// with a value of 1 meaning that the immediate payload after the common ancestor will be invalid.
func invalidMissingAncestorReOrgGen(invalid_index int, payloadField helper.InvalidPayloadBlockField, emptyTxs bool) func(*test.Env) {
	return func(t *test.Env) {

		// Wait until TTD is reached by this client
		t.CLMock.WaitForTTD()

		// Produce blocks before starting the test
		t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

		// Save the common ancestor
		cA := t.CLMock.LatestPayloadBuilt

		// Amount of blocks to deviate starting from the common ancestor
		n := 10

		// Slice to save the side B chain
		altChainPayloads := make([]*api.ExecutableData, 0)

		// Append the common ancestor
		altChainPayloads = append(altChainPayloads, &cA)

		// Produce blocks but at the same time create an side chain which contains an invalid payload at some point (INV_P)
		// CommonAncestor◄─▲── P1 ◄─ P2 ◄─ P3 ◄─ ... ◄─ Pn
		//                 │
		//                 └── P1' ◄─ P2' ◄─ ... ◄─ INV_P ◄─ ... ◄─ Pn'
		t.CLMock.ProduceBlocks(n, clmock.BlockProcessCallbacks{

			OnPayloadProducerSelected: func() {
				// Function to send at least one transaction each block produced.
				// Empty Txs Payload with invalid stateRoot discovered an issue in geth sync, hence this is customizable.
				if !emptyTxs {
					// Send the transaction to the globals.PrevRandaoContractAddr
					_, err := helper.SendNextTransaction(
						t.TestContext,
						t.CLMock.NextBlockProducer,
						&helper.BaseTransactionCreator{
							Recipient: &globals.PrevRandaoContractAddr,
							Amount:    big1,
							Payload:   nil,
							TxType:    t.TestTransactionType,
							GasLimit:  75000,
						},
					)
					if err != nil {
						t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
					}
				}
			},
			OnGetPayload: func() {
				var (
					sidePayload *api.ExecutableData
					err         error
				)
				// Insert extraData to ensure we deviate from the main payload, which contains empty extradata
				sidePayload, err = helper.CustomizePayload(&t.CLMock.LatestPayloadBuilt, &helper.CustomPayloadData{
					ParentHash: &altChainPayloads[len(altChainPayloads)-1].BlockHash,
					ExtraData:  &([]byte{0x01}),
				})
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to customize payload: %v", t.TestName, err)
				}
				if len(altChainPayloads) == invalid_index {
					sidePayload, err = helper.GenerateInvalidPayload(sidePayload, payloadField)
					if err != nil {
						t.Fatalf("FAIL (%s): Unable to customize payload: %v", t.TestName, err)
					}
				}
				altChainPayloads = append(altChainPayloads, sidePayload)
			},
		})
		t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
			// Note: We perform the test in the middle of payload creation by the CL Mock, in order to be able to
			// re-org back into this chain and use the new payload without issues.
			OnGetPayload: func() {

				// Now let's send the side chain to the client using newPayload/sync
				for i := 1; i <= n; i++ {
					// Send the payload
					payloadValidStr := "VALID"
					if i == invalid_index {
						payloadValidStr = "INVALID"
					} else if i > invalid_index {
						payloadValidStr = "VALID with INVALID ancestor"
					}
					t.Logf("INFO (%s): Invalid chain payload %d (%s): %v", t.TestName, i, payloadValidStr, altChainPayloads[i].BlockHash)

					r := t.TestEngine.TestEngineNewPayloadV1(altChainPayloads[i])
					p := t.TestEngine.TestEngineForkchoiceUpdatedV1(&api.ForkchoiceStateV1{
						HeadBlockHash:      altChainPayloads[i].BlockHash,
						SafeBlockHash:      altChainPayloads[i].BlockHash,
						FinalizedBlockHash: common.Hash{},
					}, nil)
					if i == invalid_index {
						// If this is the first payload after the common ancestor, and this is the payload we invalidated,
						// then we have all the information to determine that this payload is invalid.
						r.ExpectStatus(test.Invalid)
						r.ExpectLatestValidHash(&altChainPayloads[i-1].BlockHash)
					} else if i > invalid_index {
						// We have already sent the invalid payload, but the client could've discarded it.
						// In reality the CL will not get to this point because it will have already received the `INVALID`
						// response from the previous payload.
						// The node might save the parent as invalid, thus returning INVALID
						r.ExpectStatusEither(test.Accepted, test.Syncing, test.Invalid)
						if r.Status.Status == test.Accepted || r.Status.Status == test.Syncing {
							r.ExpectLatestValidHash(nil)
						} else if r.Status.Status == test.Invalid {
							r.ExpectLatestValidHash(&altChainPayloads[invalid_index-1].BlockHash)
						}
					} else {
						// This is one of the payloads before the invalid one, therefore is valid.
						r.ExpectStatus(test.Valid)
						p.ExpectPayloadStatus(test.Valid)
						p.ExpectLatestValidHash(&altChainPayloads[i].BlockHash)
					}

				}

				// Resend the latest correct fcU
				r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&t.CLMock.LatestForkchoice, nil)
				r.ExpectNoError()
				// After this point, the CL Mock will send the next payload of the canonical chain
			},
		})

	}
}

// Attempt to re-org to a chain which at some point contains an unknown payload which is also invalid.
// Then reveal the invalid payload and expect that the client rejects it and rejects forkchoice updated calls to this chain.
type InvalidMissingAncestorReOrgSpec struct {
	// Index of the payload to invalidate, starting with 0 being the common ancestor.
	// Value must be greater than 0.
	PayloadInvalidIndex int
	// Field of the payload to invalidate (see helper module)
	PayloadField helper.InvalidPayloadBlockField
	// Whether to create payloads with empty transactions or not:
	// Used to test scenarios where the stateRoot is invalidated but its invalidation
	// goes unnoticed by the client because of the lack of transactions.
	EmptyTransactions bool
	// Height of the common ancestor in the proof-of-stake chain.
	// Value of 0 means the common ancestor is the terminal proof-of-work block.
	CommonAncestorHeight *big.Int
	// Amount of payloads to produce between the common ancestor and the head of the
	// proof-of-stake chain.
	DeviatingPayloadCount *big.Int
	// Whether the syncing client must re-org from a canonical chain.
	// If set to true, the client is driven through a valid canonical chain first,
	// and then the client is prompted to re-org to the invalid chain.
	// If set to false, the client is prompted to sync from the genesis
	// or start chain (if specified).
	ReOrgFromCanonical bool
}

func (spec InvalidMissingAncestorReOrgSpec) GenerateSync() func(*test.Env) {
	var (
		invalidIndex = spec.PayloadInvalidIndex
	)

	return func(t *test.Env) {
		var (
			err             error
			secondaryClient *node.GethNode
		)
		// To allow having the invalid payload delivered via P2P, we need a second client to serve the payload
		starter := node.GethNodeEngineStarter{
			Config: node.GethNodeTestConfiguration{
				// Ethash engine is only needed in a single test case
				Ethash: spec.PayloadField == helper.InvalidOmmers &&
					spec.CommonAncestorHeight != nil &&
					spec.CommonAncestorHeight.Cmp(big0) == 0 &&
					spec.PayloadInvalidIndex == 1,
			},
		}
		if spec.ReOrgFromCanonical {
			// If we are doing a re-org from canonical, we can add both nodes as peers from the start
			secondaryClient, err = starter.StartGethNode(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles, t.Engine)
		} else {
			secondaryClient, err = starter.StartGethNode(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles)
		}
		if err != nil {
			t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
		}

		t.CLMock.AddEngineClient(secondaryClient)

		if !spec.ReOrgFromCanonical {
			// Remove the original client so that it does not receive the payloads created on the canonical chain
			t.CLMock.RemoveEngineClient(t.Engine)
		}

		// Wait until TTD is reached by this client
		t.CLMock.WaitForTTD()

		// Produce blocks before starting the test
		var cA *types.Block

		cAHeight := spec.CommonAncestorHeight
		if cAHeight == nil {
			// Default is to produce 5 PoS blocks before the common ancestor
			cAHeight = big.NewInt(5)
		}

		// Save the common ancestor
		if cAHeight.Cmp(big0) == 0 {
			// Common ancestor is the proof-of-work terminal block
			ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
			defer cancel()
			b, err := secondaryClient.BlockByNumber(ctx, nil)
			if err != nil {
				t.Fatalf("FAIL (%s): Error while getting latest block: %v", t.TestName, err)
			}
			cA = b
		} else {
			t.CLMock.ProduceBlocks(int(cAHeight.Int64()), clmock.BlockProcessCallbacks{})
			cA, err = api.ExecutableDataToBlock(t.CLMock.LatestPayloadBuilt)
			if err != nil {
				t.Fatalf("FAIL (%s): Error converting payload to block: %v", t.TestName, err)
			}
		}

		// Amount of blocks to deviate starting from the common ancestor
		// Default is to deviate 10 payloads from the common ancestor
		n := 10
		if spec.DeviatingPayloadCount != nil {
			n = int(spec.DeviatingPayloadCount.Int64())
		}

		// Slice to save the side B chain
		altChainPayloads := make([]*types.Block, 0)

		// Append the common ancestor
		altChainPayloads = append(altChainPayloads, cA)

		// Produce blocks but at the same time create an side chain which contains an invalid payload at some point (INV_P)
		// CommonAncestor◄─▲── P1 ◄─ P2 ◄─ P3 ◄─ ... ◄─ Pn
		//                 │
		//                 └── P1' ◄─ P2' ◄─ ... ◄─ INV_P ◄─ ... ◄─ Pn'
		t.CLMock.ProduceBlocks(n, clmock.BlockProcessCallbacks{

			OnPayloadProducerSelected: func() {
				// Function to send at least one transaction each block produced.
				// Empty Txs Payload with invalid stateRoot discovered an issue in geth sync, hence this is customizable.
				if !spec.EmptyTransactions {
					// Send the transaction to the globals.PrevRandaoContractAddr
					_, err := helper.SendNextTransaction(
						t.TestContext,
						t.CLMock.NextBlockProducer,
						&helper.BaseTransactionCreator{
							Recipient: &globals.PrevRandaoContractAddr,
							Amount:    big1,
							Payload:   nil,
							TxType:    t.TestTransactionType,
							GasLimit:  75000,
						},
					)
					if err != nil {
						t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
					}
				}
			},
			OnGetPayload: func() {
				var (
					sidePayload *api.ExecutableData
					err         error
				)
				// Insert extraData to ensure we deviate from the main payload, which contains empty extradata
				pHash := altChainPayloads[len(altChainPayloads)-1].Hash()
				sidePayload, err = helper.CustomizePayload(&t.CLMock.LatestPayloadBuilt, &helper.CustomPayloadData{
					ParentHash: &pHash,
					ExtraData:  &([]byte{0x01}),
				})
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to customize payload: %v", t.TestName, err)
				}
				if len(altChainPayloads) == invalidIndex {
					sidePayload, err = helper.GenerateInvalidPayload(sidePayload, spec.PayloadField)
					if err != nil {
						t.Fatalf("FAIL (%s): Unable to customize payload: %v", t.TestName, err)
					}
				}

				sideBlock, err := api.ExecutableDataToBlock(*sidePayload)
				if err != nil {
					t.Fatalf("FAIL (%s): Error converting payload to block: %v", t.TestName, err)
				}
				if len(altChainPayloads) == invalidIndex {
					var uncle *types.Block
					if spec.PayloadField == helper.InvalidOmmers {
						if unclePayload, ok := t.CLMock.ExecutedPayloadHistory[sideBlock.NumberU64()-1]; ok && unclePayload != nil {
							// Uncle is a PoS payload
							uncle, err = api.ExecutableDataToBlock(*unclePayload)
							if err != nil {
								t.Fatalf("FAIL (%s): Unable to get uncle block: %v", t.TestName, err)
							}
						} else {
							// Uncle is a PoW block, we need to mine an alternative PoW block
							ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
							defer cancel()
							parentPoW, err := secondaryClient.BlockByNumber(ctx, new(big.Int).Sub(sideBlock.Number(), big1))
							if err != nil || parentPoW == nil {
								t.Fatalf("FAIL (%s): Unable to get parent base block to generate uncle: %v", t.TestName, err)
							}
							uncle, err = secondaryClient.SealBlock(t.TimeoutContext, parentPoW)
							if err != nil {
								t.Fatalf("FAIL (%s): Unable generate uncle: %v", t.TestName, err)
							}
						}
					}
					// Invalidate fields not available in the ExecutableData
					sideBlock, err = helper.GenerateInvalidPayloadBlock(sideBlock, uncle, spec.PayloadField)
					if err != nil {
						t.Fatalf("FAIL (%s): Unable to customize payload block: %v", t.TestName, err)
					}
				}

				altChainPayloads = append(altChainPayloads, sideBlock)
			},
		})
		t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
			// Note: We perform the test in the middle of payload creation by the CL Mock, in order to be able to
			// re-org back into this chain and use the new payload without issues.
			OnGetPayload: func() {

				// Now let's send the side chain to the client using newPayload/sync
				for i := 1; i < n; i++ {
					// Send the payload
					payloadValidStr := "VALID"
					if i == invalidIndex {
						payloadValidStr = "INVALID"
					} else if i > invalidIndex {
						payloadValidStr = "VALID with INVALID ancestor"
					}
					payloadJs, _ := json.MarshalIndent(altChainPayloads[i].Header(), "", " ")
					t.Logf("INFO (%s): Invalid chain payload %d (%s):\n%s", t.TestName, i, payloadValidStr, payloadJs)

					if i < invalidIndex {
						ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
						defer cancel()

						p := api.BlockToExecutableData(altChainPayloads[i], common.Big0).ExecutionPayload
						pv1 := &client_types.ExecutableDataV1{}
						pv1.FromExecutableData(p)
						status, err := secondaryClient.NewPayloadV1(ctx, pv1)
						if err != nil {
							t.Fatalf("FAIL (%s): TEST ISSUE - Unable to send new payload: %v", t.TestName, err)
						}
						if status.Status != "VALID" && status.Status != "ACCEPTED" {
							t.Fatalf("FAIL (%s): TEST ISSUE - Invalid payload status, expected VALID, ACCEPTED: %v", t.TestName, status.Status)
						}

						ctx, cancel = context.WithTimeout(t.TestContext, globals.RPCTimeout)
						defer cancel()
						status2, err := secondaryClient.ForkchoiceUpdatedV1(ctx, &api.ForkchoiceStateV1{
							HeadBlockHash:      p.BlockHash,
							SafeBlockHash:      cA.Hash(),
							FinalizedBlockHash: common.Hash{},
						}, nil)
						if err != nil {
							t.Fatalf("FAIL (%s): TEST ISSUE - Unable to send new payload: %v", t.TestName, err)
						}
						if status2.PayloadStatus.Status != "VALID" && status2.PayloadStatus.Status != "SYNCING" {
							t.Fatalf("FAIL (%s): TEST ISSUE - Invalid payload status, expected VALID: %v", t.TestName, status2.PayloadStatus.Status)
						}

					} else {
						invalidBlock := altChainPayloads[i]
						if err != nil {
							t.Fatalf("FAIL (%s): TEST ISSUE - Failed to create block from payload: %v", t.TestName, err)
						}

						if err := secondaryClient.SetBlock(invalidBlock, altChainPayloads[i-1].NumberU64(), altChainPayloads[i-1].Root()); err != nil {
							t.Fatalf("FAIL (%s): TEST ISSUE - Failed to set invalid block: %v", t.TestName, err)
						}
						t.Logf("INFO (%s): Invalid block successfully set %d (%s): %v", t.TestName, i, payloadValidStr, invalidBlock.Hash())
					}
				}
				// Check that the second node has the correct head
				ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
				defer cancel()
				head, err := secondaryClient.HeaderByNumber(ctx, nil)
				if err != nil {
					t.Fatalf("FAIL (%s): TEST ISSUE - Secondary Node unable to reatrieve latest header: %v", t.TestName, err)

				}
				if head.Hash() != altChainPayloads[n-1].Hash() {
					t.Fatalf("FAIL (%s): TEST ISSUE - Secondary Node has invalid blockhash got %v want %v gotNum %v wantNum %d", t.TestName, head.Hash(), altChainPayloads[n-1].Hash(), head.Number, altChainPayloads[n].NumberU64())
				} else {
					t.Logf("INFO (%s): Secondary Node has correct block", t.TestName)
				}

				if !spec.ReOrgFromCanonical {
					// Add the main client as a peer of the secondary client so it is able to sync
					secondaryClient.AddPeer(t.Engine)

					ctx, cancel := context.WithTimeout(t.TimeoutContext, globals.RPCTimeout)
					defer cancel()
					l, err := t.Eth.BlockByNumber(ctx, nil)
					if err != nil {
						t.Fatalf("FAIL (%s): Unable to query main client for latest block: %v", t.TestName, err)
					}
					t.Logf("INFO (%s): Latest block on main client before sync: hash=%v, number=%d", t.TestName, l.Hash(), l.Number())
				}
				// If we are syncing through p2p, we need to keep polling until the client syncs the missing payloads
				for {
					r := t.TestEngine.TestEngineNewPayloadV1(api.BlockToExecutableData(altChainPayloads[n], common.Big0).ExecutionPayload)
					t.Logf("INFO (%s): Response from main client: %v", t.TestName, r.Status)
					s := t.TestEngine.TestEngineForkchoiceUpdatedV1(&api.ForkchoiceStateV1{
						HeadBlockHash:      altChainPayloads[n].Hash(),
						SafeBlockHash:      altChainPayloads[n].Hash(),
						FinalizedBlockHash: common.Hash{},
					}, nil)
					t.Logf("INFO (%s): Response from main client fcu: %v", t.TestName, s.Response.PayloadStatus)

					if r.Status.Status == test.Invalid {
						// We also expect that the client properly returns the LatestValidHash of the block on the
						// side chain that is immediately prior to the invalid payload (or zero if parent is PoW)
						var lvh common.Hash
						if cAHeight.Cmp(big0) != 0 || invalidIndex != 1 {
							// Parent is NOT Proof of Work
							lvh = altChainPayloads[invalidIndex-1].Hash()
						}
						r.ExpectLatestValidHash(&lvh)
						// Response on ForkchoiceUpdated should be the same
						s.ExpectPayloadStatus(test.Invalid)
						s.ExpectLatestValidHash(&lvh)
						break
					} else if test.PayloadStatus(r.Status.Status) == test.Valid {
						ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
						defer cancel()
						latestBlock, err := t.Eth.BlockByNumber(ctx, nil)
						if err != nil {
							t.Fatalf("FAIL (%s): Unable to get latest block: %v", t.TestName, err)
						}

						// Print last n blocks, for debugging
						k := latestBlock.Number().Int64() - int64(n)
						if k < 0 {
							k = 0
						}
						for ; k <= latestBlock.Number().Int64(); k++ {
							ctx, cancel = context.WithTimeout(t.TestContext, globals.RPCTimeout)
							defer cancel()
							latestBlock, err := t.Eth.BlockByNumber(ctx, big.NewInt(k))
							if err != nil {
								t.Fatalf("FAIL (%s): Unable to get block %d: %v", t.TestName, k, err)
							}
							js, _ := json.MarshalIndent(latestBlock.Header(), "", "  ")
							t.Logf("INFO (%s): Block %d: %s", t.TestName, k, js)
						}

						t.Fatalf("FAIL (%s): Client returned VALID on an invalid chain: %v", t.TestName, r.Status)
					}

					select {
					case <-time.After(time.Second):
						continue
					case <-t.TimeoutContext.Done():
						t.Fatalf("FAIL (%s): Timeout waiting for main client to detect invalid chain", t.TestName)
					}
				}

				if !spec.ReOrgFromCanonical {
					// We need to send the canonical chain to the main client here
					for i := t.CLMock.FirstPoSBlockNumber.Uint64(); i <= t.CLMock.LatestExecutedPayload.Number; i++ {
						if payload, ok := t.CLMock.ExecutedPayloadHistory[i]; ok {
							r := t.TestEngine.TestEngineNewPayloadV1(payload)
							r.ExpectStatus(test.Valid)
						}
					}
				}

				// Resend the latest correct fcU
				r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&t.CLMock.LatestForkchoice, nil)
				r.ExpectNoError()
				// After this point, the CL Mock will send the next payload of the canonical chain
			},
		})

	}
}

// Test to verify Block information available at the Eth RPC after NewPayload
func blockStatusExecPayloadGen(transitionBlock bool) func(t *test.Env) {
	return func(t *test.Env) {
		// Wait until this client catches up with latest PoS Block
		t.CLMock.WaitForTTD()

		// Produce blocks before starting the test, only if we are not testing the transition block
		if !transitionBlock {
			t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})
		}

		var tx *types.Transaction
		t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
			OnPayloadProducerSelected: func() {
				var err error
				tx, err = helper.SendNextTransaction(
					t.TestContext,
					t.Engine, &helper.BaseTransactionCreator{
						Recipient: &ZeroAddr,
						Amount:    big1,
						Payload:   nil,
						TxType:    t.TestTransactionType,
						GasLimit:  75000,
					},
				)
				if err != nil {
					t.Fatalf("FAIL (%s): Error trying to send transaction: %v", err)
				}
			},
			// Run test after the new payload has been broadcasted
			OnNewPayloadBroadcast: func() {
				r := t.TestEngine.TestHeaderByNumber(Head)
				r.ExpectHash(t.CLMock.LatestForkchoice.HeadBlockHash)

				s := t.TestEngine.TestBlockNumber()
				s.ExpectNumber(t.CLMock.LatestHeadNumber.Uint64())

				p := t.TestEngine.TestBlockByNumber(Head)
				p.ExpectHash(t.CLMock.LatestForkchoice.HeadBlockHash)

				// Check that the receipt for the transaction we just sent is still not available
				q := t.TestEngine.TestTransactionReceipt(tx.Hash())
				q.ExpectError()
			},
		})
	}
}

// Test to verify Block information available at the Eth RPC after new HeadBlock ForkchoiceUpdated
func blockStatusHeadBlockGen(transitionBlock bool) func(t *test.Env) {
	return func(t *test.Env) {
		// Wait until this client catches up with latest PoS Block
		t.CLMock.WaitForTTD()

		// Produce blocks before starting the test, only if we are not testing the transition block
		if !transitionBlock {
			t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})
		}

		var tx *types.Transaction
		t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
			OnPayloadProducerSelected: func() {
				var err error
				tx, err = helper.SendNextTransaction(
					t.TestContext,
					t.Engine,
					&helper.BaseTransactionCreator{
						Recipient: &ZeroAddr,
						Amount:    big1,
						Payload:   nil,
						TxType:    t.TestTransactionType,
						GasLimit:  75000,
					},
				)
				if err != nil {
					t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
				}
			},
			// Run test after a forkchoice with new HeadBlockHash has been broadcasted
			OnForkchoiceBroadcast: func() {
				r := t.TestEngine.TestHeaderByNumber(Head)
				r.ExpectHash(t.CLMock.LatestForkchoice.HeadBlockHash)

				s := t.TestEngine.TestTransactionReceipt(tx.Hash())
				s.ExpectTransactionHash(tx.Hash())
			},
		})
	}
}

// Test to verify Block information available at the Eth RPC after new SafeBlock ForkchoiceUpdated
func blockStatusSafeBlock(t *test.Env) {
	// On PoW mode, `safe` tag shall return error.
	r := t.TestEngine.TestHeaderByNumber(Safe)
	r.ExpectErrorCode(-39001)

	// Wait until this client catches up with latest PoS Block
	t.CLMock.WaitForTTD()

	// First ForkchoiceUpdated sent was equal to 0x00..00, `safe` should return error now
	p := t.TestEngine.TestHeaderByNumber(Safe)
	p.ExpectErrorCode(-39001)

	t.CLMock.ProduceBlocks(3, clmock.BlockProcessCallbacks{
		// Run test after a forkchoice with new SafeBlockHash has been broadcasted
		OnSafeBlockChange: func() {
			r := t.TestEngine.TestHeaderByNumber(Safe)
			r.ExpectHash(t.CLMock.LatestForkchoice.SafeBlockHash)
		},
	})
}

// Test to verify Block information available at the Eth RPC after new FinalizedBlock ForkchoiceUpdated
func blockStatusFinalizedBlock(t *test.Env) {
	// On PoW mode, `finalized` tag shall return error.
	r := t.TestEngine.TestHeaderByNumber(Finalized)
	r.ExpectErrorCode(-39001)

	// Wait until this client catches up with latest PoS Block
	t.CLMock.WaitForTTD()

	// First ForkchoiceUpdated sent was equal to 0x00..00, `finalized` should return error now
	p := t.TestEngine.TestHeaderByNumber(Finalized)
	p.ExpectErrorCode(-39001)

	t.CLMock.ProduceBlocks(3, clmock.BlockProcessCallbacks{
		// Run test after a forkchoice with new FinalizedBlockHash has been broadcasted
		OnFinalizedBlockChange: func() {
			r := t.TestEngine.TestHeaderByNumber(Finalized)
			r.ExpectHash(t.CLMock.LatestForkchoice.FinalizedBlockHash)
		},
	})
}

func safeFinalizedCanonicalChain(t *test.Env) {
	// Wait until this client catches up with latest PoS Block
	t.CLMock.WaitForTTD()
	gblock := t.Genesis.ToBlock()

	// At the merge we need to have head equal to the last PoW block, and safe/finalized must return error
	h := t.TestEngine.TestHeaderByNumber(Head)
	h.ExpectHash(gblock.Hash())
	h = t.TestEngine.TestHeaderByNumber(Safe)
	h.ExpectErrorCode(-39001)
	h = t.TestEngine.TestHeaderByNumber(Finalized)
	h.ExpectErrorCode(-39001)

	// Producing one PoS payload should change only the head (latest)
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})
	h = t.TestEngine.TestHeaderByNumber(Head)
	h.ExpectHash(t.CLMock.LatestPayloadBuilt.BlockHash)
	h = t.TestEngine.TestHeaderByNumber(Safe)
	h.ExpectErrorCode(-39001)
	h = t.TestEngine.TestHeaderByNumber(Finalized)
	h.ExpectErrorCode(-39001)

	// Producing a second PoS payload should change safe
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})
	h = t.TestEngine.TestHeaderByNumber(Head)
	h.ExpectHash(t.CLMock.LatestPayloadBuilt.BlockHash)
	h = t.TestEngine.TestHeaderByNumber(Safe)
	h.ExpectHash(t.CLMock.ExecutedPayloadHistory[1].BlockHash)
	h = t.TestEngine.TestHeaderByNumber(Finalized)
	h.ExpectErrorCode(-39001)

	// Producing a third PoS payload should change finalized
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})
	h = t.TestEngine.TestHeaderByNumber(Head)
	h.ExpectHash(t.CLMock.LatestPayloadBuilt.BlockHash)
	h = t.TestEngine.TestHeaderByNumber(Safe)
	h.ExpectHash(t.CLMock.ExecutedPayloadHistory[2].BlockHash)
	h = t.TestEngine.TestHeaderByNumber(Finalized)
	h.ExpectHash(t.CLMock.ExecutedPayloadHistory[1].BlockHash)

}

// Test to verify Block information available after a reorg using forkchoiceUpdated
func blockStatusReorg(t *test.Env) {
	// Wait until this client catches up with latest PoS Block
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			// Run using an alternative Payload, verify that the latest info is updated after re-org
			customRandom := common.Hash{}
			rand.Read(customRandom[:])
			customizedPayload, err := helper.CustomizePayload(&t.CLMock.LatestPayloadBuilt, &helper.CustomPayloadData{
				PrevRandao: &customRandom,
			})
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to customize payload: %v", t.TestName, err)
			}

			// Send custom payload and fcU to it
			t.CLMock.BroadcastNewPayload(customizedPayload)
			t.CLMock.BroadcastForkchoiceUpdated(&api.ForkchoiceStateV1{
				HeadBlockHash:      customizedPayload.BlockHash,
				SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
				FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
			}, nil, 1)

			// Verify the client is serving the latest HeadBlock
			r := t.TestEngine.TestHeaderByNumber(Head)
			r.ExpectHash(customizedPayload.BlockHash)

		},
		OnForkchoiceBroadcast: func() {
			// At this point, we have re-org'd to the payload that the CLMocker was originally planning to send,
			// verify that the client is serving the latest HeadBlock.
			r := t.TestEngine.TestHeaderByNumber(Head)
			r.ExpectHash(t.CLMock.LatestForkchoice.HeadBlockHash)

		},
	})

}

// Test that performing a re-org back into a previous block of the canonical chain does not produce errors and the chain
// is still capable of progressing.
func reorgBack(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})

	// We are going to reorg back to this previous hash several times
	previousHash := t.CLMock.LatestForkchoice.HeadBlockHash

	// Produce blocks before starting the test (So we don't try to reorg back to the genesis block)
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{
		OnForkchoiceBroadcast: func() {
			// Send a fcU with the HeadBlockHash pointing back to the previous block
			forkchoiceUpdatedBack := api.ForkchoiceStateV1{
				HeadBlockHash:      previousHash,
				SafeBlockHash:      previousHash,
				FinalizedBlockHash: previousHash,
			}

			// It is only expected that the client does not produce an error and the CL Mocker is able to progress after the re-org
			r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&forkchoiceUpdatedBack, nil)
			r.ExpectNoError()
		},
	})

	// Verify that the client is pointing to the latest payload sent
	r := t.TestEngine.TestBlockByNumber(Head)
	r.ExpectHash(t.CLMock.LatestPayloadBuilt.BlockHash)

}

// Test that performs a re-org to a previously validated payload on a side chain.
func reorgPrevValidatedPayloadOnSideChain(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	var (
		sidechainPayloads     = make([]*api.ExecutableData, 0)
		sidechainPayloadCount = 5
	)

	// Produce a canonical chain while at the same time generate a side chain to which we will re-org.
	t.CLMock.ProduceBlocks(sidechainPayloadCount, clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			// The side chain will consist simply of the same payloads with extra data appended
			extraData := []byte("side")
			customData := helper.CustomPayloadData{
				ExtraData: &extraData,
			}
			if len(sidechainPayloads) > 0 {
				customData.ParentHash = &sidechainPayloads[len(sidechainPayloads)-1].BlockHash
			}
			altPayload, err := helper.CustomizePayload(&t.CLMock.LatestPayloadBuilt, &customData)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to customize payload: %v", t.TestName, err)
			}
			sidechainPayloads = append(sidechainPayloads, altPayload)

			r := t.TestEngine.TestEngineNewPayloadV1(altPayload)
			r.ExpectStatus(test.Valid)
			r.ExpectLatestValidHash(&altPayload.BlockHash)
		},
	})

	// Attempt to re-org to one of the sidechain payloads, but not the leaf,
	// and also build a new payload from this sidechain.
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			prevRandao := common.Hash{}
			rand.Read(prevRandao[:])
			r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&api.ForkchoiceStateV1{
				HeadBlockHash:      sidechainPayloads[len(sidechainPayloads)-2].BlockHash,
				SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
				FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
			}, &api.PayloadAttributes{
				Timestamp:             t.CLMock.LatestHeader.Time,
				Random:                prevRandao,
				SuggestedFeeRecipient: common.Address{},
			})
			r.ExpectPayloadStatus(test.Valid)
			r.ExpectLatestValidHash(&sidechainPayloads[len(sidechainPayloads)-2].BlockHash)

			p := t.TestEngine.TestEngineGetPayloadV1(r.Response.PayloadID)
			p.ExpectPayloadParentHash(sidechainPayloads[len(sidechainPayloads)-2].BlockHash)

			s := t.TestEngine.TestEngineNewPayloadV1(&p.Payload)
			s.ExpectStatus(test.Valid)
			s.ExpectLatestValidHash(&p.Payload.BlockHash)

			// After this, the CLMocker will continue and try to re-org to canonical chain once again
			// CLMocker will fail the test if this is not possible, so nothing left to do.
		},
	})
}

// Test that performs a re-org of the safe block to a side chain.
func safeReorgToSideChain(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// Produce an alternative chain
	sidechainPayloads := make([]*api.ExecutableData, 0)

	// Produce three payloads `P1`, `P2`, `P3`, along with the side chain payloads `P2'`, `P3'`
	// First payload is finalized so no alternative payload
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})
	t.CLMock.ProduceBlocks(2, clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			// Generate an alternative payload by simply adding extraData to the block
			altParentHash := t.CLMock.LatestPayloadBuilt.ParentHash
			if len(sidechainPayloads) > 0 {
				altParentHash = sidechainPayloads[len(sidechainPayloads)-1].BlockHash
			}
			altPayload, err := helper.CustomizePayload(&t.CLMock.LatestPayloadBuilt,
				&helper.CustomPayloadData{
					ParentHash: &altParentHash,
					ExtraData:  &([]byte{0x01}),
				})
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to customize payload: %v", t.TestName, err)
			}
			sidechainPayloads = append(sidechainPayloads, altPayload)
		},
	})

	// Verify current state of labels
	head := t.TestEngine.TestHeaderByNumber(Head)
	head.ExpectHash(t.CLMock.LatestPayloadBuilt.BlockHash)

	safe := t.TestEngine.TestHeaderByNumber(Safe)
	safe.ExpectHash(t.CLMock.ExecutedPayloadHistory[2].BlockHash)

	finalized := t.TestEngine.TestHeaderByNumber(Finalized)
	finalized.ExpectHash(t.CLMock.ExecutedPayloadHistory[1].BlockHash)

	// Re-org the safe/head blocks to point to the alternative side chain
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			for _, p := range sidechainPayloads {
				r := t.TestEngine.TestEngineNewPayloadV1(p)
				r.ExpectStatusEither(test.Valid, test.Accepted)
			}
			r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&api.ForkchoiceStateV1{
				HeadBlockHash:      sidechainPayloads[1].BlockHash,
				SafeBlockHash:      sidechainPayloads[0].BlockHash,
				FinalizedBlockHash: t.CLMock.ExecutedPayloadHistory[1].BlockHash,
			}, nil)
			r.ExpectPayloadStatus(test.Valid)

			head := t.TestEngine.TestHeaderByNumber(Head)
			head.ExpectHash(sidechainPayloads[1].BlockHash)

			safe := t.TestEngine.TestHeaderByNumber(Safe)
			safe.ExpectHash(sidechainPayloads[0].BlockHash)

			finalized := t.TestEngine.TestHeaderByNumber(Finalized)
			finalized.ExpectHash(t.CLMock.ExecutedPayloadHistory[1].BlockHash)

		},
	})
}

// Test that performs a re-org back to the canonical chain after re-org to syncing/unavailable chain.
func reorgBackFromSyncing(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// Produce an alternative chain
	sidechainPayloads := make([]*api.ExecutableData, 0)
	t.CLMock.ProduceBlocks(10, clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			// Generate an alternative payload by simply adding extraData to the block
			altParentHash := t.CLMock.LatestPayloadBuilt.ParentHash
			if len(sidechainPayloads) > 0 {
				altParentHash = sidechainPayloads[len(sidechainPayloads)-1].BlockHash
			}
			altPayload, err := helper.CustomizePayload(&t.CLMock.LatestPayloadBuilt,
				&helper.CustomPayloadData{
					ParentHash: &altParentHash,
					ExtraData:  &([]byte{0x01}),
				})
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to customize payload: %v", t.TestName, err)
			}
			sidechainPayloads = append(sidechainPayloads, altPayload)
		},
	})

	// Produce blocks before starting the test (So we don't try to reorg back to the genesis block)
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			r := t.TestEngine.TestEngineNewPayloadV1(sidechainPayloads[len(sidechainPayloads)-1])
			r.ExpectStatusEither(test.Syncing, test.Accepted)
			r.ExpectLatestValidHash(nil)
			// We are going to send one of the alternative payloads and fcU to it
			forkchoiceUpdatedBack := api.ForkchoiceStateV1{
				HeadBlockHash:      sidechainPayloads[len(sidechainPayloads)-1].BlockHash,
				SafeBlockHash:      sidechainPayloads[len(sidechainPayloads)-2].BlockHash,
				FinalizedBlockHash: sidechainPayloads[len(sidechainPayloads)-3].BlockHash,
			}

			// It is only expected that the client does not produce an error and the CL Mocker is able to progress after the re-org
			s := t.TestEngine.TestEngineForkchoiceUpdatedV1(&forkchoiceUpdatedBack, nil)
			s.ExpectLatestValidHash(nil)
			s.ExpectPayloadStatus(test.Syncing)

			// After this, the CLMocker will continue and try to re-org to canonical chain once again
			// CLMocker will fail the test if this is not possible, so nothing left to do.
		},
	})
}

// Test transaction status after a forkchoiceUpdated re-orgs to an alternative hash where a transaction is not present
func transactionReorg(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test (So we don't try to reorg back to the genesis block)
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	// Create transactions that modify the state in order to check after the reorg.
	var (
		txCount            = 5
		sstoreContractAddr = common.HexToAddress("0000000000000000000000000000000000000317")
	)

	for i := 0; i < txCount; i++ {
		var (
			noTxnPayload api.ExecutableData
			tx           *types.Transaction
		)
		// Generate two payloads, one with the transaction and the other one without it
		t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
			OnPayloadProducerSelected: func() {
				// At this point we have not broadcast the transaction,
				// therefore any payload we get should not contain any
				t.CLMock.RequestNextPayload()
				t.CLMock.GetNextPayload()
				noTxnPayload = t.CLMock.LatestPayloadBuilt
				if len(noTxnPayload.Transactions) != 0 {
					t.Fatalf("FAIL (%s): Empty payload contains transactions: %v", t.TestName, noTxnPayload)
				}

				// At this point we can broadcast the transaction and it will be included in the next payload
				// Data is the key where a `1` will be stored
				data := common.LeftPadBytes([]byte{byte(i)}, 32)
				t.Logf("transactionReorg, i=%v, data=%v\n", i, data)
				var err error
				tx, err = helper.SendNextTransaction(
					t.TestContext,
					t.Engine,
					&helper.BaseTransactionCreator{
						Recipient: &sstoreContractAddr,
						Amount:    big0,
						Payload:   data,
						TxType:    t.TestTransactionType,
						GasLimit:  75000,
					},
				)
				if err != nil {
					t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
				}

				// Get the receipt
				ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
				defer cancel()
				receipt, _ := t.Eth.TransactionReceipt(ctx, tx.Hash())
				if receipt != nil {
					t.Fatalf("FAIL (%s): Receipt obtained before tx included in block: %v", t.TestName, receipt)
				}
			},
			OnGetPayload: func() {
				// Check that indeed the payload contains the transaction
				if !helper.TransactionInPayload(&t.CLMock.LatestPayloadBuilt, tx) {
					t.Fatalf("FAIL (%s): Payload built does not contain the transaction: %v", t.TestName, t.CLMock.LatestPayloadBuilt)
				}
			},
			OnForkchoiceBroadcast: func() {
				// Transaction is now in the head of the canonical chain, re-org and verify it's removed
				// Get the receipt
				ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
				defer cancel()
				_, err := t.Eth.TransactionReceipt(ctx, tx.Hash())
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to obtain transaction receipt: %v", t.TestName, err)
				}

				if noTxnPayload.ParentHash != t.CLMock.LatestPayloadBuilt.ParentHash {
					t.Fatalf("FAIL (%s): Incorrect parent hash for payloads: %v != %v", t.TestName, noTxnPayload.ParentHash, t.CLMock.LatestPayloadBuilt.ParentHash)
				}
				if noTxnPayload.BlockHash == t.CLMock.LatestPayloadBuilt.BlockHash {
					t.Fatalf("FAIL (%s): Incorrect hash for payloads: %v == %v", t.TestName, noTxnPayload.BlockHash, t.CLMock.LatestPayloadBuilt.BlockHash)
				}

				r := t.TestEngine.TestEngineNewPayloadV1(&noTxnPayload)
				r.ExpectStatus(test.Valid)
				r.ExpectLatestValidHash(&noTxnPayload.BlockHash)

				s := t.TestEngine.TestEngineForkchoiceUpdatedV1(&api.ForkchoiceStateV1{
					HeadBlockHash:      noTxnPayload.BlockHash,
					SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
					FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
				}, nil)
				s.ExpectPayloadStatus(test.Valid)

				p := t.TestEngine.TestBlockByNumber(Head)
				p.ExpectHash(noTxnPayload.BlockHash)

				ctx, cancel = context.WithTimeout(t.TestContext, globals.RPCTimeout)
				defer cancel()
				reorgReceipt, err := t.Eth.TransactionReceipt(ctx, tx.Hash())
				if reorgReceipt != nil {
					t.Fatalf("FAIL (%s): Receipt was obtained when the tx had been re-org'd out: %v", t.TestName, reorgReceipt)
				}

				// Re-org back
				t.CLMock.BroadcastForkchoiceUpdated(&t.CLMock.LatestForkchoice, nil, 1)
			},
		})

	}

}

// Test transaction blockhash after a forkchoiceUpdated re-orgs to an alternative block with the same transaction
func transactionReorgBlockhash(newNPOnRevert bool) func(t *test.Env) {
	return func(t *test.Env) {
		// Wait until this client catches up with latest PoS
		t.CLMock.WaitForTTD()

		// Produce blocks before starting the test (So we don't try to reorg back to the genesis block)
		t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

		// Create transactions that modify the state in order to check after the reorg.
		var (
			txCount            = 5
			sstoreContractAddr = common.HexToAddress("0000000000000000000000000000000000000317")
		)

		for i := 0; i < txCount; i++ {
			var (
				mainPayload *api.ExecutableData
				sidePayload *api.ExecutableData
				tx          *types.Transaction
			)

			t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
				OnPayloadProducerSelected: func() {
					// At this point we have not broadcast the transaction,
					// therefore any payload we get should not contain any
					t.CLMock.RequestNextPayload()
					t.CLMock.GetNextPayload()

					// Create the transaction
					data := common.LeftPadBytes([]byte{byte(i)}, 32)
					t.Logf("transactionReorg, i=%v, data=%v\n", i, data)
					var err error
					tx, err = helper.SendNextTransaction(
						t.TestContext,
						t.Engine,
						&helper.BaseTransactionCreator{
							Recipient: &sstoreContractAddr,
							Amount:    big0,
							Payload:   data,
							TxType:    t.TestTransactionType,
							GasLimit:  75000,
						},
					)
					if err != nil {
						t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
					}

					// Get the receipt
					ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
					defer cancel()
					receipt, _ := t.Eth.TransactionReceipt(ctx, tx.Hash())
					if receipt != nil {
						t.Fatalf("FAIL (%s): Receipt obtained before tx included in block: %v", t.TestName, receipt)
					}
				},
				OnGetPayload: func() {
					// Check that indeed the payload contains the transaction
					if !helper.TransactionInPayload(&t.CLMock.LatestPayloadBuilt, tx) {
						t.Fatalf("FAIL (%s): Payload built does not contain the transaction: %v", t.TestName, t.CLMock.LatestPayloadBuilt)
					}

					mainPayload = &t.CLMock.LatestPayloadBuilt

					// Create side payload with different hash
					var err error
					sidePayload, err = helper.CustomizePayload(mainPayload, &helper.CustomPayloadData{
						ExtraData: &([]byte{0x01}),
					})
					if err != nil {
						t.Fatalf("Error creating reorg payload %v", err)
					}

					if sidePayload.ParentHash != mainPayload.ParentHash {
						t.Fatalf("FAIL (%s): Incorrect parent hash for payloads: %v != %v", t.TestName, sidePayload.ParentHash, t.CLMock.LatestPayloadBuilt.ParentHash)
					}
					if sidePayload.BlockHash == mainPayload.BlockHash {
						t.Fatalf("FAIL (%s): Incorrect hash for payloads: %v == %v", t.TestName, sidePayload.BlockHash, t.CLMock.LatestPayloadBuilt.BlockHash)
					}

				},
				OnForkchoiceBroadcast: func() {

					// At first, the tx will be on main payload
					txt := t.TestEngine.TestTransactionReceipt(tx.Hash())
					txt.ExpectBlockHash(mainPayload.BlockHash)

					r := t.TestEngine.TestEngineNewPayloadV1(sidePayload)
					r.ExpectStatus(test.Valid)
					r.ExpectLatestValidHash(&sidePayload.BlockHash)

					s := t.TestEngine.TestEngineForkchoiceUpdatedV1(&api.ForkchoiceStateV1{
						HeadBlockHash:      sidePayload.BlockHash,
						SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
						FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
					}, nil)
					s.ExpectPayloadStatus(test.Valid)

					p := t.TestEngine.TestBlockByNumber(Head)
					p.ExpectHash(sidePayload.BlockHash)

					// Now it should be with sidePayload
					txt = t.TestEngine.TestTransactionReceipt(tx.Hash())
					txt.ExpectBlockHash(sidePayload.BlockHash)

					// Re-org back to main payload
					if newNPOnRevert {
						r = t.TestEngine.TestEngineNewPayloadV1(mainPayload)
						r.ExpectStatus(test.Valid)
						r.ExpectLatestValidHash(&mainPayload.BlockHash)
					}
					t.CLMock.BroadcastForkchoiceUpdated(&api.ForkchoiceStateV1{
						HeadBlockHash:      mainPayload.BlockHash,
						SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
						FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
					}, nil, 1)

					// Not it should be back with main payload
					txt = t.TestEngine.TestTransactionReceipt(tx.Hash())
					txt.ExpectBlockHash(mainPayload.BlockHash)
				},
			})

		}

	}
}

// Reorg to a Sidechain using ForkchoiceUpdated
func sidechainReorg(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	// Produce two payloads, send fcU with first payload, check transaction outcome, then reorg, check transaction outcome again

	// This single transaction will change its outcome based on the payload
	tx, err := helper.SendNextTransaction(
		t.TestContext,
		t.Engine,
		&helper.BaseTransactionCreator{
			Recipient: &globals.PrevRandaoContractAddr,
			Amount:    big0,
			Payload:   nil,
			TxType:    t.TestTransactionType,
			GasLimit:  75000,
		},
	)
	if err != nil {
		t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
	}
	t.Logf("INFO (%s): sent tx %v", t.TestName, tx.Hash())

	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnNewPayloadBroadcast: func() {
			// At this point the CLMocker has a payload that will result in a specific outcome,
			// we can produce an alternative payload, send it, fcU to it, and verify the changes
			alternativePrevRandao := common.Hash{}
			rand.Read(alternativePrevRandao[:])

			r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&t.CLMock.LatestForkchoice,
				&api.PayloadAttributes{
					Timestamp:             t.CLMock.LatestHeader.Time + 1,
					Random:                alternativePrevRandao,
					SuggestedFeeRecipient: t.CLMock.NextFeeRecipient,
				})
			r.ExpectNoError()

			time.Sleep(t.CLMock.PayloadProductionClientDelay)

			ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
			defer cancel()
			alternativePayload, err := t.Engine.GetPayloadV1(ctx, r.Response.PayloadID)
			if err != nil {
				t.Fatalf("FAIL (%s): Could not get alternative payload: %v", t.TestName, err)
			}
			if len(alternativePayload.Transactions) == 0 {
				t.Fatalf("FAIL (%s): alternative payload does not contain the prevRandao opcode tx", t.TestName)
			}

			s := t.TestEngine.TestEngineNewPayloadV1(&alternativePayload)
			s.ExpectStatus(test.Valid)
			s.ExpectLatestValidHash(&alternativePayload.BlockHash)

			// We sent the alternative payload, fcU to it
			p := t.TestEngine.TestEngineForkchoiceUpdatedV1(&api.ForkchoiceStateV1{
				HeadBlockHash:      alternativePayload.BlockHash,
				SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
				FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
			}, nil)
			p.ExpectPayloadStatus(test.Valid)

			// PrevRandao should be the alternative prevRandao we sent
			checkPrevRandaoValue(t, alternativePrevRandao, alternativePayload.Number)
		},
	})
	// The reorg actually happens after the CLMocker continues,
	// verify here that the reorg was successful
	latestBlockNum := t.CLMock.LatestHeadNumber.Uint64()
	checkPrevRandaoValue(t, t.CLMock.PrevRandaoHistory[latestBlockNum], latestBlockNum)

}

// Re-Execute Previous Payloads
func reExecPayloads(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// How many Payloads we are going to re-execute
	var payloadReExecCount = 10

	// Create those blocks
	t.CLMock.ProduceBlocks(payloadReExecCount, clmock.BlockProcessCallbacks{})

	// Re-execute the payloads
	r := t.TestEngine.TestBlockNumber()
	r.ExpectNoError()
	lastBlock := r.Number
	t.Logf("INFO (%s): Started re-executing payloads at block: %v", t.TestName, lastBlock)

	for i := lastBlock - uint64(payloadReExecCount) + 1; i <= lastBlock; i++ {
		payload, found := t.CLMock.ExecutedPayloadHistory[i]
		if !found {
			t.Fatalf("FAIL (%s): (test issue) Payload with index %d does not exist", i)
		}

		r := t.TestEngine.TestEngineNewPayloadV1(payload)
		r.ExpectStatus(test.Valid)
		r.ExpectLatestValidHash(&payload.BlockHash)
	}
}

// Multiple New Payloads Extending Canonical Chain
func multipleNewCanonicalPayloads(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		// Run test after a new payload has been obtained
		OnGetPayload: func() {
			payloadCount := 80
			basePayload := t.CLMock.LatestPayloadBuilt

			// Fabricate and send multiple new payloads by changing the PrevRandao field
			for i := 0; i < payloadCount; i++ {
				newPrevRandao := common.Hash{}
				rand.Read(newPrevRandao[:])
				newPayload, err := helper.CustomizePayload(&basePayload, &helper.CustomPayloadData{
					PrevRandao: &newPrevRandao,
				})
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to customize payload %v: %v", t.TestName, i, err)
				}

				r := t.TestEngine.TestEngineNewPayloadV1(newPayload)
				r.ExpectStatus(test.Valid)
				r.ExpectLatestValidHash(&newPayload.BlockHash)
			}
		},
	})
	// At the end the CLMocker continues to try to execute fcU with the original payload, which should not fail
}

// Consecutive Payload Execution: Secondary client should be able to set the forkchoiceUpdated to payloads received consecutively
func inOrderPayloads(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// First prepare payloads on a first client, which will also contain multiple transactions

	// We will be also verifying that the transactions are correctly interpreted in the canonical chain,
	// prepare a random account to receive funds.
	recipient := common.Address{}
	rand.Read(recipient[:])
	amountPerTx := big.NewInt(1000)
	txPerPayload := 20
	payloadCount := 10

	t.CLMock.ProduceBlocks(payloadCount, clmock.BlockProcessCallbacks{
		// We send the transactions after we got the Payload ID, before the CLMocker gets the prepared Payload
		OnPayloadProducerSelected: func() {
			for i := 0; i < txPerPayload; i++ {
				_, err := helper.SendNextTransaction(
					t.TestContext,
					t.CLMock.NextBlockProducer,
					&helper.BaseTransactionCreator{
						Recipient: &recipient,
						Amount:    amountPerTx,
						Payload:   nil,
						TxType:    t.TestTransactionType,
						GasLimit:  75000,
					},
				)
				if err != nil {
					t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
				}
			}
		},
		OnGetPayload: func() {
			if len(t.CLMock.LatestPayloadBuilt.Transactions) != txPerPayload {
				t.Fatalf("FAIL (%s): Client failed to include all the expected transactions in payload built: %d < %d", t.TestName, len(t.CLMock.LatestPayloadBuilt.Transactions), txPerPayload)
			}
		},
	})

	expectedBalance := amountPerTx.Mul(amountPerTx, big.NewInt(int64(payloadCount*txPerPayload)))

	// Check balance on this first client
	r := t.TestEngine.TestBalanceAt(recipient, nil)
	r.ExpectBalanceEqual(expectedBalance)

	// Start a second client to send newPayload consecutively without fcU
	secondaryClient, err := hive_rpc.HiveRPCEngineStarter{}.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles)
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to start secondary client: %v", t.TestName, err)
	}
	secondaryTestEngineClient := test.NewTestEngineClient(t, secondaryClient)

	// Send the forkchoiceUpdated with the LatestExecutedPayload hash, we should get SYNCING back
	fcU := api.ForkchoiceStateV1{
		HeadBlockHash:      t.CLMock.LatestExecutedPayload.BlockHash,
		SafeBlockHash:      t.CLMock.LatestExecutedPayload.BlockHash,
		FinalizedBlockHash: t.CLMock.LatestExecutedPayload.BlockHash,
	}

	s := secondaryTestEngineClient.TestEngineForkchoiceUpdatedV1(&fcU, nil)
	s.ExpectPayloadStatus(test.Syncing)
	s.ExpectLatestValidHash(nil)
	s.ExpectNoValidationError()

	// Send all the payloads in the opposite order
	for k := t.CLMock.FirstPoSBlockNumber.Uint64(); k <= t.CLMock.LatestExecutedPayload.Number; k++ {
		payload := t.CLMock.ExecutedPayloadHistory[k]

		s := secondaryTestEngineClient.TestEngineNewPayloadV1(payload)
		s.ExpectStatus(test.Valid)
		s.ExpectLatestValidHash(&payload.BlockHash)

	}

	// Add the client to the CLMocker
	t.CLMock.AddEngineClient(secondaryClient)

	// Produce a single block on top of the canonical chain, all clients must accept this
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})

	// Head must point to the latest produced payload
	p := secondaryTestEngineClient.TestBlockByNumber(nil)
	p.ExpectHash(t.CLMock.LatestExecutedPayload.BlockHash)
	// At this point we should have our funded account balance equal to the expected value.
	q := secondaryTestEngineClient.TestBalanceAt(recipient, nil)
	q.ExpectBalanceEqual(expectedBalance)

}

// Send a valid payload on a client that is currently SYNCING
func validPayloadFcUSyncingClient(t *test.Env) {
	var (
		secondaryClient client.EngineClient
		previousPayload api.ExecutableData
	)
	{
		// To allow sending the primary engine client into SYNCING state, we need a secondary client to guide the payload creation
		var err error
		secondaryClient, err = hive_rpc.HiveRPCEngineStarter{}.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles)

		if err != nil {
			t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
		}
		t.CLMock.AddEngineClient(secondaryClient)
	}

	// Wait until TTD is reached by all clients
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	// Disconnect the first engine client from the CL Mocker and produce a block
	t.CLMock.RemoveEngineClient(t.Engine)
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})

	previousPayload = t.CLMock.LatestPayloadBuilt

	// Send the fcU to set it to syncing mode
	r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&t.CLMock.LatestForkchoice, nil)
	r.ExpectPayloadStatus(test.Syncing)

	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		// Run test after the new payload has been obtained
		OnGetPayload: func() {
			// Send the new payload from the second client to the first, it won't be able to validate it
			r := t.TestEngine.TestEngineNewPayloadV1(&t.CLMock.LatestPayloadBuilt)
			r.ExpectStatusEither(test.Accepted, test.Syncing)
			r.ExpectLatestValidHash(nil)

			// Send the forkchoiceUpdated with a reference to the valid payload on the SYNCING client.
			s := t.TestEngine.TestEngineForkchoiceUpdatedV1(&api.ForkchoiceStateV1{
				HeadBlockHash:      t.CLMock.LatestPayloadBuilt.BlockHash,
				SafeBlockHash:      t.CLMock.LatestPayloadBuilt.BlockHash,
				FinalizedBlockHash: t.CLMock.LatestPayloadBuilt.BlockHash,
			}, &api.PayloadAttributes{
				Timestamp:             t.CLMock.LatestPayloadBuilt.Timestamp + 1,
				Random:                common.Hash{},
				SuggestedFeeRecipient: common.Address{},
			})
			s.ExpectPayloadStatus(test.Syncing)

			// Send the previous payload to be able to continue
			p := t.TestEngine.TestEngineNewPayloadV1(&previousPayload)
			p.ExpectStatus(test.Valid)
			p.ExpectLatestValidHash(&previousPayload.BlockHash)

			// Send the new payload again

			p = t.TestEngine.TestEngineNewPayloadV1(&t.CLMock.LatestPayloadBuilt)
			p.ExpectStatus(test.Valid)
			p.ExpectLatestValidHash(&t.CLMock.LatestPayloadBuilt.BlockHash)

			s = t.TestEngine.TestEngineForkchoiceUpdatedV1(&api.ForkchoiceStateV1{
				HeadBlockHash:      t.CLMock.LatestPayloadBuilt.BlockHash,
				SafeBlockHash:      t.CLMock.LatestPayloadBuilt.BlockHash,
				FinalizedBlockHash: t.CLMock.LatestPayloadBuilt.BlockHash,
			}, nil)
			s.ExpectPayloadStatus(test.Valid)

		},
	})

	// Add the secondary client again to the CL Mocker
	t.CLMock.AddEngineClient(t.Engine)

	t.CLMock.RemoveEngineClient(secondaryClient)
}

// Send a valid `newPayload` in correct order but skip `forkchoiceUpdated` until the last payload
func missingFcu(t *test.Env) {
	// Wait until TTD is reached by this client
	t.CLMock.WaitForTTD()

	// Get last PoW block hash
	lastPoWBlockHash := t.TestEngine.TestBlockByNumber(Head).Block.Hash()

	// Produce blocks on the main client, these payloads will be replayed on the secondary client.
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	var secondaryEngineTest *test.TestEngineClient
	{
		secondaryEngine, err := hive_rpc.HiveRPCEngineStarter{}.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles)

		if err != nil {
			t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
		}
		secondaryEngineTest = test.NewTestEngineClient(t, secondaryEngine)
		t.CLMock.AddEngineClient(secondaryEngine)
	}

	// Send each payload in the correct order but skip the ForkchoiceUpdated for each
	for i := t.CLMock.FirstPoSBlockNumber.Uint64(); i <= t.CLMock.LatestHeadNumber.Uint64(); i++ {
		payload := t.CLMock.ExecutedPayloadHistory[i]
		p := secondaryEngineTest.TestEngineNewPayloadV1(payload)
		p.ExpectStatus(test.Valid)
		p.ExpectLatestValidHash(&payload.BlockHash)
	}

	// Verify that at this point, the client's head still points to the last non-PoS block
	r := secondaryEngineTest.TestBlockByNumber(Head)
	r.ExpectHash(lastPoWBlockHash)

	// Verify that the head correctly changes after the last ForkchoiceUpdated
	fcU := api.ForkchoiceStateV1{
		HeadBlockHash:      t.CLMock.ExecutedPayloadHistory[t.CLMock.LatestHeadNumber.Uint64()].BlockHash,
		SafeBlockHash:      t.CLMock.ExecutedPayloadHistory[t.CLMock.LatestHeadNumber.Uint64()-1].BlockHash,
		FinalizedBlockHash: t.CLMock.ExecutedPayloadHistory[t.CLMock.LatestHeadNumber.Uint64()-2].BlockHash,
	}
	p := secondaryEngineTest.TestEngineForkchoiceUpdatedV1(&fcU, nil)
	p.ExpectPayloadStatus(test.Valid)
	p.ExpectLatestValidHash(&fcU.HeadBlockHash)

	// Now the head should've changed to the latest PoS block
	s := secondaryEngineTest.TestBlockByNumber(Head)
	s.ExpectHash(fcU.HeadBlockHash)

}

// Build on top of the latest valid payload after an invalid payload has been received:
// P <- INV_P, newPayload(INV_P), fcU(head: P, payloadAttributes: attrs) + getPayload(…)
func payloadBuildAfterNewInvalidPayload(t *test.Env) {
	// Add a second client to build the invalid payload
	secondaryEngine, err := hive_rpc.HiveRPCEngineStarter{}.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles)

	if err != nil {
		t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
	}
	secondaryEngineTest := test.NewTestEngineClient(t, secondaryEngine)
	t.CLMock.AddEngineClient(secondaryEngine)

	// Wait until TTD is reached by this client
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	// Produce another block, but at the same time send an invalid payload from the other client
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {
			// We are going to use the client that was not selected
			// by the CLMocker to produce the invalid payload
			invalidPayloadProducer := t.TestEngine
			if t.CLMock.NextBlockProducer == invalidPayloadProducer.Engine {
				invalidPayloadProducer = secondaryEngineTest
			}
			var inv_p *api.ExecutableData

			{
				// Get a payload from the invalid payload producer and invalidate it
				r := invalidPayloadProducer.TestEngineForkchoiceUpdatedV1(&t.CLMock.LatestForkchoice, &api.PayloadAttributes{
					Timestamp:             t.CLMock.LatestHeader.Time + 1,
					Random:                common.Hash{},
					SuggestedFeeRecipient: common.Address{},
				})
				r.ExpectPayloadStatus(test.Valid)
				// Wait for the payload to be produced by the EL
				time.Sleep(time.Second)

				s := invalidPayloadProducer.TestEngineGetPayloadV1(r.Response.PayloadID)
				s.ExpectNoError()

				inv_p, err = helper.GenerateInvalidPayload(&s.Payload, helper.InvalidStateRoot)
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to invalidate payload: %v", t.TestName, err)
				}
			}

			// Broadcast the invalid payload
			r := t.TestEngine.TestEngineNewPayloadV1(inv_p)
			r.ExpectStatus(test.Invalid)
			r.ExpectLatestValidHash(&t.CLMock.LatestForkchoice.HeadBlockHash)
			s := secondaryEngineTest.TestEngineNewPayloadV1(inv_p)
			s.ExpectStatus(test.Invalid)
			s.ExpectLatestValidHash(&t.CLMock.LatestForkchoice.HeadBlockHash)

			// Let the block production continue.
			// At this point the selected payload producer will
			// try to continue creating a valid payload.
		},
	})
}

// Attempt to produce a payload after a transaction with an invalid Chain ID was sent to the client
// using `eth_sendRawTransaction`.
func buildPayloadWithInvalidChainIDTx(t *test.Env) {
	// Wait until TTD is reached by this client
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	// Send a transaction with an incorrect ChainID.
	// Transaction must be not be included in payload creation.
	var invalidChainIDTx *types.Transaction
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		// Run test after a new payload has been broadcast
		OnPayloadProducerSelected: func() {
			ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
			defer cancel()
			nonce, err := t.CLMock.NextBlockProducer.NonceAt(ctx, globals.VaultAccountAddress, nil)
			if err != nil {
				t.Fatalf("FAIL(%s): Unable to get address nonce: %v", t.TestName, err)
			}
			txData := &types.LegacyTx{
				Nonce:    nonce,
				To:       &globals.PrevRandaoContractAddr,
				Value:    big0,
				Gas:      75000,
				GasPrice: globals.GasPrice,
				Data:     nil,
			}
			invalidChainID := new(big.Int).Set(globals.ChainID)
			invalidChainID.Add(invalidChainID, big1)
			invalidChainIDTx, err := types.SignTx(types.NewTx(txData), types.NewLondonSigner(invalidChainID), globals.VaultKey)
			if err != nil {
				t.Fatalf("FAIL(%s): Unable to sign tx with invalid chain ID: %v", t.TestName, err)
			}
			ctx, cancel = context.WithTimeout(t.TestContext, globals.RPCTimeout)
			defer cancel()
			err = t.Engine.SendTransaction(ctx, invalidChainIDTx)
			if err != nil {
				t.Logf("INFO (%s): Error on sending transaction with incorrect chain ID (Expected): %v", t.TestName, err)
			}
		},
	})

	// Verify that the latest payload built does NOT contain the invalid chain Tx
	if helper.TransactionInPayload(&t.CLMock.LatestPayloadBuilt, invalidChainIDTx) {
		p, _ := json.MarshalIndent(t.CLMock.LatestPayloadBuilt, "", " ")
		t.Fatalf("FAIL (%s): Invalid chain ID tx was included in payload: %s", t.TestName, p)
	}

}

// Fee Recipient Tests
func suggestedFeeRecipient(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// Amount of transactions to send
	txCount := 20

	// Verify that, in a block with transactions, fees are accrued by the suggestedFeeRecipient
	feeRecipient := common.Address{}
	rand.Read(feeRecipient[:])

	// Send multiple transactions
	for i := 0; i < txCount; i++ {
		_, err := helper.SendNextTransaction(
			t.TestContext,
			t.Engine,
			&helper.BaseTransactionCreator{
				Recipient: &globals.VaultAccountAddress,
				Amount:    big0,
				Payload:   nil,
				TxType:    t.TestTransactionType,
				GasLimit:  75000,
			},
		)
		if err != nil {
			t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
		}
	}
	// Produce the next block with the fee recipient set
	t.CLMock.NextFeeRecipient = feeRecipient
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})

	// Calculate the fees and check that they match the balance of the fee recipient
	r := t.TestEngine.TestBlockByNumber(Head)
	r.ExpectTransactionCountEqual(txCount)
	r.ExpectCoinbase(feeRecipient)
	blockIncluded := r.Block

	feeRecipientFees := big.NewInt(0)
	for _, tx := range blockIncluded.Transactions() {
		effGasTip, err := tx.EffectiveGasTip(blockIncluded.BaseFee())
		if err != nil {
			t.Fatalf("FAIL (%s): unable to obtain EffectiveGasTip: %v", t.TestName, err)
		}
		ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
		defer cancel()
		receipt, err := t.Eth.TransactionReceipt(ctx, tx.Hash())
		if err != nil {
			t.Fatalf("FAIL (%s): unable to obtain receipt: %v", t.TestName, err)
		}
		feeRecipientFees = feeRecipientFees.Add(feeRecipientFees, effGasTip.Mul(effGasTip, big.NewInt(int64(receipt.GasUsed))))
	}

	s := t.TestEngine.TestBalanceAt(feeRecipient, nil)
	s.ExpectBalanceEqual(feeRecipientFees)

	// Produce another block without txns and get the balance again
	t.CLMock.NextFeeRecipient = feeRecipient
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})

	s = t.TestEngine.TestBalanceAt(feeRecipient, nil)
	s.ExpectBalanceEqual(feeRecipientFees)

}

// TODO: Do a PENDING block suggestedFeeRecipient

func checkPrevRandaoValue(t *test.Env, expectedPrevRandao common.Hash, blockNumber uint64) {
	storageKey := common.Hash{}
	storageKey[31] = byte(blockNumber)
	r := t.TestEngine.TestStorageAt(globals.PrevRandaoContractAddr, storageKey, nil)
	r.ExpectStorageEqual(expectedPrevRandao)

}

// PrevRandao Opcode tests
func prevRandaoOpcodeTx(t *test.Env) {
	// We need to send PREVRANDAO opcode transactions in PoW and particularly in the block where the TTD is reached.
	ttdReached := make(chan interface{})

	// Try to send many transactions before PoW transition to guarantee at least one enters in the block
	go func(t *test.Env) {
		for {
			_, err := helper.SendNextTransaction(
				t.TestContext,
				t.Engine,
				&helper.BaseTransactionCreator{
					Recipient: &globals.PrevRandaoContractAddr,
					Amount:    big0,
					Payload:   nil,
					TxType:    t.TestTransactionType,
					GasLimit:  75000,
				},
			)
			if err != nil {
				t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
			}

			select {
			case <-t.TimeoutContext.Done():
				t.Fatalf("FAIL (%s): Timeout while sending PREVRANDAO opcode transactions: %v")
			case <-ttdReached:
				return
			case <-time.After(time.Second / 10):
			}
		}
	}(t)
	t.CLMock.WaitForTTD()
	close(ttdReached)

	// Ideally all blocks up until TTD must have a DIFFICULTY opcode tx in it
	r := t.TestEngine.TestBlockNumber()
	r.ExpectNoError()
	ttdBlockNumber := r.Number

	// Start
	for i := uint64(ttdBlockNumber); i <= ttdBlockNumber; i++ {
		// First check that the block actually contained the transaction
		r := t.TestEngine.TestBlockByNumber(big.NewInt(int64(i)))
		r.ExpectTransactionCountGreaterThan(0)

		storageKey := common.Hash{}
		storageKey[31] = byte(i)
		s := t.TestEngine.TestStorageAt(globals.PrevRandaoContractAddr, storageKey, nil)
		s.ExpectBigIntStorageEqual(big.NewInt(2))

	}

	// Send transactions now past TTD, the value of the storage in these blocks must match the prevRandao value
	var (
		txCount        = 10
		currentTxIndex = 0
		txs            = make([]*types.Transaction, 0)
	)
	t.CLMock.ProduceBlocks(txCount, clmock.BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {
			tx, err := helper.SendNextTransaction(
				t.TestContext,
				t.Engine,
				&helper.BaseTransactionCreator{
					Recipient: &globals.PrevRandaoContractAddr,
					Amount:    big0,
					Payload:   nil,
					TxType:    t.TestTransactionType,
					GasLimit:  75000,
				},
			)
			if err != nil {
				t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
			}
			txs = append(txs, tx)
			currentTxIndex++
		},
		OnForkchoiceBroadcast: func() {
			// Check the transaction tracing, which is client specific
			expectedPrevRandao := t.CLMock.PrevRandaoHistory[t.CLMock.LatestHeader.Number.Uint64()+1]
			ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
			defer cancel()
			if err := helper.DebugPrevRandaoTransaction(ctx, t.Client.RPC(), t.Client.Type, txs[currentTxIndex-1],
				&expectedPrevRandao); err != nil {
				t.Fatalf("FAIL (%s): Error during transaction tracing: %v", t.TestName, err)
			}
		},
	})

	ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
	defer cancel()
	lastBlockNumber, err := t.Eth.BlockNumber(ctx)
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to get latest block number: %v", t.TestName, err)
	}
	for i := ttdBlockNumber + 1; i <= lastBlockNumber; i++ {
		checkPrevRandaoValue(t, t.CLMock.PrevRandaoHistory[i], i)
	}

}
