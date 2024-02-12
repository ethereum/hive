package suite_engine

import (
	"fmt"

	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

type InvalidPayloadAttributesTest struct {
	test.BaseSpec
	Description string
	Customizer  helper.PayloadAttributesCustomizer
	Syncing     bool
}

func (s InvalidPayloadAttributesTest) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (tc InvalidPayloadAttributesTest) GetName() string {
	name := fmt.Sprintf("Invalid PayloadAttributes, %s,", tc.Description)
	if tc.Syncing {
		name += " Syncing=True"
	} else {
		name += " Syncing=False"
	}
	return name
}

func (tc InvalidPayloadAttributesTest) Execute(t *test.Env) {
	// Wait until TTD is reached by this client
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	// Send a forkchoiceUpdated with invalid PayloadAttributes
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnNewPayloadBroadcast: func() {
			// Try to apply the new payload with invalid attributes
			fcu := t.CLMock.LatestForkchoice
			if tc.Syncing {
				// Setting a random hash will put the client into `SYNCING`
				t.Rand.Read(fcu.HeadBlockHash[:])
			} else {
				fcu.HeadBlockHash = t.CLMock.LatestPayloadBuilt.BlockHash
			}
			t.Logf("INFO (%s): Sending EngineForkchoiceUpdated (Syncing=%t) with invalid payload attributes: %s", t.TestName, tc.Syncing, tc.Description)

			// Get the payload attributes
			originalAttr := t.CLMock.LatestPayloadAttributes
			originalAttr.Timestamp += 1
			attr, err := tc.Customizer.GetPayloadAttributes(&originalAttr)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to customize payload attributes: %v", t.TestName, err)
			}

			// 0) Check headBlock is known and there is no missing data, if not respond with SYNCING
			// 1) Check headBlock is VALID, if not respond with INVALID
			// 2) Apply forkchoiceState
			// 3) Check payloadAttributes, if invalid respond with error: code: Invalid payload attributes
			// 4) Start payload build process and respond with VALID
			if tc.Syncing {
				// If we are SYNCING, the outcome should be SYNCING regardless of the validity of the payload atttributes
				r := t.TestEngine.TestEngineForkchoiceUpdated(&fcu, attr, t.CLMock.LatestPayloadBuilt.Timestamp)
				r.ExpectPayloadStatus(test.Syncing)
				r.ExpectPayloadID(nil)
			} else {
				r := t.TestEngine.TestEngineForkchoiceUpdated(&fcu, attr, t.CLMock.LatestPayloadBuilt.Timestamp)
				r.ExpectErrorCode(*globals.INVALID_PAYLOAD_ATTRIBUTES)

				// Check that the forkchoice was applied, regardless of the error
				s := t.TestEngine.TestHeaderByNumber(Head)
				s.ExpectationDescription = "Forkchoice is applied even on invalid payload attributes"
				s.ExpectHash(fcu.HeadBlockHash)
			}
		},
	})
}
