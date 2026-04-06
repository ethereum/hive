package suite_exchange_capabilities

import (
	"fmt"

	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
	"golang.org/x/exp/slices"
)

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

var Tests = make([]test.Spec, 0)

func init() {
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
				forkTime uint64
			)
			if active {
				nameStr = "Active"
				forkTime = 0
			} else {
				nameStr = "Not active"
				forkTime = globals.GenesisTimestamp * 2
			}
			Tests = append(Tests, ExchangeCapabilitiesSpec{
				BaseSpec: test.BaseSpec{
					Name:     fmt.Sprintf("Exchange Capabilities - %s (%s)", fork, nameStr),
					MainFork: fork,
					ForkTime: forkTime,
				},
				MinimalExpectedCapabilitiesSet: capabilities,
			})
		}
	}
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
	t.HiveEngine.PrepareDefaultAuthCallToken()

	returnedCapabilities, err := t.HiveEngine.ExchangeCapabilities(t.TestContext, s.MinimalExpectedCapabilitiesSet)
	if err != nil {
		t.Fatalf("FAIL (%s): Unable request capabilities: %v", t.TestName, err)
	}
	for _, cap := range s.MinimalExpectedCapabilitiesSet {
		if !slices.Contains(returnedCapabilities, cap) {
			t.Fatalf("FAIL (%s): Expected capability (%s) not found", t.TestName, cap)
		}
	}
}
