package suite_builder

import (
	"math/big"

	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/ethereum/hive/simulators/eth2/common/testnet"
	"github.com/ethereum/hive/simulators/eth2/common/utils"
	suite_base "github.com/ethereum/hive/simulators/eth2/dencun/suites/base"
	mock_builder "github.com/marioevz/mock-builder/mock"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

var REQUIRES_FINALIZATION_TO_ACTIVATE_BUILDER = []string{
	"lighthouse",
	"teku",
}

type BuilderTestSpec struct {
	suite_base.BaseTestSpec
	VerifyMissedSlotsCount      bool
	ErrorOnHeaderRequest        bool
	ErrorOnPayloadReveal        bool
	InvalidPayloadVersion       bool
	InvalidatePayload           mock_builder.PayloadInvalidation
	InvalidatePayloadAttributes mock_builder.PayloadAttributesInvalidation
}

func (ts BuilderTestSpec) GetTestnetConfig(
	allNodeDefinitions clients.NodeDefinitions,
) *testnet.Config {
	tc := ts.BaseTestSpec.GetTestnetConfig(allNodeDefinitions)

	// Builders are always enabled for these tests
	tc.EnableBuilders = true

	// Builder config
	// Configure the builder according to the error
	tc.BuilderOptions = make([]mock_builder.Option, 0)

	// Bump the built payloads value
	tc.BuilderOptions = append(
		tc.BuilderOptions,
		mock_builder.WithPayloadWeiValueMultiplier(big.NewInt(10)),
		mock_builder.WithExtraDataWatermark("builder payload tst"),
	)

	// Inject test error
	errorInjectEpoch := beacon.Epoch(0)
	if ts.ErrorOnHeaderRequest {
		tc.BuilderOptions = append(
			tc.BuilderOptions,
			mock_builder.WithErrorOnHeaderRequestAtEpoch(errorInjectEpoch),
		)
	}
	if ts.ErrorOnPayloadReveal {
		tc.BuilderOptions = append(
			tc.BuilderOptions,
			mock_builder.WithErrorOnPayloadRevealAtEpoch(errorInjectEpoch),
		)
	}
	if ts.InvalidatePayload != "" {
		tc.BuilderOptions = append(
			tc.BuilderOptions,
			mock_builder.WithPayloadInvalidatorAtEpoch(
				errorInjectEpoch,
				ts.InvalidatePayload,
			),
		)
	}
	if ts.InvalidatePayloadAttributes != "" {
		tc.BuilderOptions = append(
			tc.BuilderOptions,
			mock_builder.WithPayloadAttributesInvalidatorAtEpoch(
				errorInjectEpoch,
				ts.InvalidatePayloadAttributes,
			),
		)
	}
	if ts.InvalidPayloadVersion {
		tc.BuilderOptions = append(
			tc.BuilderOptions,
			mock_builder.WithInvalidBuilderBidVersionAtEpoch(errorInjectEpoch),
		)
	}

	return tc
}

func (ts BuilderTestSpec) GetDescription() *utils.Description {
	desc := ts.BaseTestSpec.GetDescription()
	desc.Add(utils.CategoryTestnetConfiguration, `
	- Deneb starts from genesis.
	- Builder is enabled for all nodes
	- Builder action is enabled from genesis
	- Nodes have the mock-builder configured as builder endpoint`)
	if ts.BuilderProducesValidPayload() {
		desc.Add(utils.CategoryVerificationsConsensusClient, `
	- The builder must be able to include blocks with blobs in the canonical chain, which implicitly verifies:
		- Consensus client is able to properly format header requests to the builder
		- Consensus client is able to properly format blinded signed requests to the builder
		- No signed block contained an invalid format or signature
		- Test fails with a timeout if no payload with blobs is produced after the fork`)
	} else {
		desc.Add(utils.CategoryVerificationsConsensusClient, `
	- The builder starts producing invalid payloads, verify that:
		- None of the produced payloads are included in the canonical chain`)
		if ts.CausesMissedSlot() {
			desc.Add(utils.CategoryVerificationsConsensusClient, `
		- Since action causes missed slot, verify that the circuit breaker correctly kicks in and disables the builder workflow. Builder starts corrupting payloads after fork, hence a single block in the canonical chain after the fork is enough to verify the circuit breaker`)
		}
	}
	return desc
}
