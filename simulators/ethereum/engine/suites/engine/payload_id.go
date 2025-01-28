package suite_engine

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

type PayloadAttributesFieldChange string

const (
	PayloadAttributesIncreaseTimestamp         PayloadAttributesFieldChange = "Increase Timestamp"
	PayloadAttributesRandom                    PayloadAttributesFieldChange = "Modify Random"
	PayloadAttributesSuggestedFeeRecipient     PayloadAttributesFieldChange = "Modify SuggestedFeeRecipient"
	PayloadAttributesAddWithdrawal             PayloadAttributesFieldChange = "Add Withdrawal"
	PayloadAttributesModifyWithdrawalAmount    PayloadAttributesFieldChange = "Modify Withdrawal Amount"
	PayloadAttributesModifyWithdrawalIndex     PayloadAttributesFieldChange = "Modify Withdrawal Index"
	PayloadAttributesModifyWithdrawalValidator PayloadAttributesFieldChange = "Modify Withdrawal Validator"
	PayloadAttributesModifyWithdrawalAddress   PayloadAttributesFieldChange = "Modify Withdrawal Address"
	PayloadAttributesRemoveWithdrawal          PayloadAttributesFieldChange = "Remove Withdrawal"
	PayloadAttributesParentBeaconRoot          PayloadAttributesFieldChange = "Modify Parent Beacon Root"
)

type UniquePayloadIDTest struct {
	test.BaseSpec
	FieldModification PayloadAttributesFieldChange
}

func (s UniquePayloadIDTest) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (tc UniquePayloadIDTest) GetName() string {
	return fmt.Sprintf("Unique Payload ID, %s", tc.FieldModification)
}

// Check that the payload id returned on a forkchoiceUpdated call is different
// when the attributes change
func (tc UniquePayloadIDTest) Execute(t *test.Env) {
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnPayloadAttributesGenerated: func() {
			payloadAttributes := t.CLMock.LatestPayloadAttributes
			switch tc.FieldModification {
			case PayloadAttributesIncreaseTimestamp:
				payloadAttributes.Timestamp += 1
			case PayloadAttributesRandom:
				payloadAttributes.Random[0] = payloadAttributes.Random[0] + 1
			case PayloadAttributesSuggestedFeeRecipient:
				payloadAttributes.SuggestedFeeRecipient[0] = payloadAttributes.SuggestedFeeRecipient[0] + 1
			case PayloadAttributesAddWithdrawal:
				newWithdrawal := &types.Withdrawal{}
				payloadAttributes.Withdrawals = append(payloadAttributes.Withdrawals, newWithdrawal)
			case PayloadAttributesRemoveWithdrawal:
				payloadAttributes.Withdrawals = payloadAttributes.Withdrawals[1:]
			case PayloadAttributesModifyWithdrawalAmount,
				PayloadAttributesModifyWithdrawalIndex,
				PayloadAttributesModifyWithdrawalValidator,
				PayloadAttributesModifyWithdrawalAddress:
				if len(payloadAttributes.Withdrawals) == 0 {
					t.Fatalf("Cannot modify withdrawal when there are no withdrawals")
				}
				modifiedWithdrawal := *payloadAttributes.Withdrawals[0]
				switch tc.FieldModification {
				case PayloadAttributesModifyWithdrawalAmount:
					modifiedWithdrawal.Amount += 1
				case PayloadAttributesModifyWithdrawalIndex:
					modifiedWithdrawal.Index += 1
				case PayloadAttributesModifyWithdrawalValidator:
					modifiedWithdrawal.Validator += 1
				case PayloadAttributesModifyWithdrawalAddress:
					modifiedWithdrawal.Address[0] = modifiedWithdrawal.Address[0] + 1
				}
				payloadAttributes.Withdrawals = append(types.Withdrawals{&modifiedWithdrawal}, payloadAttributes.Withdrawals[1:]...)
			case PayloadAttributesParentBeaconRoot:
				if payloadAttributes.BeaconRoot == nil {
					t.Fatalf("Cannot modify parent beacon root when there is no parent beacon root")
				}
				newBeaconRoot := *payloadAttributes.BeaconRoot
				newBeaconRoot[0] = newBeaconRoot[0] + 1
				payloadAttributes.BeaconRoot = &newBeaconRoot
			default:
				t.Fatalf("Unknown field change: %s", tc.FieldModification)
			}

			// Request the payload with the modified attributes and add the payload ID to the list of known IDs
			r := t.TestEngine.TestEngineForkchoiceUpdated(&t.CLMock.
				LatestForkchoice, &payloadAttributes, t.CLMock.LatestHeader.Time)
			r.ExpectNoError()
			t.CLMock.AddPayloadID(t.Engine, r.Response.PayloadID)
		},
	})
}
