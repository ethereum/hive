//go:build pbtgen

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"math/big"
	"os"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Anchor state: one account per embedding rule.
var (
	eoaBalance      = common.HexToAddress("0x0000000000000000000000000000000000000101")
	eoaNonce        = common.HexToAddress("0x0000000000000000000000000000000000000102")
	eoaMaxima       = common.HexToAddress("0x0000000000000000000000000000000000000103")
	precompile      = common.HexToAddress("0x0000000000000000000000000000000000000004")
	codeOneByte     = common.HexToAddress("0x0000000000000000000000000000000000000201")
	codeChunkExact  = common.HexToAddress("0x0000000000000000000000000000000000000202")
	codeChunkPlus   = common.HexToAddress("0x0000000000000000000000000000000000000203")
	codeStraddle    = common.HexToAddress("0x0000000000000000000000000000000000000204")
	codeZeroChunk   = common.HexToAddress("0x0000000000000000000000000000000000000205")
	codeAllZero     = common.HexToAddress("0x0000000000000000000000000000000000000206")
	codeTwoGroups   = common.HexToAddress("0x0000000000000000000000000000000000000207")
	codeFakeDelega  = common.HexToAddress("0x0000000000000000000000000000000000000208")
	codeFake23      = common.HexToAddress("0x0000000000000000000000000000000000000209")
	codePushPastEnd = common.HexToAddress("0x000000000000000000000000000000000000020a")
	codeSmallPush   = common.HexToAddress("0x000000000000000000000000000000000000020b")
	codeTrailZero   = common.HexToAddress("0x000000000000000000000000000000000000020c")
	codeZeroBalance = common.HexToAddress("0x000000000000000000000000000000000000020d")
	codeWithNonce   = common.HexToAddress("0x000000000000000000000000000000000000020e")
	sharedA         = common.HexToAddress("0x0000000000000000000000000000000000000301")
	sharedB         = common.HexToAddress("0x0000000000000000000000000000000000000302")
	delegatedA      = common.HexToAddress("0x0000000000000000000000000000000000000401")
	delegatedB      = common.HexToAddress("0x0000000000000000000000000000000000000402")
	delegatedC      = common.HexToAddress("0x0000000000000000000000000000000000000403")
	delegatedToNone = common.HexToAddress("0x0000000000000000000000000000000000000404")
	delegatedToEOA  = common.HexToAddress("0x0000000000000000000000000000000000000405")
	delegatedChain  = common.HexToAddress("0x0000000000000000000000000000000000000406")
	delegateTarget  = common.HexToAddress("0x00000000000000000000000000000000000004ff")
	storageSpread   = common.HexToAddress("0x0000000000000000000000000000000000000501")
	storageValues   = common.HexToAddress("0x0000000000000000000000000000000000000502")
	storageOnEOA    = common.HexToAddress("0x0000000000000000000000000000000000000503")
	storageOverflow = common.HexToAddress("0x0000000000000000000000000000000000000504")
	storageHeader   = common.HexToAddress("0x0000000000000000000000000000000000000505")
)

const chunk = 31

var (
	maxUint128 = new(big.Int).Sub(pow2(128), big.NewInt(1))
	maxSlot    = new(big.Int).Sub(pow2(256), big.NewInt(1))
)

func pow2(n uint) *big.Int { return new(big.Int).Lsh(big.NewInt(1), n) }

// h is a 32-byte big-endian slot number or value.
func h(n int64) common.Hash { return common.BigToHash(big.NewInt(n)) }

// fill is n JUMPDESTs: every chunk present, no PUSHDATA.
func fill(n int) []byte { return bytes.Repeat([]byte{0x5b}, n) }

// straddlingPush has a PUSH32 whose operand crosses the first chunk boundary.
func straddlingPush() []byte {
	code := fill(2 * chunk)
	code[chunk-2] = 0x7f
	for i := chunk - 1; i < chunk+30; i++ {
		code[i] = 0xaa
	}
	return code
}

// pushPastEnd has a PUSH32 as the last opcode: its operand is 31 zero bytes
// running past the code's end, so the second chunk is present with a leading
// count at the cap of 31 and a partial, all-zero slice.
func pushPastEnd() []byte { return append(fill(chunk-1), 0x7f, 0x00, 0x00) }

// smallPushStraddle has a PUSH2 whose second operand byte starts the next
// chunk, and whose operand bytes look like PUSH32 opcodes.
func smallPushStraddle() []byte {
	return slices.Concat(fill(chunk-2), []byte{0x61, 0x7f, 0x7f}, fill(chunk))
}

// zeroChunkCode has an all-zero middle chunk with no PUSHDATA running into
// it, which must be absent while its neighbours are present.
func zeroChunkCode() []byte {
	return slices.Concat(fill(chunk), make([]byte, chunk), fill(chunk))
}

func delegation(target common.Address) []byte {
	return append([]byte{0xef, 0x01, 0x00}, target.Bytes()...)
}

func edgeCaseAlloc() types.GenesisAlloc {
	shared := fill(3 * chunk)
	one := big.NewInt(1)

	return types.GenesisAlloc{
		eoaBalance: {Balance: big.NewInt(1_000_000)},
		eoaNonce:   {Balance: big.NewInt(0), Nonce: 7},
		eoaMaxima:  {Balance: maxUint128, Nonce: ^uint64(0)},
		precompile: {Balance: one},

		codeOneByte:     {Balance: one, Code: fill(1)},
		codeChunkExact:  {Balance: one, Code: fill(chunk)},
		codeChunkPlus:   {Balance: one, Code: fill(chunk + 1)},
		codeStraddle:    {Balance: one, Code: straddlingPush()},
		codePushPastEnd: {Balance: one, Code: pushPastEnd()},
		codeSmallPush:   {Balance: one, Code: smallPushStraddle()},
		codeZeroChunk:   {Balance: one, Code: zeroChunkCode()},
		codeTrailZero:   {Balance: one, Code: append(fill(chunk), 0x00)},
		codeAllZero:     {Balance: one, Code: make([]byte, 2*chunk)},
		codeTwoGroups:   {Balance: one, Code: fill(257*chunk + 1)},
		codeFakeDelega:  {Balance: one, Code: append(delegation(delegateTarget), 0x5b)},
		codeFake23:      {Balance: one, Code: append([]byte{0xef, 0x02, 0x00}, delegateTarget.Bytes()...)},
		codeZeroBalance: {Balance: big.NewInt(0), Code: fill(1)},
		codeWithNonce:   {Balance: one, Nonce: 1, Code: fill(chunk + 1)},

		sharedA: {Balance: one, Code: shared},
		sharedB: {Balance: big.NewInt(2), Code: shared},

		delegatedA: {Balance: one, Code: delegation(delegateTarget)},
		delegatedB: {Balance: one, Code: delegation(delegateTarget)},
		delegatedC: {
			Balance: big.NewInt(3), Nonce: 4, Code: delegation(sharedA),
			Storage: map[common.Hash]common.Hash{h(1): h(0x11), h(300): h(0x12)},
		},
		delegatedToNone: {Balance: one, Code: delegation(common.HexToAddress("0x0999"))},
		delegatedToEOA:  {Balance: one, Code: delegation(eoaBalance)},
		delegatedChain:  {Balance: one, Code: delegation(delegatedA)},
		delegateTarget:  {Balance: one, Code: fill(chunk)},

		storageSpread: {
			Balance: one, Code: fill(chunk),
			Storage: map[common.Hash]common.Hash{
				h(0): h(0xa0), h(63): h(0xa1), h(64): h(0xa2), h(255): h(0xa3),
				h(256): h(0xa4), h(511): h(0xa5), h(512): h(0xa6),
				common.BigToHash(pow2(255)): h(0xa8),
				common.BigToHash(maxSlot):   h(0xa7),
			},
		},
		storageValues: {
			Balance: one, Code: fill(chunk),
			Storage: map[common.Hash]common.Hash{
				h(1): h(1), h(2): common.BigToHash(maxSlot), h(3): h(256),
				h(4): common.BigToHash(pow2(248)), h(5): common.BigToHash(new(big.Int).Lsh(big.NewInt(0xff), 128)),
				h(65): h(0xff),
			},
		},
		storageOnEOA:    {Balance: one, Storage: map[common.Hash]common.Hash{h(0): h(1), h(64): h(2)}},
		storageOverflow: {Balance: one, Storage: map[common.Hash]common.Hash{h(300): h(3)}},
		storageHeader:   {Balance: one, Storage: map[common.Hash]common.Hash{h(7): h(4), h(63): h(5)}},
	}
}

// derivePreimages lays the EIP-8347 preimage file out from the allocation:
// fixed-width records in keccak256(address) order, slots in keccak256(slot)
// order.
func derivePreimages(alloc types.GenesisAlloc) []byte {
	recs := make([]record, 0, len(alloc))
	for addr, acct := range alloc {
		slots := make([]common.Hash, 0, len(acct.Storage))
		for slot := range acct.Storage {
			slots = append(slots, slot)
		}
		recs = append(recs, record{addr: addr, slots: slots})
	}
	return encodePreimages(recs)
}

// checkOrdering fails unless the state can tell hashed-key order from raw
// order, which the ordering cases depend on.
func checkOrdering(alloc types.GenesisAlloc) error {
	addrs := slices.SortedFunc(maps.Keys(alloc), func(a, b common.Address) int { return byKeccak(a[:], b[:]) })
	if slices.IsSortedFunc(addrs, func(a, b common.Address) int { return bytes.Compare(a[:], b[:]) }) {
		return fmt.Errorf("addresses sort the same by raw bytes and by keccak")
	}
	slots := slices.SortedFunc(maps.Keys(alloc[storageSpread].Storage), func(a, b common.Hash) int { return byKeccak(a[:], b[:]) })
	if slices.IsSortedFunc(slots, func(a, b common.Hash) int { return bytes.Compare(a[:], b[:]) }) {
		return fmt.Errorf("storageSpread's slots sort the same numerically and by keccak")
	}
	return nil
}

func genesisJSON(alloc types.GenesisAlloc) ([]byte, error) {
	type genesis struct {
		Config     map[string]any     `json:"config"`
		Nonce      string             `json:"nonce"`
		Timestamp  string             `json:"timestamp"`
		ExtraData  string             `json:"extraData"`
		GasLimit   string             `json:"gasLimit"`
		Difficulty string             `json:"difficulty"`
		MixHash    string             `json:"mixHash"`
		Coinbase   string             `json:"coinbase"`
		Alloc      types.GenesisAlloc `json:"alloc"`
	}
	g := genesis{
		Config: map[string]any{
			"chainId":                 7347,
			"homesteadBlock":          0,
			"eip150Block":             0,
			"eip155Block":             0,
			"eip158Block":             0,
			"byzantiumBlock":          0,
			"constantinopleBlock":     0,
			"petersburgBlock":         0,
			"istanbulBlock":           0,
			"muirGlacierBlock":        0,
			"berlinBlock":             0,
			"londonBlock":             0,
			"arrowGlacierBlock":       0,
			"grayGlacierBlock":        0,
			"mergeNetsplitBlock":      0,
			"shanghaiTime":            0,
			"cancunTime":              0,
			"terminalTotalDifficulty": 0,
			"ethash":                  map[string]any{},
			"blobSchedule": map[string]any{
				"cancun": map[string]any{"target": 3, "max": 6, "baseFeeUpdateFraction": 3338477},
			},
		},
		Nonce:      "0x0",
		Timestamp:  "0x0",
		ExtraData:  "0x7062742d617274696661637473",
		GasLimit:   "0x23f3e20",
		Difficulty: "0x1",
		MixHash:    "0x0000000000000000000000000000000000000000000000000000000000000000",
		Coinbase:   "0x0000000000000000000000000000000000000000",
		Alloc:      alloc,
	}
	return json.MarshalIndent(g, "", "  ")
}

func writeGenesis(path string, alloc types.GenesisAlloc) error {
	if err := checkOrdering(alloc); err != nil {
		return err
	}
	blob, err := genesisJSON(alloc)
	if err != nil {
		return fmt.Errorf("rendering genesis: %w", err)
	}
	return os.WriteFile(path, append(blob, '\n'), 0644)
}
