package testnet

import (
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	consensus_config "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
	blsu "github.com/protolambda/bls12-381-util"
)

type Environment struct {
	Clients        *clients.ClientDefinitionsByRole
	Keys           []*consensus_config.KeyDetails
	Secrets        *[]blsu.SecretKey
	LogEngineCalls bool
}
