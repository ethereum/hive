package suite_exchange_capabilities

import (
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
	"golang.org/x/exp/slices"
)

var Tests = []test.Spec{
	{
		Name: "Exchange Capabilities",
		Run:  exCapTests,
	},
}

var minimalSetExpectedSupportedELCapabilities = []string{
	"engine_newPayloadV1",
	"engine_newPayloadV2",
	"engine_forkchoiceUpdatedV1",
	"engine_forkchoiceUpdatedV2",
	"engine_getPayloadV1",
	"engine_getPayloadV2",
	// "engine_getPayloadBodiesByRangeV1",
}

func exCapTests(t *test.Env) {
	if returnedCapabilities, err := t.HiveEngine.ExchangeCapabilities(t.TestContext, minimalSetExpectedSupportedELCapabilities); err != nil {
		t.Fatalf("FAIL (%s): Unable request capabilities: %v", t.TestName, err)
	} else {
		for _, cap := range minimalSetExpectedSupportedELCapabilities {
			if !slices.Contains(returnedCapabilities, cap) {
				t.Fatalf("FAIL (%s): Expected capability (%s) not found", t.TestName, cap)
			}
		}
	}
}
