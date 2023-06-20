package suite_exchange_capabilities

import (
	"math/big"

	"github.com/ethereum/hive/simulators/ethereum/engine/test"
	"golang.org/x/exp/slices"
)

var (
	ShanghaiCapabilities = []string{
		"engine_newPayloadV1",
		"engine_newPayloadV2",
		"engine_forkchoiceUpdatedV1",
		"engine_forkchoiceUpdatedV2",
		"engine_getPayloadV1",
		"engine_getPayloadV2",
	}
	CancunCapabilities = []string{
		"engine_newPayloadV1",
		"engine_newPayloadV2",
		"engine_newPayloadV3",
		"engine_forkchoiceUpdatedV1",
		"engine_forkchoiceUpdatedV2",
		"engine_getPayloadV1",
		"engine_getPayloadV2",
		"engine_getPayloadV3",
	}
)

var Tests = []test.SpecInterface{
	ExchangeCapabilitiesSpec{
		Spec: test.Spec{
			Name: "Exchange Capabilities - Shanghai",
			ForkConfig: test.ForkConfig{
				ShanghaiTimestamp: big.NewInt(0),
			},
		},
		MinimalExpectedCapabilitiesSet: ShanghaiCapabilities,
	},
	ExchangeCapabilitiesSpec{
		Spec: test.Spec{
			Name: "Exchange Capabilities - Shanghai (Not active)",
			ForkConfig: test.ForkConfig{
				ShanghaiTimestamp: big.NewInt(1000),
			},
		},
		MinimalExpectedCapabilitiesSet: ShanghaiCapabilities,
	},
}

type ExchangeCapabilitiesSpec struct {
	test.Spec
	MinimalExpectedCapabilitiesSet []string
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
