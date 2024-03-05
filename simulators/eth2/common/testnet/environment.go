package testnet

import (
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	consensus_config "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
)

type Environment struct {
	Clients        *clients.ClientDefinitionsByRole
	Validators     consensus_config.ValidatorsSetupDetails
	LogEngineCalls bool
}
