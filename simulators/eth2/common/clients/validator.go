package clients

import (
	"fmt"

	"github.com/ethereum/hive/hivesim"
	consensus_config "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
	blsu "github.com/protolambda/bls12-381-util"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
	"github.com/protolambda/ztyp/tree"
)

type ValidatorClient struct {
	T                *hivesim.T
	HiveClient       *hivesim.Client
	ClientType       string
	OptionsGenerator func(map[common.ValidatorIndex]*consensus_config.KeyDetails) ([]hivesim.StartOption, error)
	Keys             map[common.ValidatorIndex]*consensus_config.KeyDetails
	beacon           *BeaconClient
}

func NewValidatorClient(
	t *hivesim.T,
	validatorDef *hivesim.ClientDefinition,
	optionsGenerator func(map[common.ValidatorIndex]*consensus_config.KeyDetails) ([]hivesim.StartOption, error),
	keys map[common.ValidatorIndex]*consensus_config.KeyDetails,
	bn *BeaconClient,
) *ValidatorClient {
	return &ValidatorClient{
		T:                t,
		ClientType:       validatorDef.Name,
		OptionsGenerator: optionsGenerator,
		Keys:             keys,
		beacon:           bn,
	}
}

func (vc *ValidatorClient) Start(extraOptions ...hivesim.StartOption) error {
	if vc.HiveClient != nil {
		return fmt.Errorf("client already started")
	}
	if len(vc.Keys) == 0 {
		vc.T.Logf("Skipping validator because it has 0 validator keys")
		return nil
	}
	vc.T.Logf("Starting client %s", vc.ClientType)
	opts, err := vc.OptionsGenerator(vc.Keys)
	if err != nil {
		return fmt.Errorf("unable to get start options: %v", err)
	}
	opts = append(opts, extraOptions...)

	if vc.beacon.Builder != nil {
		opts = append(opts, hivesim.Params{
			"HIVE_ETH2_BUILDER_ENDPOINT": vc.beacon.Builder.Address(),
		})
	}

	vc.HiveClient = vc.T.StartClient(vc.ClientType, opts...)
	return nil
}

func (vc *ValidatorClient) Shutdown() error {
	if err := vc.T.Sim.StopClient(vc.T.SuiteID, vc.T.TestID, vc.HiveClient.Container); err != nil {
		return err
	}
	vc.HiveClient = nil
	return nil
}

func (vc *ValidatorClient) IsRunning() bool {
	return vc.HiveClient != nil
}

func (v *ValidatorClient) ContainsKey(pk [48]byte) bool {
	for _, k := range v.Keys {
		if k.ValidatorPubkey == pk {
			return true
		}
	}
	return false
}

func (v *ValidatorClient) ContainsValidatorIndex(
	index common.ValidatorIndex,
) bool {
	_, ok := v.Keys[index]
	return ok
}

type BLSToExecutionChangeInfo struct {
	common.ValidatorIndex
	common.Eth1Address
}

func (v *ValidatorClient) SignBLSToExecutionChange(
	domain common.BLSDomain,
	c BLSToExecutionChangeInfo,
) (*common.SignedBLSToExecutionChange, error) {
	kd, ok := v.Keys[c.ValidatorIndex]
	if !ok {
		return nil, fmt.Errorf(
			"validator client does not contain validator index %d",
			c.ValidatorIndex,
		)
	}
	if len(c.Eth1Address) != 20 {
		return nil, fmt.Errorf("invalid length for execution address")
	}
	kdPubKey := common.BLSPubkey{}
	copy(kdPubKey[:], kd.WithdrawalPubkey[:])
	blsToExecChange := common.BLSToExecutionChange{
		ValidatorIndex:     c.ValidatorIndex,
		FromBLSPubKey:      kdPubKey,
		ToExecutionAddress: c.Eth1Address,
	}
	sigRoot := common.ComputeSigningRoot(
		blsToExecChange.HashTreeRoot(tree.GetHashFn()),
		domain,
	)

	sk := new(blsu.SecretKey)
	sk.Deserialize(&kd.WithdrawalSecretKey)
	signature := blsu.Sign(sk, sigRoot[:]).Serialize()
	return &common.SignedBLSToExecutionChange{
		BLSToExecutionChange: blsToExecChange,
		Signature:            common.BLSSignature(signature),
	}, nil
}

func (v *ValidatorClient) SignVoluntaryExit(
	domain common.BLSDomain,
	epoch common.Epoch,
	validatorIndex common.ValidatorIndex,
) (*phase0.SignedVoluntaryExit, error) {
	kd, ok := v.Keys[validatorIndex]
	if !ok {
		return nil, fmt.Errorf(
			"validator client does not contain validator index %d",
			validatorIndex,
		)
	}
	kdPubKey := common.BLSPubkey{}
	copy(kdPubKey[:], kd.ValidatorPubkey[:])
	voluntaryExit := phase0.VoluntaryExit{
		Epoch:          epoch,
		ValidatorIndex: validatorIndex,
	}
	sigRoot := common.ComputeSigningRoot(
		voluntaryExit.HashTreeRoot(tree.GetHashFn()),
		domain,
	)

	sk := new(blsu.SecretKey)
	sk.Deserialize(&kd.ValidatorSecretKey)
	signature := blsu.Sign(sk, sigRoot[:]).Serialize()
	return &phase0.SignedVoluntaryExit{
		Message:   voluntaryExit,
		Signature: common.BLSSignature(signature),
	}, nil
}

type ValidatorClients []*ValidatorClient

// Return subset of clients that are currently running
func (all ValidatorClients) Running() ValidatorClients {
	res := make(ValidatorClients, 0)
	for _, vc := range all {
		if vc.IsRunning() {
			res = append(res, vc)
		}
	}
	return res
}

// Returns the validator that contains specified validator index
func (all ValidatorClients) ByValidatorIndex(
	validatorIndex common.ValidatorIndex,
) *ValidatorClient {
	for _, v := range all {
		if v.ContainsValidatorIndex(validatorIndex) {
			return v
		}
	}
	return nil
}

func (all ValidatorClients) SignBLSToExecutionChange(
	domain common.BLSDomain,
	c BLSToExecutionChangeInfo,
) (*common.SignedBLSToExecutionChange, error) {
	if v := all.ByValidatorIndex(c.ValidatorIndex); v == nil {
		return nil, fmt.Errorf(
			"validator index %d not found",
			c.ValidatorIndex,
		)
	} else {
		return v.SignBLSToExecutionChange(
			domain,
			c,
		)
	}
}
