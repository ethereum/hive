package clients

import (
	"fmt"

	consensus_config "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
	"github.com/ethereum/hive/simulators/eth2/common/utils"
	blsu "github.com/protolambda/bls12-381-util"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
	"github.com/protolambda/ztyp/tree"
)

type ValidatorClient struct {
	Client
	Logger      utils.Logging
	ClientIndex int

	Keys         map[common.ValidatorIndex]*consensus_config.KeyDetails
	BeaconClient *BeaconClient
}

func (vc *ValidatorClient) Logf(format string, values ...interface{}) {
	if l := vc.Logger; l != nil {
		l.Logf(format, values...)
	}
}

func (vc *ValidatorClient) Start() error {
	if !vc.Client.IsRunning() {
		if len(vc.Keys) == 0 {
			vc.Logf("Skipping validator because it has 0 validator keys")
			return nil
		}
		if managedClient, ok := vc.Client.(ManagedClient); !ok {
			return fmt.Errorf("attempted to start an unmanaged client")
		} else {
			return managedClient.Start()
		}
	}
	return nil
}

func (vc *ValidatorClient) Shutdown() error {
	if managedClient, ok := vc.Client.(ManagedClient); !ok {
		return fmt.Errorf("attempted to shutdown an unmanaged client")
	} else {
		return managedClient.Shutdown()
	}
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
