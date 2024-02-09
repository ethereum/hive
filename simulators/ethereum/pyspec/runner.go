package main

import (
	"context"
	"fmt"
	"io/fs"
	"math/big"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
)

// loadFixtureTests extracts tests from fixture.json files in a given directory,
// creates a testcase for each test, and passes the testcase struct to fn.
func loadFixtureTests(t *hivesim.T, root string, re *regexp.Regexp, fn func(TestCase)) {
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

		// extract fixture.json tests (multiple forks) into fixture structs
		var fixtureTests map[string]*Fixture
		if err := common.LoadJSON(path, &fixtureTests); err != nil {
			t.Logf("invalid test file: %v, unable to load json", err)
			return nil
		}

		// create testcase structure from fixtureTests
		for name, fixture := range fixtureTests {
			// skip networks post merge or not supported
			network := fixture.Fork
			if _, exist := envForks[network]; !exist {
				continue
			}
			// define testcase (tc) struct with initial fields
			tc := TestCase{
				Name:     path[10:len(path)-5] + "/" + name,
				FilePath: path,
				Fixture:  fixture,
			}
			// match test case name against regex if provided
			if !re.MatchString(tc.Name) {
				continue
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
// response received from the lastest block.
func (tc *TestCase) run(t *hivesim.T) {
	start := time.Now()
	tc.FailCallback = t

	t.Log("setting variables required for starting client.")
	engineStarter := hive_rpc.HiveRPCEngineStarter{
		ClientType: tc.ClientType,
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
	if tc.FailedErr != nil {
		t.Fatalf("test failed early: %v", tc.FailedErr)
	}
	// start client (also creates an engine RPC client internally)
	t.Log("starting client with Engine API.")
	engineClient, err := engineStarter.StartClient(t, ctx, tc.Genesis(), env, nil)
	if err != nil {
		tc.Fatalf("can't start client with Engine API: %v", err)
	}
	// verify genesis hash matches that of the fixture
	genesisBlock, err := engineClient.BlockByNumber(ctx, big.NewInt(0))
	if err != nil {
		tc.Fatalf("unable to get genesis block: %v", err)
	}
	if genesisBlock.Hash() != tc.GenesisBlock.Hash {
		tc.Fatalf("genesis hash mismatch")
	}
	t1 := time.Now()

	// send payloads and check response
	var latestValidPayload *EngineNewPayload
	for _, engineNewPayload := range tc.EngineNewPayloads {
		engineNewPayload := engineNewPayload
		if syncing, err := engineNewPayload.ExecuteValidate(
			ctx,
			engineClient,
		); err != nil {
			tc.Fatalf("Payload validation error: %v", err)
		} else if syncing {
			tc.Fatalf("Payload validation failed (not synced)")
		}
		// update latest valid block hash if payload status is VALID
		if engineNewPayload.Valid() {
			latestValidPayload = engineNewPayload
		}
	}
	t2 := time.Now()

	// only update head of beacon chain if valid response occurred
	if latestValidPayload != nil {
		if syncing, err := latestValidPayload.ForkchoiceValidate(ctx, engineClient, tc.EngineFcuVersion); err != nil {
			tc.Fatalf("unable to update head of chain: %v", err)
		} else if syncing {
			tc.Fatalf("forkchoice update failed (not synced)")
		}
	}
	t3 := time.Now()
	if err := tc.ValidatePost(ctx, engineClient); err != nil {
		tc.Fatalf("unable to verify post allocation in test %s: %v", tc.Name, err)
	}

	if tc.SyncPayload != nil {
		// First send a new payload to the already running client
		if syncing, err := tc.SyncPayload.ExecuteValidate(
			ctx,
			engineClient,
		); err != nil {
			tc.Fatalf("unable to send sync payload: %v", err)
		} else if syncing {
			tc.Fatalf("sync payload failed (not synced)")
		}
		// Send a forkchoice update to the already running client to head to the sync payload
		if syncing, err := tc.SyncPayload.ForkchoiceValidate(ctx, engineClient, tc.EngineFcuVersion); err != nil {
			tc.Fatalf("unable to update head of chain: %v", err)
		} else if syncing {
			tc.Fatalf("forkchoice update failed (not synced)")
		}

		// Spawn a second client connected to the already running client,
		// send the forkchoice updated with the head hash and wait for sync.
		// Then verify the post allocation.
		// Add a timeout too.
		secondEngineClient, err := engineStarter.StartClient(t, ctx, tc.Genesis(), env, nil, engineClient)
		if err != nil {
			tc.Fatalf("can't start client with Engine API: %v", err)
		}

		if _, err := tc.SyncPayload.ExecuteValidate(
			ctx,
			secondEngineClient,
		); err != nil {
			tc.Fatalf("unable to send sync payload: %v", err)
		} // Don't check syncing here because some clients do sync immediately

		timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		for {
			if syncing, err := tc.SyncPayload.ForkchoiceValidate(ctx, secondEngineClient, tc.EngineFcuVersion); err != nil {
				tc.Fatalf("unable to update head of chain: %v", err)
			} else if !syncing {
				break
			}
			select {
			case <-timeoutCtx.Done():
				tc.Fatalf("timeout waiting for sync of secondary client")
			default:
			}
			time.Sleep(time.Second)
		}
	}

	end := time.Now()

	if tc.FailedErr == nil {
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
func (tc *TestCase) updateEnv(env hivesim.Params) {
	forkRules := envForks[tc.Fork]
	for k, v := range forkRules {
		env[k] = fmt.Sprintf("%d", v)
	}
}
