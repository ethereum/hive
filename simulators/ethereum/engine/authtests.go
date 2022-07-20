package main

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
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
		TimeDriftSeconds:      -1 - maxTimeDriftSeconds,
		CustomAuthSecretBytes: nil,
		AuthOk:                false,
		RetryAttempts:         5,
	},
	{
		Name:                  "JWT Authentication: Negative time drift, within limit, correct secret",
		TimeDriftSeconds:      1 - maxTimeDriftSeconds,
		CustomAuthSecretBytes: nil,
		AuthOk:                true,
		RetryAttempts:         5,
	},
	{
		Name:                  "JWT Authentication: Positive time drift, exceeding limit, correct secret",
		TimeDriftSeconds:      maxTimeDriftSeconds + 1,
		CustomAuthSecretBytes: nil,
		AuthOk:                false,
		RetryAttempts:         5,
	},
	{
		Name:                  "JWT Authentication: Positive time drift, within limit, correct secret",
		TimeDriftSeconds:      maxTimeDriftSeconds - 1,
		CustomAuthSecretBytes: nil,
		AuthOk:                true,
		RetryAttempts:         5,
	},
}

var authTests = func() []TestSpec {
	testSpecs := make([]TestSpec, 0)
	for _, authTest := range authTestSpecs {
		testSpecs = append(testSpecs, GenerateAuthTestSpec(authTest))
	}
	return testSpecs
}()

func GenerateAuthTestSpec(authTestSpec AuthTestSpec) TestSpec {
	runFunc := func(t *TestEnv) {
		// Default values
		var (
			// All test cases send a simple TransitionConfigurationV1 to check the Authentication mechanism (JWT)
			tConf = TransitionConfigurationV1{
				TerminalTotalDifficulty: t.MainTTD(),
				TerminalBlockHash:       &(common.Hash{}),
				TerminalBlockNumber:     0,
			}
			testSecret = authTestSpec.CustomAuthSecretBytes
			// Time drift test cases are reattempted in order to mitigate false negatives
			retryAttemptsLeft = authTestSpec.RetryAttempts
		)

		for {
			var testTime = time.Now()
			if testSecret == nil {
				testSecret = defaultJwtTokenSecretBytes
			}
			if authTestSpec.TimeDriftSeconds != 0 {
				testTime = testTime.Add(time.Second * time.Duration(authTestSpec.TimeDriftSeconds))
			}
			if err := t.Engine.PrepareAuthCallToken(testSecret, testTime); err != nil {
				t.Fatalf("FAIL (%s): Unable to prepare the auth token: %v", t.TestName, err)
			}
			err := t.Engine.c.CallContext(t.Engine.Ctx(), &tConf, "engine_exchangeTransitionConfigurationV1", tConf)
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
	return TestSpec{
		Name: authTestSpec.Name,
		Run:  runFunc,
	}
}
