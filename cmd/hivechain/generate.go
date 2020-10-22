package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

func init() {
	// If any of these accounts have balance in the genesis block, it will be spent on random transactions.
	addr1 := ethcommon.HexToAddress("0x71562b71999873DB5b286dF957af199Ec94617F7")
	addr2 := ethcommon.HexToAddress("0x703c4b2bD70c169f5717101CaeE543299Fc946C7")
	addr3 := ethcommon.HexToAddress("0x0D3ab14BBaD3D99F4203bd7a11aCB94882050E7e")
	key1, _ := crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	key2, _ := crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
	key3, _ := crypto.HexToECDSA("49a7b37aa6f6645917e7b807e9d1c00d4fa71f18343b0d4122a4d2df64dd6fee")
	knownAccounts[addr1] = key1
	knownAccounts[addr2] = key2
	knownAccounts[addr3] = key3
}

var (
	txInterval    = 10 // tx will be added every interval block
	knownAccounts = make(map[ethcommon.Address]*ecdsa.PrivateKey)
	genstorage    = hexutil.MustDecode("0x60015b8080556001015a6161a81063000000025700")
	genlogs       = hexutil.MustDecode("0x60004360005260006020525b63000000176300000029565b60206020a15a61271010630000000b57005b60205160010160205260406000209056")
	gencode       = hexutil.MustDecode("0x630000001960010138038063000000196001016000396000f35b")
)

// loadGenesis loads genesis.json.
func loadGenesis(file string) (*core.Genesis, error) {
	var gspec core.Genesis
	err := ethcommon.LoadJSON(file, &gspec)
	return &gspec, err
}

const (
	txTypeValue = iota
	txTypeStorage
	txTypeLogs
	txTypeCode
	txTypeMax
)

// addTxForKnownAccounts adds a transaction to the generated chain if the genesis block
// contains certain known accounts.
func addTxForKnownAccounts(genesis *core.Genesis, i int, gen *core.BlockGen) {
	if i%txInterval != 0 {
		return
	}
	txType := (i / txInterval) % txTypeMax
	for addr, key := range knownAccounts {
		if a, ok := genesis.Alloc[addr]; ok {
			// It exists, check remaining balance. Would be nice if BlockGen had a way to
			// check balance, but it doesn't, so we need to estimate the remaining
			// balance.
			txCost := big.NewInt(1)
			spent := new(big.Int).Mul(txCost, big.NewInt(int64(i)))
			gbal := new(big.Int).Set(a.Balance)
			if gbal.Sub(gbal, spent).Cmp(txCost) < 0 {
				continue // no funds left in this account
			}
			// Add transaction.
			tx := generateTx(txType, key, genesis, gen)
			log.Printf("adding tx (type %d) from %s in block %d", txType, addr.String(), gen.Number())
			log.Printf("0x%x (%d gas)", tx.Hash(), tx.Gas())
			gen.AddTx(tx)
			return
		}
	}
}

// generateTx creates a random transaction signed by the given account.
func generateTx(txType int, key *ecdsa.PrivateKey, genesis *core.Genesis, gen *core.BlockGen) *types.Transaction {
	var (
		src      = crypto.PubkeyToAddress(key.PublicKey)
		gasprice = big.NewInt(0)
		tx       *types.Transaction
	)
	// Generate according to type.
	switch txType {
	case txTypeValue:
		amount := big.NewInt(1)
		var dst common.Address
		rand.Read(dst[:])
		tx = types.NewTransaction(gen.TxNonce(src), dst, amount, params.TxGas, gasprice, nil)
	case txTypeStorage:
		gas := createTxGasLimit(genstorage, 80000)
		tx = types.NewContractCreation(gen.TxNonce(src), new(big.Int), gas, gasprice, genstorage)
	case txTypeLogs:
		gas := createTxGasLimit(genlogs, 20000)
		tx = types.NewContractCreation(gen.TxNonce(src), new(big.Int), gas, gasprice, genlogs)
	case txTypeCode:
		// The code generator contract deploys any data given after its own bytecode.
		codesize := 128
		input := make([]byte, len(gencode)+codesize)
		copy(input, gencode)
		rand.Read(input[len(gencode):])
		gas := createTxGasLimit(gencode, 10000+params.CreateDataGas*uint64(codesize))
		tx = types.NewContractCreation(gen.TxNonce(src), new(big.Int), gas, gasprice, input)
	}
	// Sign the transaction.
	signer := types.MakeSigner(genesis.Config, gen.Number())
	signedTx, err := types.SignTx(tx, signer, key)
	if err != nil {
		panic(err)
	}
	return signedTx
}

func createTxGasLimit(data []byte, extra uint64) uint64 {
	igas, _ := core.IntrinsicGas(data, true, true)
	return igas + extra
}

// produceTestChainFromGenesisFile creates a test chain with no transactions or other
// modifications based on an externally specified genesis file. The blockTimeInSeconds is
// used to manipulate the block difficulty.
func produceTestChainFromGenesisFile(genesis string, outputPath string, blockCount uint, blockTimeInSeconds uint) error {
	gspec, err := loadGenesis(genesis)
	if err != nil {
		return err
	}
	blockModifier := func(i int, gen *core.BlockGen) {
		log.Println("generating block", i)
		gen.OffsetTime(int64((i+1)*int(blockTimeInSeconds) - 10))
		addTxForKnownAccounts(gspec, i, gen)
	}
	return generateChainAndSave(gspec, blockCount, outputPath, blockModifier)
}

// writeChain exports the given chain to a file.
func writeChain(chain *core.BlockChain, filename string, start uint64) error {
	out, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()
	return chain.ExportN(out, start, chain.CurrentBlock().NumberU64())
}

// GenerateChainAndSave produces a chain based on the genesis-config with
// valid proof of work, and block difficulty modified by the OffsetTime function
// in the per-block post-processing function.
func generateChainAndSave(gspec *core.Genesis, blockCount uint, path string, blockModifier func(i int, gen *core.BlockGen)) error {
	db := rawdb.NewMemoryDatabase()
	genesis := gspec.MustCommit(db)
	config := ethash.Config{
		PowMode:        ethash.ModeNormal,
		CachesInMem:    2,
		DatasetsOnDisk: 2,
		DatasetDir:     ethashDir(),
	}
	engine := ethash.New(config, nil, false)
	insta := instaSeal{engine}

	// Generate a chain where each block is created, modified, and immediately sealed.
	chain, _ := core.GenerateChain(gspec.Config, genesis, insta, db, int(blockCount), blockModifier)

	// Import the chain. This runs all block validation rules.
	blockchain, err := core.NewBlockChain(db, nil, gspec.Config, engine, vm.Config{}, nil)
	if err != nil {
		return fmt.Errorf("can't create blockchain: %v", err)
	}
	defer blockchain.Stop()
	if _, err := blockchain.InsertChain(chain); err != nil {
		return fmt.Errorf("chain validation error: %v", err)
	}
	headstate, _ := blockchain.State()
	dump := headstate.Dump(false, false, false)

	// Write out the generated blockchain
	if err := writeChain(blockchain, filepath.Join(path, "chain.rlp"), 1); err != nil {
		return err
	}
	if err := writeChain(blockchain, filepath.Join(path, "chain_genesis.rlp"), 0); err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(path, "chain_poststate.json"), dump, 0644); err != nil {
		return err
	}
	return nil
}

func ethashDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ethash")
}

// instaSeal wraps a consensus engine with instant block sealing. When a block is produced
// using FinalizeAndAssemble, it also applies Seal.
type instaSeal struct{ consensus.Engine }

// FinalizeAndAssemble implements consensus.Engine, accumulating the block and uncle rewards,
// setting the final state and assembling the block.
func (e instaSeal) FinalizeAndAssemble(chain consensus.ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {
	block, err := e.Engine.FinalizeAndAssemble(chain, header, state, txs, uncles, receipts)
	if err != nil {
		return nil, err
	}
	sealedBlock := make(chan *types.Block, 1)
	if err = e.Engine.Seal(nil, block, sealedBlock, nil); err != nil {
		return nil, err
	}
	return <-sealedBlock, nil
}
