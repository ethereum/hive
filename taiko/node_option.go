package taiko

import (
	"math/big"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/hivesim"
)

func WithNoCheck() NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			"HIVE_CHECK_LIVE_PORT": "0",
		})
	}
}
func WithELNodeType(typ string) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envNodeType: typ,
		})
	}
}

func WithNetworkID(id uint64) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envNetworkID: strconv.FormatUint(id, 10),
		})
	}
}

func WithBootNode(nodes string) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envBootNode: nodes,
		})
	}
}

func WithCliquePeriod(seconds uint64) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envCliquePeriod: strconv.FormatUint(seconds, 10),
		})
	}
}

func WithL1ChainID(chainID *big.Int) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envTaikoL1ChainID: chainID.String(),
		})
	}

}

func WithRole(role string) NodeOption {
	return func(n *Node) {
		n.role = role
		n.opts = append(n.opts, hivesim.Params{envTaikoRole: role})
	}
}

func WithL1HTTPEndpoint(url string) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envTaikoL1HTTPEndpoint: url,
		})
	}
}

func WithL2HTTPEndpoint(url string) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envTaikoL2HTTPEndpoint: url,
		})
	}
}

func WithL1WSEndpoint(url string) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envTaikoL1WSEndpoint: url,
		})
	}
}

func WithL2WSEndpoint(url string) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envTaikoL2WSEndpoint: url,
		})
	}
}

func WithL2EngineEndpoint(url string) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envTaikoL2EngineEndpoint: url,
		})
	}
}

func WithL1ContractAddress(addr common.Address) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envTaikoL1RollupAddress: addr.Hex(),
		})
	}
}

func WithL2ContractAddress(addr common.Address) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envTaikoL2RollupAddress: addr.Hex(),
		})
	}
}

func WithThrowawayBlockBuilderPrivateKey(key string) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envTaikoThrowawayBlockBuilderPrivateKey: key,
		})
	}
}

func WithEnableL2P2P() NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			evnTaikoEnableL2P2P: "true",
		})
	}
}

func WithJWTSecret(key string) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envTaikoJWTSecret: key,
		})
	}
}

func WithProposerPrivateKey(key string) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envTaikoProposerPrivateKey: key,
		})
	}
}

func WithSuggestedFeeRecipient(add common.Address) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envTaikoSuggestedFeeRecipient: add.Hex(),
		})
	}
}

func WithProposeInterval(t time.Duration) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envTaikoProposeInterval: t.String(),
		})
	}
}

func WithProverPrivateKey(key string) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envTaikoProverPrivateKey: key,
		})
	}
}

func WithPrivateKey(key string) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envTaikoPrivateKey: key,
		})
	}
}

func WithL1DeployerAddress(add common.Address) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envTaikoL1DeployerAddress: add.Hex(),
		})
	}
}

func WithL2GenesisBlockHash(hash common.Hash) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envTaikoL2GenesisBlockHash: hash.Hex(),
		})
	}
}

func WithMainnetUrl(url string) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envTaikoMainnetUrl: url,
		})
	}
}

func WithL2ChainID(chainID *big.Int) NodeOption {
	return func(n *Node) {
		n.opts = append(n.opts, hivesim.Params{
			envTaikoL2ChainID: chainID.String(),
		})
	}
}
