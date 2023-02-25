package main

import (
	"context"
	"fmt"
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

// ------------------------------------------------------------------------------- //
// loadFixtureTests() extracts tests from fixture.json files in a given directory, //
// creates a testcase struct for each test, and passes the testcase struct to a    //
// func() parameter `fn`, which is used within fixtureRunner() to run the tests.   //
// ------------------------------------------------------------------------------- //
func loadFixtureTests(t *hivesim.T, root string, fn func(testcase)) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		// check file is actually a fixture
		if err != nil {
			t.Logf("unable to walk path: %s", err)
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), "withdrawals_use_value_in_tx.json") {
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

			if !strings.HasSuffix(name, "000_shanghai") {
				continue
			}

			// skip networks post merge or not supported
			network := fixture.json.Network
			for _, skip := range []string{"Istanbul", "Berlin", "London"} {
				if strings.Contains(network, skip) || envForks[network] == nil {
					t.Logf("skipping test for network %s", network)
					continue
				}
			}

			// define testcase (tc) struct with initial fields
			tc := testcase{
				fixture:  fixture,
				name:     name,
				filepath: path,
			}

			// extract genesis, payloads & post allocation field to tc
			tc.extractFixtureFields(fixture.json)

			// feed tc to single worker within fixtureRunner()
			fn(tc)
		}
		return nil
	})
}

// ----------------------------------------------------------------------------------------------------//
// run() executes a testcase against the client, called within a test channel from fixtureRunner().    //
// All testcase payloads are sent and executed using the EngineAPI. For verification all fixture       //
// nonce, balance and storage values are checked against the response recieved from the lastest block. //
// ----------------------------------------------------------------------------------------------------//
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
		"HIVE_NODETYPE":      "full",
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

	// send payloads and check response
	latestValidHash := common.Hash{}
	for blockNumber, payload := range tc.payloads {
		plException := tc.fixture.json.Blocks[blockNumber].Exception
		expectedStatus := "VALID"
		if plException != "" {
			expectedStatus = "INVALID"
		}
		// execute fixture block payload
		plStatus, plErr := engineClient.NewPayloadV2(context.Background(), payload)
		if plErr != nil {
			t.Fatalf("HIVE FATAL ---> Unable to send payload %v in test %s: %v ", blockNumber, tc.name, plErr)
		}
		// update latest valid block hash
		if plStatus.Status == "VALID" {
			latestValidHash = *plStatus.LatestValidHash
		}
		// check payload status is expected from fixture
		if expectedStatus != plStatus.Status {
			t.Errorf(`HIVE ERROR ---> Payload status mismatch for block %v in test %s. 
				Expected from fixture: %s. 	Got from payload: %s.`, blockNumber, tc.name, expectedStatus, plStatus.Status)
		}
		// check error message if invalid
		if expectedStatus == "INVALID" && *plStatus.ValidationError != plException {
			t.Errorf(`HIVE ERROR ---> Error message mismatch for block %v in test %s. Payload status is expected to be INVALID. 
				Expected message from fixture: %s. Got message from payload: %s.`, blockNumber, tc.name, plException, *plStatus.ValidationError)
		}

	}
	t2 := time.Now()

	// only update head of beacon chain if valid response occurred
	if latestValidHash != (common.Hash{}) {
		// update with latest valid response
		fcState := &api.ForkchoiceStateV1{HeadBlockHash: latestValidHash}
		engineClient.ForkchoiceUpdatedV2(ctx, fcState, nil)
	}
	t3 := time.Now()

	// check nonce, balance & storage of accounts in final block against fixture values
	for account, genesisAccount := range *tc.postAlloc {

		// get nonce & balance from last block (end of test execution)
		gotNonce, errN := engineClient.NonceAt(ctx, account, nil)
		gotBalance, errB := engineClient.BalanceAt(ctx, account, nil)
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
		if genesisAccount.Balance.Cmp(gotBalance) != 0 {
			// if common.BigToHash(genesisAccount.Balance) != common.BigToHash(gotBalance) {
			t.Errorf(`HIVE ERROR ---> Balance recieved from account %v doesn't match expected from fixture in test %s:
			Recieved from block: %v
			Expected in fixture: %v`, account, tc.name, gotBalance, genesisAccount.Balance)
		}

		// check values in storage match those in fixture
		for stPosition, stValue := range genesisAccount.Storage {
			// get value in storage position stPosition from last block
			gotStorage, errS := engineClient.StorageAt(ctx, account, stPosition, nil)
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

// ----------------------------------------------------------------------- //
// updateEnv() updates the environment variables against the fork rules    //
// defined in envForks, for the network specified in the testcase fixture. //
// ----------------------------------------------------------------------- //
func (tc *testcase) updateEnv(env hivesim.Params) {
	forkRules := envForks[tc.fixture.json.Network]
	for k, v := range forkRules {
		env[k] = fmt.Sprintf("%d", v)
	}
}
