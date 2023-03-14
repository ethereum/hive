package suite_auth

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

// JWT Authentication Tests
type AuthTestSpec struct {
	Name                  string
	TimeDriftSeconds      int64
	CustomAuthSecretBytes []byte
	AuthOk                bool
	RetryAttempts         int64
}

var authTestSpecs = []AuthTestSpec{
	{
		Name:                  "JWT Authentication: No time drift, correct secret",
		TimeDriftSeconds:      0,
		CustomAuthSecretBytes: nil,
		AuthOk:                true,
	},
	{
		Name:                  "JWT Authentication: No time drift, incorrect secret (shorter)",
		TimeDriftSeconds:      0,
		CustomAuthSecretBytes: []byte("secretsecretsecretsecretsecrets"),
		AuthOk:                false,
	},
	{
		Name:                  "JWT Authentication: No time drift, incorrect secret (longer)",
		TimeDriftSeconds:      0,
		CustomAuthSecretBytes: append([]byte{0}, []byte("secretsecretsecretsecretsecretse")...),
		AuthOk:                false,
	},
	{
		Name:                  "JWT Authentication: Negative time drift, exceeding limit, correct secret",
		TimeDriftSeconds:      -1 - globals.MaxTimeDriftSeconds,
		CustomAuthSecretBytes: nil,
		AuthOk:                false,
		RetryAttempts:         5,
	},
	{
		Name:                  "JWT Authentication: Negative time drift, within limit, correct secret",
		TimeDriftSeconds:      1 - globals.MaxTimeDriftSeconds,
		CustomAuthSecretBytes: nil,
		AuthOk:                true,
		RetryAttempts:         5,
	},
	{
		Name:                  "JWT Authentication: Positive time drift, exceeding limit, correct secret",
		TimeDriftSeconds:      globals.MaxTimeDriftSeconds + 1,
		CustomAuthSecretBytes: nil,
		AuthOk:                false,
		RetryAttempts:         5,
	},
	{
		Name:                  "JWT Authentication: Positive time drift, within limit, correct secret",
		TimeDriftSeconds:      globals.MaxTimeDriftSeconds - 1,
		CustomAuthSecretBytes: nil,
		AuthOk:                true,
		RetryAttempts:         5,
	},
}

var Tests = func() []test.Spec {
	testSpecs := make([]test.Spec, 0)
	for _, authTest := range authTestSpecs {
		testSpecs = append(testSpecs, GenerateAuthTestSpec(authTest))
	}
	return testSpecs
}()

func GenerateAuthTestSpec(authTestSpec AuthTestSpec) test.Spec {
	runFunc := func(t *test.Env) {
		// Default values
		var (
			// All test cases send a simple TransitionConfigurationV1 to check the Authentication mechanism (JWT)
			tConf = api.TransitionConfigurationV1{
				TerminalTotalDifficulty: (*hexutil.Big)(t.MainTTD()),
				TerminalBlockHash:       common.Hash{},
				TerminalBlockNumber:     0,
			}
			testSecret = authTestSpec.CustomAuthSecretBytes
			// Time drift test cases are reattempted in order to mitigate false negatives
			retryAttemptsLeft = authTestSpec.RetryAttempts
		)

		for {
			var testTime = time.Now()
			if testSecret == nil {
				testSecret = globals.DefaultJwtTokenSecretBytes
			}
			if authTestSpec.TimeDriftSeconds != 0 {
				testTime = testTime.Add(time.Second * time.Duration(authTestSpec.TimeDriftSeconds))
			}
			if err := t.HiveEngine.PrepareAuthCallToken(testSecret, testTime); err != nil {
				t.Fatalf("FAIL (%s): Unable to prepare the auth token: %v", t.TestName, err)
			}
			ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
			defer cancel()
			_, err := t.HiveEngine.ExchangeTransitionConfigurationV1(ctx, &tConf)
			if (authTestSpec.AuthOk && err == nil) || (!authTestSpec.AuthOk && err != nil) {
				// Test passed
				return
			}
			if retryAttemptsLeft == 0 {
				if err != nil {
					// Test failed because unexpected error
					t.Fatalf("FAIL (%s): Authentication was supposed to pass authentication but failed: %v", t.TestName, err)
				} else {
					// Test failed because unexpected success
					t.Fatalf("FAIL (%s): Authentication was supposed to fail authentication but passed", t.TestName)
				}
			}
			retryAttemptsLeft--
			// Wait at least a second before trying again
			time.Sleep(time.Second)
		}
	}
	return test.Spec{
		Name: authTestSpec.Name,
		Run:  runFunc,
	}
}
