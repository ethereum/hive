package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
)

// loadFixtureTests extracts tests from fixture.json files in a given directory,
// creates a testcase for each test, and passes the testcase struct to fn.
func loadFixtureTests(t *hivesim.T, root string, fn func(testcase)) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		// check file is actually a fixture
		if err != nil {
			t.Logf("unable to walk path: %s", err)
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".json") {
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
			// extract genesis, payloads & post allocation field to tc
			tc.extractFixtureFields(fixture.json)
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
		"HIVE_SKIP_POW":      "1",
		"HIVE_NODETYPE":      "full",
	}
	tc.updateEnv(env)
	t0 := time.Now()

	// start client (also creates an engine RPC client internally)
	t.Log("starting client with Engine API.")
	engineClient, err := engineStarter.StartClient(t, ctx, tc.genesis, env, nil)
	if err != nil {
		tc.failedErr = err
		t.Fatalf("can't start client with Engine API: %v", err)
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
			tc.failedErr = plErr
			t.Fatalf("unable to send block %v in test %s: %v ", blockNumber+1, tc.name, plErr)
		}
		// update latest valid block hash
		if plStatus.Status == "VALID" {
			latestValidHash = *plStatus.LatestValidHash
		}
		// check payload status is expected from fixture
		if expectedStatus != plStatus.Status {
			tc.failedErr = errors.New("payload status mismatch")
			t.Fatalf(`payload status mismatch for block %v in test %s.
				expected from fixture: %s
				got from payload: %s`, blockNumber+1, tc.name, expectedStatus, plStatus.Status)
		}
	}
	t2 := time.Now()

	// only update head of beacon chain if valid response occurred
	if latestValidHash != (common.Hash{}) {
		// update with latest valid response
		fcState := &api.ForkchoiceStateV1{HeadBlockHash: latestValidHash}
		if _, fcErr := engineClient.ForkchoiceUpdatedV2(ctx, fcState, nil); fcErr != nil {
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
