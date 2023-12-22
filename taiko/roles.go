package taiko

import (
	"github.com/ethereum/hive/hivesim"
	"github.com/taikoxyz/taiko-client/proposer"
	"github.com/taikoxyz/taiko-client/prover"
)

const (
	taikoL1       = "taiko-l1"
	taikoDriver   = "taiko-driver"
	taikoGeth     = "taiko-geth"
	taikoProposer = "taiko-proposer"
	taikoProver   = "taiko-prover"
	taikoProtocol = "taiko-protocol"
)

// ClientsByRole is a collection of ClientDefinitions, grouped by role.
type ClientsByRole struct {
	L1       *hivesim.ClientDefinition
	L2       *hivesim.ClientDefinition
	Driver   *hivesim.ClientDefinition
	Proposer *hivesim.ClientDefinition
	Prover   *hivesim.ClientDefinition
	Contract *hivesim.ClientDefinition
}

func Roles(t *hivesim.T, clientDefs []*hivesim.ClientDefinition) *ClientsByRole {
	var out ClientsByRole
	for _, client := range clientDefs {
		if client.HasRole(taikoL1) {
			out.L1 = client
		}
		if client.HasRole(taikoDriver) {
			out.Driver = client
		}
		if client.HasRole(taikoGeth) {
			out.L2 = client
		}
		if client.HasRole(taikoProposer) {
			out.Proposer = client
		}
		if client.HasRole(taikoProver) {
			out.Prover = client
		}
		if client.HasRole(taikoProtocol) {
			out.Contract = client
		}
	}
	return &out
}

func NewProposerConfig(env *TestEnv, l1, l2 *ELNode) *proposer.Config {
	return &proposer.Config{
		L1Endpoint:              l1.WsRpcEndpoint(),
		L2Endpoint:              l2.HttpRpcEndpoint(),
		TaikoL1Address:          l1.deploy.rollupAddress,
		TaikoL2Address:          l2.deploy.rollupAddress,
		L1ProposerPrivKey:       env.Conf.L2.Proposer.PrivateKey,
		L2SuggestedFeeRecipient: env.Conf.L2.SuggestedFeeRecipient.Address,
		ProposeInterval:         &env.Conf.L2.ProposeInterval,
		ShufflePoolContent:      true,
	}
}

func NewProposer(t *hivesim.T, env *TestEnv, c *proposer.Config) (*proposer.Proposer, error) {
	p := new(proposer.Proposer)
	if err := proposer.InitFromConfig(env.Context, p, c); err != nil {
		return nil, err
	}
	return p, nil
}

func NewProverConfig(env *TestEnv) *prover.Config {
	return &prover.Config{
		// TODO
	}
}

func NewProver(t *hivesim.T, env *TestEnv, c *prover.Config) (*prover.Prover, error) {
	p := new(prover.Prover)
	if err := prover.InitFromConfig(env.Context, p, c); err != nil {
		return nil, err
	}
	return p, nil
}
