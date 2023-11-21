package suite_exchange_capabilities

import (
	"fmt"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/suites/filler"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
	"golang.org/x/exp/slices"
)

var Suite = hivesim.Suite{
	Name: "engine-exchange-capabilities",
	Description: `
Test Engine API exchange capabilities: https://github.com/ethereum/execution-apis/blob/main/src/engine/common.md#capabilities`,
	Location: "suites/exchange_capabilities",
}

var CapabilitiesMap = map[config.Fork][]string{
	config.Shanghai: {
		"engine_newPayloadV1",
		"engine_newPayloadV2",
		"engine_forkchoiceUpdatedV1",
		"engine_forkchoiceUpdatedV2",
		"engine_getPayloadV1",
		"engine_getPayloadV2",
	},
	config.Cancun: {
		"engine_newPayloadV1",
		"engine_newPayloadV2",
		"engine_newPayloadV3",
		"engine_forkchoiceUpdatedV1",
		"engine_forkchoiceUpdatedV2",
		"engine_forkchoiceUpdatedV3",
		"engine_getPayloadV1",
		"engine_getPayloadV2",
		"engine_getPayloadV3",
	},
}

func init() {
	tests := make([]test.Spec, 0)

	for _, fork := range []config.Fork{
		config.Shanghai,
		config.Cancun,
	} {
		// Each fork we test:
		// - The fork is configured and active
		// - The fork is configured but not active
		capabilities, ok := CapabilitiesMap[fork]
		if !ok {
			panic("Capabilities not defined for fork")
		}
		for _, active := range []bool{true, false} {
			var (
				nameStr  string
				descStr  string
				forkTime uint64
			)
			if active {
				nameStr = "Active"
				descStr = fmt.Sprintf(`
				- Start a node with the %s fork configured at genesis
				- Query engine_exchangeCapabilities and verify the capabilities returned
				- Capabilities must include the following list:
			`, fork)
				forkTime = 0
			} else {
				nameStr = "Not active"
				descStr = fmt.Sprintf(`
				- Start a node with the %s fork configured in the future
				- Query engine_exchangeCapabilities and verify the capabilities returned
				- Capabilities must include the following list, even when the fork is not active yet:
			`, fork)
				forkTime = globals.GenesisTimestamp * 2
			}
			for _, cap := range capabilities {
				descStr += fmt.Sprintf(`
				- %s\n`, cap)
			}

			tests = append(tests, ExchangeCapabilitiesSpec{
				BaseSpec: test.BaseSpec{
					Name:     fmt.Sprintf("Exchange Capabilities - %s (%s)", fork, nameStr),
					About:    descStr,
					Category: string(fork),
					MainFork: fork,
					ForkTime: forkTime,
				},
				MinimalExpectedCapabilitiesSet: capabilities,
			})
		}
	}
	// Add the tests to the suite
	filler.FillSuite(&Suite, tests, filler.FullNode)
}

type ExchangeCapabilitiesSpec struct {
	test.BaseSpec
	MinimalExpectedCapabilitiesSet []string
}

func (s ExchangeCapabilitiesSpec) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (s ExchangeCapabilitiesSpec) Execute(t *test.Env) {
	if returnedCapabilities, err := t.HiveEngine.ExchangeCapabilities(t.TestContext, s.MinimalExpectedCapabilitiesSet); err != nil {
		t.Fatalf("FAIL (%s): Unable request capabilities: %v", t.TestName, err)
	} else {
		for _, cap := range s.MinimalExpectedCapabilitiesSet {
			if !slices.Contains(returnedCapabilities, cap) {
				t.Fatalf("FAIL (%s): Expected capability (%s) not found", t.TestName, cap)
			}
		}
	}
}
