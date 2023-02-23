package main

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	api "github.com/ethereum/go-ethereum/core/beacon"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
)

// ---------------------------------------------------------------------------//
// loadFixtureTests() yields every test recursively within a fixture.json     //
// file from the given 'root' path. Each test is yieled within a testcase     //
// struct that holds its genesis elements, payloads and post allocation.      //
// This is passed to the func() 'fn' yielded directly within fixtureRunner(), //
// such that workers can start to run the tests against each client.          //
// ---------------------------------------------------------------------------//
func loadFixtureTests(t *hivesim.T, root string, fn func(testcase)) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		// check file is actually a fixture	
		if err != nil {
			t.Logf("unable to walk path: %s", err)
			return err
		}
		if info.IsDir() { 
			return nil
		}
		if fname := info.Name(); !strings.HasSuffix(fname, ".json") {
			return nil
		}

		// extract fixture.json tests (multiple forks) into fixtureTest structs
		var fixtureTests map[string] fixtureTest
		if err := common.LoadJSON(path, &fixtureTests); err != nil {
			t.Logf("invalid test file: %v, unable to load json", err)
			return nil
		}

		// create testcase structure from fixtureTests
		for name, fixture := range fixtureTests {
			network := fixture.json.Network
			// skip networks post merge
			for _, skip := range []string{"Istanbul", "Berlin", "London"} {
				if strings.Contains(network, skip) {
					continue
				}
			}
			// check network is valid 
			if _, exist := envForks[network]; !exist {
				return fmt.Errorf("network `%v` not defined in fork ruleset", network)
			}

			// extract genesis fields from fixture test
			genesis := extractGenesis(fixture.json)

			// extract payloads from each block
			payloads := []*api.ExecutableData{}
			for _, block := range fixture.json.Blocks {
				gblock, _ := block.decodeBlock()
				payload := api.BlockToExecutableData(gblock, common.Big0).ExecutionPayload
				payloads = append(payloads, payload)
			}

			// extract post account information
			postAlloc := fixture.json.Post

			// define testcase struct adding the extracted fields
			tc := testcase {
				fixture: fixture, 
				name: name, 
				filepath: path,
				genesis: genesis,  
				payloads: payloads,
				postAlloc: postAlloc,
			}

			// feed testcase to single worker within fixtureRunner()
			fn(tc)
		}
		return nil

	})
}

// run launches the client and runs the test case against it.
func (tc *testcase) run(t *hivesim.T) {
	start := time.Now()

	t.Log("HIVE LOG --> Setting variables required for starting client.")
	engineAPI := hive_rpc.HiveRPCEngineStarter{
		ClientType: tc.clientType,
		EnginePort: globals.EnginePortHTTP,
		EthPort:    globals.EthPortHTTP,
		JWTSecret:  globals.DefaultJwtTokenSecretBytes,
	}
	ctx := context.Background()
	env := hivesim.Params{
		"HIVE_FORK_DAO_VOTE": "1",
		"HIVE_CHAIN_ID":      "1",
		"HIVE_SKIP_POW":      "1",
		"HIVE_NODETYPE": 	  "full",
	}
	tc.updateEnv(env)
	t0 := time.Now()

	// start client (also creates an engine RPC client internally)
	t.Log("HIVE LOG --> Starting client with Engine API.")
	engineClient, err := engineAPI.StartClient(t, ctx, tc.genesis, env, nil)
	if err != nil {
		t.Fatalf("HIVE FATAL --> Can't start client with Engine API: %v", err)
	}
	t1 := time.Now()

	// send payloads
	for blockNumber, payload :=  range tc.payloads {
		_, err := engineClient.NewPayloadV2(context.Background(), payload)
		if err != nil {
			t.Fatalf("HIVE FATAL ---> Unable to send payload[%v] in test %s: %v ", blockNumber, tc.name, err)
		}
	}
	t2 := time.Now()

	// update head of "beacon" chain with the latest valid response
	latestBlockHash := *engineClient.LatestNewPayloadResponse().LatestValidHash
	fcState := &api.ForkchoiceStateV1{HeadBlockHash: latestBlockHash}
	engineClient.ForkchoiceUpdatedV2(ctx, fcState, nil)
	t3 := time.Now()

	// check nonce, balance & storage of accounts in final block against fixture values
	latestBlockNumber := new(big.Int).SetUint64(engineClient.LatestNewPayloadSent().Number)
	for account, genesisAccount :=  range tc.postAlloc {

		// get nonce & balance from last block (end of test execution)
		gotNonce, errN := engineClient.NonceAt(ctx, account, latestBlockNumber)
		gotBalance, errB := engineClient.BalanceAt(ctx, account, latestBlockNumber)
		if errN != nil {
			t.Errorf("HIVE ERROR ---> Unable to call nonce from account: %v, in test %s: %v", account, tc.name, errN)
		} else if errB != nil {
			t.Errorf("HIVE ERROR ---> Unable to call balance from account: %v, in test %s: %v", account, tc.name, errB)
		} 

		// check final nonce & balance matches expected in fixture 
		if genesisAccount.Nonce != gotNonce {
			t.Errorf(`HIVE ERROR ---> Nonce recieved from account %v doesn't match expected from fixture in test %s:
			Recieved from block: %v
			Expected in fixture: %v`, account, tc.name, gotNonce, genesisAccount.Nonce)
		}
		if common.BigToHash(genesisAccount.Balance) != common.BigToHash(gotBalance) {
			t.Errorf(`HIVE ERROR ---> Balance recieved from account %v doesn't match expected from fixture in test %s:
			Recieved from block: %v
			Expected in fixture: %v`, account, tc.name, gotBalance, genesisAccount.Balance)
		}
	
		// check values in storage match those in fixture
		for stPosition, stValue := range genesisAccount.Storage {
			// get value in storage position stPosition from last block
			gotStorage, errS := engineClient.StorageAt(ctx, account, common.BytesToHash(stPosition.Bytes()), latestBlockNumber)
			if errS != nil {
				t.Errorf("HIVE ERROR ---> Unable to call storage bytes from account address %v, storage position %v, in test %s: %v", account, stPosition, tc.name, errS)
			}
			// check recieved value against fixture value
			if stValue != common.BytesToHash(gotStorage) {
				t.Errorf(`HIVE ERROR ---> Storage recieved from account %v doesn't match expected from fixture in test %s:
				From storage address: %v
				Recieved from block: %v
				Expected in fixture: %v`, account, tc.name, stPosition, common.BytesToHash(gotStorage), stValue)
			}
		}
	}
	end := time.Now()

	t.Logf(`HIVE END ---> Test Timing:
			setupClientEnv %v
 			startClient %v
 			sendAllPayloads %v
 			setNewHeadOfChain %v
			checkStorageValues %v
			totalTestTime %v`, t0.Sub(start), t1.Sub(t0), t2.Sub(t1), t3.Sub(t2), end.Sub(t3), end.Sub(start))
}

// updateEnv sets environment variables from the test
func (tc *testcase) updateEnv(env hivesim.Params) {
	// environment variables for fork ruleset
	forkRules := envForks[tc.fixture.json.Network]
	for k, v := range forkRules {
		env[k] = fmt.Sprintf("%d", v)
	}
}