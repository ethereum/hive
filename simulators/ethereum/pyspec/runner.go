package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/tests"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
)

// loadFixtureTests extracts tests from fixture.json files in a given directory,
// creates a testcase for each test, and passes the testcase struct to fn.
func loadFixtureTests(t *hivesim.T, root string, re *regexp.Regexp, fn func(testcase)) {
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		// check file is actually a fixture
		if err != nil {
			t.Logf("unable to walk path: %s", err)
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		excludePaths := []string{"example/"} // modify for tests to exclude
		if strings.Contains(path, strings.Join(excludePaths, "")) {
			return nil
		}

		// extract fixture.json tests (multiple forks) into fixtureTest structs
		var fixtureTests map[string]fixtureTest
		if err := common.LoadJSON(path, &fixtureTests); err != nil {
			t.Logf("invalid test file: %v, unable to load json", err)
			return nil
		}

		// create testcase structure from fixtureTests
		for name, fixture := range fixtureTests {
			// skip networks post merge or not supported
			network := fixture.json.Network
			if _, exist := envForks[network]; !exist {
				continue
			}
			// define testcase (tc) struct with initial fields
			tc := testcase{
				fixture:  fixture,
				name:     path[10:len(path)-5] + "/" + name,
				filepath: path,
			}
			// match test case name against regex if provided
			if !re.MatchString(tc.name) {
				continue
			}
			// extract genesis, payloads & post allocation field to tc
			if err := tc.extractFixtureFields(fixture.json); err != nil {
				t.Logf("test %v / %v: unable to extract fixture fields: %v", d.Name(), name, err)
				tc.failedErr = fmt.Errorf("unable to extract fixture fields: %v", err)
			}
			// feed tc to single worker within fixtureRunner()
			fn(tc)
		}
		return nil
	})
}

// run executes a testcase against the client, called within a test channel from
// fixtureRunner, all testcase payloads are sent and executed using the EngineAPI. for
// verification all fixture nonce, balance and storage values are checked against the
// response recieved from the lastest block.
func (tc *testcase) run(t *hivesim.T) {
	start := time.Now()

	t.Log("setting variables required for starting client.")
	engineStarter := hive_rpc.HiveRPCEngineStarter{
		ClientType: tc.clientType,
		EnginePort: globals.EnginePortHTTP,
		EthPort:    globals.EthPortHTTP,
		JWTSecret:  globals.DefaultJwtTokenSecretBytes,
	}
	ctx := context.Background()
	env := hivesim.Params{
		"HIVE_FORK_DAO_VOTE": "1",
		"HIVE_CHAIN_ID":      "1",
		"HIVE_NODETYPE":      "full",
	}
	tc.updateEnv(env)
	t0 := time.Now()
	// If test is already failed, don't bother spinning up a client
	if tc.failedErr != nil {
		t.Errorf("test failed early: %v", tc.failedErr)
		return
	}
	// start client (also creates an engine RPC client internally)
	t.Log("starting client with Engine API.")
	engineClient, err := engineStarter.StartClient(t, ctx, tc.genesis, env, nil)
	if err != nil {
		tc.failedErr = err
		t.Fatalf("can't start client with Engine API: %v", err)
	}
	// verify genesis hash matches that of the fixture
	genesisBlock, err := engineClient.BlockByNumber(ctx, big.NewInt(0))
	if err != nil {
		tc.failedErr = err
		t.Fatalf("unable to get genesis block: %v", err)
	}
	if genesisBlock.Hash() != tc.fixture.json.Genesis.Hash {
		tc.failedErr = errors.New("genesis hash mismatch")
		t.Fatalf("genesis hash mismatch")
	}
	t1 := time.Now()

	// send payloads and check response
	latestValidHash := common.Hash{}
	latestVersion := uint64(1)
	for i, engineNewPayload := range tc.payloads {
		// execute fixture block payload
		ed, err := engineNewPayload.ToExecutableData()
		if err != nil {
			tc.failedErr = err
			t.Fatalf("unable to convert engineNewPayload to executableData: %v", err)
		}
		plStatus, plErr := engineClient.NewPayload(
			context.Background(),
			int(engineNewPayload.Version),
			ed,
		)
		latestVersion = engineNewPayload.Version
		// check for rpc errors and compare error codes
		fxErrCode := int(tc.fixture.json.Blocks[i].EngineNewPayload.ErrorCode)
		if fxErrCode != 0 {
			checkRPCErrors(plErr, fxErrCode, t, tc)
			continue
		}
		// set expected payload return status
		expectedStatus := "VALID"
		if tc.fixture.json.Blocks[i].Exception != "" {
			expectedStatus = "INVALID"
		}
		// check payload status matches expected
		if plStatus.Status != expectedStatus {
			tc.failedErr = fmt.Errorf("payload status mismatch: client returned %v and fixture expected %v", plStatus.Status, expectedStatus)
			t.Fatalf("payload status mismatch: client returned %v\n fixture expected %v\n", plStatus.Status, expectedStatus, tc.name)
		}
		// update latest valid block hash if payload status is VALID
		if plStatus.Status == "VALID" {
			latestValidHash = *plStatus.LatestValidHash
		}
	}
	t2 := time.Now()

	// only update head of beacon chain if valid response occurred
	if latestValidHash != (common.Hash{}) {
		// update with latest valid response
		fcState := &api.ForkchoiceStateV1{HeadBlockHash: latestValidHash}
		// TODO: This is incorrect, up to this point, the `engine_forkchoiceUpdated` and `engine_newPayload` versions for each
		// fork match, but it could change in the future. Ideally we should embed the version in the fixture.
		if _, fcErr := engineClient.ForkchoiceUpdated(ctx, int(latestVersion), fcState, nil); fcErr != nil {
			tc.failedErr = fcErr
			t.Fatalf("unable to update head of beacon chain in test %s: %v ", tc.name, fcErr)
		}
	}
	t3 := time.Now()

	// check nonce, balance & storage of accounts in final block against fixture values
	for account, genesisAccount := range *tc.postAlloc {
		// get nonce & balance from last block (end of test execution)
		gotNonce, errN := engineClient.NonceAt(ctx, account, nil)
		gotBalance, errB := engineClient.BalanceAt(ctx, account, nil)
		if errN != nil {
			tc.failedErr = errN
			t.Errorf("unable to call nonce from account: %v, in test %s: %v", account, tc.name, errN)
		} else if errB != nil {
			tc.failedErr = errB
			t.Errorf("unable to call balance from account: %v, in test %s: %v", account, tc.name, errB)
		}
		// check final nonce & balance matches expected in fixture
		if genesisAccount.Nonce != gotNonce {
			tc.failedErr = errors.New("nonce recieved doesn't match expected from fixture")
			t.Errorf(`nonce recieved from account %v doesn't match expected from fixture in test %s:
			recieved from block: %v
			expected in fixture: %v`, account, tc.name, gotNonce, genesisAccount.Nonce)
		}
		if genesisAccount.Balance.Cmp(gotBalance) != 0 {
			tc.failedErr = errors.New("balance recieved doesn't match expected from fixture")
			t.Errorf(`balance recieved from account %v doesn't match expected from fixture in test %s:
			recieved from block: %v
			expected in fixture: %v`, account, tc.name, gotBalance, genesisAccount.Balance)
		}
		// check final storage
		if len(genesisAccount.Storage) > 0 {
			// extract fixture storage keys
			keys := make([]common.Hash, 0, len(genesisAccount.Storage))
			for key := range genesisAccount.Storage {
				keys = append(keys, key)
			}
			// get storage values for account with keys: keys
			gotStorage, errS := engineClient.StorageAtKeys(ctx, account, keys, nil)
			if errS != nil {
				tc.failedErr = errS
				t.Errorf("unable to get storage values from account: %v, in test %s: %v", account, tc.name, errS)
			}
			// check values in storage match with fixture
			for _, key := range keys {
				if genesisAccount.Storage[key] != *gotStorage[key] {
					tc.failedErr = errors.New("storage recieved doesn't match expected from fixture")
					t.Errorf(`storage recieved from account %v doesn't match expected from fixture in test %s:
						from storage address: %v
						recieved from block:  %v
						expected in fixture:  %v`, account, tc.name, key, gotStorage[key], genesisAccount.Storage[key])
				}
			}
		}
	}
	end := time.Now()

	if tc.failedErr == nil {
		t.Logf(`test timing:
			setupClientEnv %v
 			startClient %v
 			sendAllPayloads %v
 			setNewHeadOfChain %v
			checkStorageValues %v
			totalTestTime %v`, t0.Sub(start), t1.Sub(t0), t2.Sub(t1), t3.Sub(t2), end.Sub(t3), end.Sub(start))

	}
}

// updateEnv updates the environment variables against the fork rules
// defined in envForks, for the network specified in the testcase fixture.
func (tc *testcase) updateEnv(env hivesim.Params) {
	forkRules := envForks[tc.fixture.json.Network]
	for k, v := range forkRules {
		env[k] = fmt.Sprintf("%d", v)
	}
}

// extractFixtureFields extracts the genesis, post allocation and payload
// fields from the given fixture test and stores them in the testcase struct.
func (tc *testcase) extractFixtureFields(fixture fixtureJSON) error {
	var err error
	tc.genesis, err = extractGenesis(fixture)
	if err != nil {
		return err
	}
	tc.postAlloc = &fixture.Post
	var engineNewPayloads []*engineNewPayload
	for _, bl := range fixture.Blocks {
		if bl.EngineNewPayload == nil {
			return errors.New("engineNewPayload is nil")
		}
		engineNewPayloads = append(engineNewPayloads, bl.EngineNewPayload)
	}
	tc.payloads = engineNewPayloads
	return nil
}

// extractGenesis extracts the genesis block information from the given fixture
// and returns a core.Genesis struct containing the extracted information.
func extractGenesis(fixture fixtureJSON) (*core.Genesis, error) {
	if fixture.Genesis.BeaconRoot != nil {
		emptyHash := common.Hash{}
		if !bytes.Equal(fixture.Genesis.BeaconRoot[:], emptyHash[:]) {
			return nil, errors.New("beacon root in genesis is not empty")
		}
	}
	genesis := &core.Genesis{
		Config:        tests.Forks[fixture.Network],
		Coinbase:      fixture.Genesis.Coinbase,
		Difficulty:    fixture.Genesis.Difficulty,
		GasLimit:      fixture.Genesis.GasLimit,
		Timestamp:     fixture.Genesis.Timestamp.Uint64(),
		ExtraData:     fixture.Genesis.ExtraData,
		Mixhash:       fixture.Genesis.MixHash,
		Nonce:         fixture.Genesis.Nonce.Uint64(),
		BaseFee:       fixture.Genesis.BaseFee,
		BlobGasUsed:   fixture.Genesis.BlobGasUsed,
		ExcessBlobGas: fixture.Genesis.ExcessBlobGas,
		Alloc:         fixture.Pre,
	}
	return genesis, nil
}

// checkRPCErrors checks for RPC errors and compares error codes if expected.
func checkRPCErrors(plErr error, fxErrCode int, t *hivesim.T, tc *testcase) {
	rpcErr, isRpcErr := plErr.(rpc.Error)
	if isRpcErr {
		plErrCode := rpcErr.ErrorCode()
		if plErrCode != fxErrCode {
			tc.failedErr = fmt.Errorf("error code mismatch: client returned %v and fixture expected %v", plErrCode, fxErrCode)
			t.Fatalf("error code mismatch\n client returned: %v\n fixture expected: %v\n in test %s", plErrCode, fxErrCode, tc.name)
		}
		t.Logf("expected error code caught by client: %v", plErrCode)
	} else {
		tc.failedErr = fmt.Errorf("fixture expected rpc error code: %v but none was returned from client", fxErrCode)
		t.Fatalf("fixture expected rpc error code: %v but none was returned from client in test %s", fxErrCode, tc.name)
	}
}
