package execution_config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/config"
)

// The runtime deposit contract code, along with the storage that would otherwise have been initialized
// in the deployment constructor call.
// The storage tracks the default zero-hash of each binary tree layer, to shape the initial stack of an empty tree.
var embeddedDepositContract = `
{
  "balance": "0",
  "code": "0x60806040526004361061003f5760003560e01c806301ffc9a71461004457806322895118146100a4578063621fd130146101ba578063c5f2892f14610244575b600080fd5b34801561005057600080fd5b506100906004803603602081101561006757600080fd5b50357fffffffff000000000000000000000000000000000000000000000000000000001661026b565b604080519115158252519081900360200190f35b6101b8600480360360808110156100ba57600080fd5b8101906020810181356401000000008111156100d557600080fd5b8201836020820111156100e757600080fd5b8035906020019184600183028401116401000000008311171561010957600080fd5b91939092909160208101903564010000000081111561012757600080fd5b82018360208201111561013957600080fd5b8035906020019184600183028401116401000000008311171561015b57600080fd5b91939092909160208101903564010000000081111561017957600080fd5b82018360208201111561018b57600080fd5b803590602001918460018302840111640100000000831117156101ad57600080fd5b919350915035610304565b005b3480156101c657600080fd5b506101cf6110b5565b6040805160208082528351818301528351919283929083019185019080838360005b838110156102095781810151838201526020016101f1565b50505050905090810190601f1680156102365780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b34801561025057600080fd5b506102596110c7565b60408051918252519081900360200190f35b60007fffffffff0000000000000000000000000000000000000000000000000000000082167f01ffc9a70000000000000000000000000000000000000000000000000000000014806102fe57507fffffffff0000000000000000000000000000000000000000000000000000000082167f8564090700000000000000000000000000000000000000000000000000000000145b92915050565b6030861461035d576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260268152602001806118056026913960400191505060405180910390fd5b602084146103b6576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252603681526020018061179c6036913960400191505060405180910390fd5b6060821461040f576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260298152602001806118786029913960400191505060405180910390fd5b670de0b6b3a7640000341015610470576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260268152602001806118526026913960400191505060405180910390fd5b633b9aca003406156104cd576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260338152602001806117d26033913960400191505060405180910390fd5b633b9aca00340467ffffffffffffffff811115610535576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252602781526020018061182b6027913960400191505060405180910390fd5b6060610540826114ba565b90507f649bbc62d0e31342afea4e5cd82d4049e7e1ee912fc0889aa790803be39038c589898989858a8a6105756020546114ba565b6040805160a0808252810189905290819060208201908201606083016080840160c085018e8e80828437600083820152601f017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe01690910187810386528c815260200190508c8c808284376000838201819052601f9091017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe01690920188810386528c5181528c51602091820193918e019250908190849084905b83811015610648578181015183820152602001610630565b50505050905090810190601f1680156106755780820380516001836020036101000a031916815260200191505b5086810383528881526020018989808284376000838201819052601f9091017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0169092018881038452895181528951602091820193918b019250908190849084905b838110156106ef5781810151838201526020016106d7565b50505050905090810190601f16801561071c5780820380516001836020036101000a031916815260200191505b509d505050505050505050505050505060405180910390a1600060028a8a600060801b604051602001808484808284377fffffffffffffffffffffffffffffffff0000000000000000000000000000000090941691909301908152604080517ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0818403018152601090920190819052815191955093508392506020850191508083835b602083106107fc57805182527fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe090920191602091820191016107bf565b51815160209384036101000a7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff01801990921691161790526040519190930194509192505080830381855afa158015610859573d6000803e3d6000fd5b5050506040513d602081101561086e57600080fd5b5051905060006002806108846040848a8c6116fe565b6040516020018083838082843780830192505050925050506040516020818303038152906040526040518082805190602001908083835b602083106108f857805182527fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe090920191602091820191016108bb565b51815160209384036101000a7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff01801990921691161790526040519190930194509192505080830381855afa158015610955573d6000803e3d6000fd5b5050506040513d602081101561096a57600080fd5b5051600261097b896040818d6116fe565b60405160009060200180848480828437919091019283525050604080518083038152602092830191829052805190945090925082918401908083835b602083106109f457805182527fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe090920191602091820191016109b7565b51815160209384036101000a7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff01801990921691161790526040519190930194509192505080830381855afa158015610a51573d6000803e3d6000fd5b5050506040513d6020811015610a6657600080fd5b5051604080516020818101949094528082019290925280518083038201815260609092019081905281519192909182918401908083835b60208310610ada57805182527fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe09092019160209182019101610a9d565b51815160209384036101000a7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff01801990921691161790526040519190930194509192505080830381855afa158015610b37573d6000803e3d6000fd5b5050506040513d6020811015610b4c57600080fd5b50516040805160208101858152929350600092600292839287928f928f92018383808284378083019250505093505050506040516020818303038152906040526040518082805190602001908083835b60208310610bd957805182527fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe09092019160209182019101610b9c565b51815160209384036101000a7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff01801990921691161790526040519190930194509192505080830381855afa158015610c36573d6000803e3d6000fd5b5050506040513d6020811015610c4b57600080fd5b50516040518651600291889160009188916020918201918291908601908083835b60208310610ca957805182527fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe09092019160209182019101610c6c565b6001836020036101000a0380198251168184511680821785525050505050509050018367ffffffffffffffff191667ffffffffffffffff1916815260180182815260200193505050506040516020818303038152906040526040518082805190602001908083835b60208310610d4e57805182527fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe09092019160209182019101610d11565b51815160209384036101000a7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff01801990921691161790526040519190930194509192505080830381855afa158015610dab573d6000803e3d6000fd5b5050506040513d6020811015610dc057600080fd5b5051604080516020818101949094528082019290925280518083038201815260609092019081905281519192909182918401908083835b60208310610e3457805182527fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe09092019160209182019101610df7565b51815160209384036101000a7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff01801990921691161790526040519190930194509192505080830381855afa158015610e91573d6000803e3d6000fd5b5050506040513d6020811015610ea657600080fd5b50519050858114610f02576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260548152602001806117486054913960600191505060405180910390fd5b60205463ffffffff11610f60576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260218152602001806117276021913960400191505060405180910390fd5b602080546001019081905560005b60208110156110a9578160011660011415610fa0578260008260208110610f9157fe5b0155506110ac95505050505050565b600260008260208110610faf57fe5b01548460405160200180838152602001828152602001925050506040516020818303038152906040526040518082805190602001908083835b6020831061102557805182527fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe09092019160209182019101610fe8565b51815160209384036101000a7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff01801990921691161790526040519190930194509192505080830381855afa158015611082573d6000803e3d6000fd5b5050506040513d602081101561109757600080fd5b50519250600282049150600101610f6e565b50fe5b50505050505050565b60606110c26020546114ba565b905090565b6020546000908190815b60208110156112f05781600116600114156111e6576002600082602081106110f557fe5b01548460405160200180838152602001828152602001925050506040516020818303038152906040526040518082805190602001908083835b6020831061116b57805182527fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0909201916020918201910161112e565b51815160209384036101000a7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff01801990921691161790526040519190930194509192505080830381855afa1580156111c8573d6000803e3d6000fd5b5050506040513d60208110156111dd57600080fd5b505192506112e2565b600283602183602081106111f657fe5b015460405160200180838152602001828152602001925050506040516020818303038152906040526040518082805190602001908083835b6020831061126b57805182527fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0909201916020918201910161122e565b51815160209384036101000a7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff01801990921691161790526040519190930194509192505080830381855afa1580156112c8573d6000803e3d6000fd5b5050506040513d60208110156112dd57600080fd5b505192505b6002820491506001016110d1565b506002826112ff6020546114ba565b600060401b6040516020018084815260200183805190602001908083835b6020831061135a57805182527fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0909201916020918201910161131d565b51815160209384036101000a7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff01801990921691161790527fffffffffffffffffffffffffffffffffffffffffffffffff000000000000000095909516920191825250604080518083037ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff8018152601890920190819052815191955093508392850191508083835b6020831061143f57805182527fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe09092019160209182019101611402565b51815160209384036101000a7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff01801990921691161790526040519190930194509192505080830381855afa15801561149c573d6000803e3d6000fd5b5050506040513d60208110156114b157600080fd5b50519250505090565b60408051600880825281830190925260609160208201818036833701905050905060c082901b8060071a60f81b826000815181106114f457fe5b60200101907effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916908160001a9053508060061a60f81b8260018151811061153757fe5b60200101907effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916908160001a9053508060051a60f81b8260028151811061157a57fe5b60200101907effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916908160001a9053508060041a60f81b826003815181106115bd57fe5b60200101907effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916908160001a9053508060031a60f81b8260048151811061160057fe5b60200101907effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916908160001a9053508060021a60f81b8260058151811061164357fe5b60200101907effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916908160001a9053508060011a60f81b8260068151811061168657fe5b60200101907effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916908160001a9053508060001a60f81b826007815181106116c957fe5b60200101907effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916908160001a90535050919050565b6000808585111561170d578182fd5b83861115611719578182fd5b505082019391909203915056fe4465706f736974436f6e74726163743a206d65726b6c6520747265652066756c6c4465706f736974436f6e74726163743a207265636f6e7374727563746564204465706f7369744461746120646f6573206e6f74206d6174636820737570706c696564206465706f7369745f646174615f726f6f744465706f736974436f6e74726163743a20696e76616c6964207769746864726177616c5f63726564656e7469616c73206c656e6774684465706f736974436f6e74726163743a206465706f7369742076616c7565206e6f74206d756c7469706c65206f6620677765694465706f736974436f6e74726163743a20696e76616c6964207075626b6579206c656e6774684465706f736974436f6e74726163743a206465706f7369742076616c756520746f6f20686967684465706f736974436f6e74726163743a206465706f7369742076616c756520746f6f206c6f774465706f736974436f6e74726163743a20696e76616c6964207369676e6174757265206c656e677468a26469706673582212201dd26f37a621703009abf16e77e69c93dc50c79db7f6cc37543e3e0e3decdc9764736f6c634300060b0033",
  "storage": {
	"0x0000000000000000000000000000000000000000000000000000000000000022": "0xf5a5fd42d16a20302798ef6ed309979b43003d2320d9f0e8ea9831a92759fb4b",
	"0x0000000000000000000000000000000000000000000000000000000000000023": "0xdb56114e00fdd4c1f85c892bf35ac9a89289aaecb1ebd0a96cde606a748b5d71",
	"0x0000000000000000000000000000000000000000000000000000000000000024": "0xc78009fdf07fc56a11f122370658a353aaa542ed63e44c4bc15ff4cd105ab33c",
	"0x0000000000000000000000000000000000000000000000000000000000000025": "0x536d98837f2dd165a55d5eeae91485954472d56f246df256bf3cae19352a123c",
	"0x0000000000000000000000000000000000000000000000000000000000000026": "0x9efde052aa15429fae05bad4d0b1d7c64da64d03d7a1854a588c2cb8430c0d30",
	"0x0000000000000000000000000000000000000000000000000000000000000027": "0xd88ddfeed400a8755596b21942c1497e114c302e6118290f91e6772976041fa1",
	"0x0000000000000000000000000000000000000000000000000000000000000028": "0x87eb0ddba57e35f6d286673802a4af5975e22506c7cf4c64bb6be5ee11527f2c",
	"0x0000000000000000000000000000000000000000000000000000000000000029": "0x26846476fd5fc54a5d43385167c95144f2643f533cc85bb9d16b782f8d7db193",
	"0x000000000000000000000000000000000000000000000000000000000000002a": "0x506d86582d252405b840018792cad2bf1259f1ef5aa5f887e13cb2f0094f51e1",
	"0x000000000000000000000000000000000000000000000000000000000000002b": "0xffff0ad7e659772f9534c195c815efc4014ef1e1daed4404c06385d11192e92b",
	"0x000000000000000000000000000000000000000000000000000000000000002c": "0x6cf04127db05441cd833107a52be852868890e4317e6a02ab47683aa75964220",
	"0x000000000000000000000000000000000000000000000000000000000000002d": "0xb7d05f875f140027ef5118a2247bbb84ce8f2f0f1123623085daf7960c329f5f",
	"0x000000000000000000000000000000000000000000000000000000000000002e": "0xdf6af5f5bbdb6be9ef8aa618e4bf8073960867171e29676f8b284dea6a08a85e",
	"0x000000000000000000000000000000000000000000000000000000000000002f": "0xb58d900f5e182e3c50ef74969ea16c7726c549757cc23523c369587da7293784",
	"0x0000000000000000000000000000000000000000000000000000000000000030": "0xd49a7502ffcfb0340b1d7885688500ca308161a7f96b62df9d083b71fcc8f2bb",
	"0x0000000000000000000000000000000000000000000000000000000000000031": "0x8fe6b1689256c0d385f42f5bbe2027a22c1996e110ba97c171d3e5948de92beb",
	"0x0000000000000000000000000000000000000000000000000000000000000032": "0x8d0d63c39ebade8509e0ae3c9c3876fb5fa112be18f905ecacfecb92057603ab",
	"0x0000000000000000000000000000000000000000000000000000000000000033": "0x95eec8b2e541cad4e91de38385f2e046619f54496c2382cb6cacd5b98c26f5a4",
	"0x0000000000000000000000000000000000000000000000000000000000000034": "0xf893e908917775b62bff23294dbbe3a1cd8e6cc1c35b4801887b646a6f81f17f",
	"0x0000000000000000000000000000000000000000000000000000000000000035": "0xcddba7b592e3133393c16194fac7431abf2f5485ed711db282183c819e08ebaa",
	"0x0000000000000000000000000000000000000000000000000000000000000036": "0x8a8d7fe3af8caa085a7639a832001457dfb9128a8061142ad0335629ff23ff9c",
	"0x0000000000000000000000000000000000000000000000000000000000000037": "0xfeb3c337d7a51a6fbf00b9e34c52e1c9195c969bd4e7a0bfd51d5c5bed9c1167",
	"0x0000000000000000000000000000000000000000000000000000000000000038": "0xe71f0aa83cc32edfbefa9f4d3e0174ca85182eec9f3a09f6a6c0df6377a510d7",
	"0x0000000000000000000000000000000000000000000000000000000000000039": "0x31206fa80a50bb6abe29085058f16212212a60eec8f049fecb92d8c8e0a84bc0",
	"0x000000000000000000000000000000000000000000000000000000000000003a": "0x21352bfecbeddde993839f614c3dac0a3ee37543f9b412b16199dc158e23b544",
	"0x000000000000000000000000000000000000000000000000000000000000003b": "0x619e312724bb6d7c3153ed9de791d764a366b389af13c58bf8a8d90481a46765",
	"0x000000000000000000000000000000000000000000000000000000000000003c": "0x7cdd2986268250628d0c10e385c58c6191e6fbe05191bcc04f133f2cea72c1c4",
	"0x000000000000000000000000000000000000000000000000000000000000003d": "0x848930bd7ba8cac54661072113fb278869e07bb8587f91392933374d017bcbe1",
	"0x000000000000000000000000000000000000000000000000000000000000003e": "0x8869ff2c22b28cc10510d9853292803328be4fb0e80495e8bb8d271f5b889636",
	"0x000000000000000000000000000000000000000000000000000000000000003f": "0xb5fe28e79f1b850f8658246ce9b6a1e7b49fc06db7143e8fe0b4f2b0c5523a5c",
	"0x0000000000000000000000000000000000000000000000000000000000000040": "0x985e929f70af28d0bdd1a90a808f977f597c7c778c489e98d3bd8910d31ac0f7"
  }
}
`
var embeddedBeaconRootContract = `
{
	"balance": "0",
	"code": "0x3373fffffffffffffffffffffffffffffffffffffffe14604457602036146024575f5ffd5b620180005f350680545f35146037575f5ffd5b6201800001545f5260205ff35b42620180004206555f3562018000420662018000015500",
	"nonce": "1",
	"storage": {}
  }
  `

var beaconRootContractAddress = common.HexToAddress("0x000F3df6D732807Ef1319fB7B8bB8522d0Beac02")

var (
	CLIQUE_PERIOD_DEFAULT        = uint64(2)
	DEFAULT_CLIQUE_PRIVATE_KEY   = "9c647b8b7c4e7c3490668fb6c11473619db80c93704c70893d3813af4090c39c"
	DEFAULT_CLIQUE_MINER_ADDRESS = "658bdf435d810c91414ec09147daa6db62406379"
	DEFAULT_ETHASH_MINER_ADDRESS = "1212121212121212121212121212121212121212"
)

type ExecutionConsensus interface {
	Configure(*ExecutionGenesis) error
	HiveParams(int) hivesim.Params
	DifficultyPerBlock() *big.Int
	SecondsPerBlock() uint64
}

type ExecutionEthashConsensus struct {
	MinerAddress string
	MiningNodes  int
}

func (c ExecutionEthashConsensus) Configure(*ExecutionGenesis) error {
	// Nothing to do here...
	return nil
}

func (c ExecutionEthashConsensus) HiveParams(node int) hivesim.Params {
	if c.MinerAddress == "" {
		c.MinerAddress = DEFAULT_ETHASH_MINER_ADDRESS
	}
	if c.MiningNodes == 0 {
		// Default is that only one node is a miner
		c.MiningNodes = 1
	}
	if node < c.MiningNodes {
		return hivesim.Params{"HIVE_MINER": c.MinerAddress}
	}
	return hivesim.Params{}
}

func (c ExecutionEthashConsensus) DifficultyPerBlock() *big.Int {
	// Approximately 0x20000
	return big.NewInt(131072)
}

func (c ExecutionEthashConsensus) SecondsPerBlock() uint64 {
	// It is really hard to approxmate this value
	return 10
}

// A pre-existing chain is imported by the client, and it is not
// expected that the client mines or produces any blocks.
type ExecutionPreChain struct{}

func (c ExecutionPreChain) Configure(*ExecutionGenesis) error {
	return nil
}

func (c ExecutionPreChain) HiveParams(node int) hivesim.Params {
	return hivesim.Params{}
}

func (c ExecutionPreChain) DifficultyPerBlock() *big.Int {
	// Approximately 0x20000
	return big.NewInt(131072)
}

func (c ExecutionPreChain) SecondsPerBlock() uint64 {
	return 1
}

// A pre-existing chain is imported by the client, and it is not
// expected that the client mines or produces any blocks.
type ExecutionPostMergeGenesis struct{}

func (c ExecutionPostMergeGenesis) Configure(*ExecutionGenesis) error {
	return nil
}

func (c ExecutionPostMergeGenesis) HiveParams(node int) hivesim.Params {
	return hivesim.Params{}
}

func (c ExecutionPostMergeGenesis) DifficultyPerBlock() *big.Int {
	return big.NewInt(0)
}

func (c ExecutionPostMergeGenesis) SecondsPerBlock() uint64 {
	return 12
}

type ExecutionCliqueConsensus struct {
	CliquePeriod uint64
	PrivateKey   string
	MinerAddress string
}

func (c ExecutionCliqueConsensus) Configure(genesis *ExecutionGenesis) error {
	if c.CliquePeriod == 0 {
		c.CliquePeriod = CLIQUE_PERIOD_DEFAULT
	}
	if c.MinerAddress == "" {
		c.MinerAddress = DEFAULT_CLIQUE_MINER_ADDRESS
	}
	genesis.Genesis.Config.Clique = &params.CliqueConfig{
		Period: c.CliquePeriod,
		Epoch:  0,
	}
	genesis.Genesis.ExtraData = common.FromHex(
		"0x0000000000000000000000000000000000000000000000000000000000000000" + c.MinerAddress + "0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
	)
	return nil
}

func (c ExecutionCliqueConsensus) HiveParams(node int) hivesim.Params {
	if node > 0 {
		return hivesim.Params{}
	}
	if c.PrivateKey == "" {
		c.PrivateKey = DEFAULT_CLIQUE_PRIVATE_KEY
	}
	if c.MinerAddress == "" {
		c.MinerAddress = DEFAULT_CLIQUE_MINER_ADDRESS
	}
	return hivesim.Params{
		"HIVE_CLIQUE_PRIVATEKEY": c.PrivateKey,
		"HIVE_MINER":             c.MinerAddress,
	}
}

func (c ExecutionCliqueConsensus) DifficultyPerBlock() *big.Int {
	return big.NewInt(2)
}

func (c ExecutionCliqueConsensus) SecondsPerBlock() uint64 {
	if c.CliquePeriod == 0 {
		return CLIQUE_PERIOD_DEFAULT
	}
	return c.CliquePeriod
}

func BuildChainConfig(
	ttd *big.Int,
	beaconChainGenesisTime uint64,
	slotsPerEpoch uint64,
	secondsPerSlot uint64,
	config *config.ForkConfig,
) (*params.ChainConfig, error) {
	chainConfig := &params.ChainConfig{
		ChainID:                 big.NewInt(7),
		HomesteadBlock:          big.NewInt(0),
		DAOForkBlock:            nil,
		DAOForkSupport:          false,
		EIP150Block:             big.NewInt(0),
		EIP155Block:             big.NewInt(0),
		EIP158Block:             big.NewInt(0),
		ByzantiumBlock:          big.NewInt(0),
		ConstantinopleBlock:     big.NewInt(0),
		PetersburgBlock:         big.NewInt(0),
		IstanbulBlock:           big.NewInt(0),
		MuirGlacierBlock:        big.NewInt(0),
		BerlinBlock:             big.NewInt(0),
		LondonBlock:             big.NewInt(0),
		ArrowGlacierBlock:       big.NewInt(0),
		MergeNetsplitBlock:      big.NewInt(0),
		TerminalTotalDifficulty: ttd,
		Clique:                  nil,
	}

	// Configure post-merge forks
	var (
		previousForkTime = config.BellatrixForkEpoch
		previousFork     = "bellatrix"
	)
	for _, forkConfig := range []struct {
		ForkName       string
		BeaconForkTime *big.Int
		ChainConfig    **uint64
	}{
		{
			ForkName:       "capella",
			BeaconForkTime: config.CapellaForkEpoch,
			ChainConfig:    &chainConfig.ShanghaiTime,
		},
		{
			ForkName:       "deneb",
			BeaconForkTime: config.DenebForkEpoch,
			ChainConfig:    &chainConfig.CancunTime,
		},
	} {
		if forkConfig.BeaconForkTime != nil {
			if previousForkTime == nil {
				return nil, fmt.Errorf("fork '%s' has a time but previous fork '%s' does not", forkConfig.ForkName, previousFork)
			}
			if forkConfig.BeaconForkTime.Cmp(previousForkTime) < 0 {
				return nil, fmt.Errorf("fork '%s' has a time before previous fork '%s'", forkConfig.ForkName, previousFork)
			}
			timestamp := beaconChainGenesisTime + (forkConfig.BeaconForkTime.Uint64() * secondsPerSlot * slotsPerEpoch)
			*forkConfig.ChainConfig = &timestamp
		}
		previousForkTime = forkConfig.BeaconForkTime
		previousFork = forkConfig.ForkName
	}
	return chainConfig, nil
}

type ExecutionGenesis struct {
	Genesis        *core.Genesis
	Block          *types.Block
	Hash           common.Hash
	DepositAddress common.Address
}

func BuildExecutionGenesis(
	genesisTime uint64,
	consensus ExecutionConsensus,
	chainConfig *params.ChainConfig,
	genesisExecAccounts map[common.Address]core.GenesisAccount,
	initialBaseFee *big.Int,
) (*ExecutionGenesis, error) {
	depositContractAddr := common.HexToAddress(
		"0x4242424242424242424242424242424242424242",
	)
	var depositContractAcc core.GenesisAccount
	if err := json.Unmarshal([]byte(embeddedDepositContract), &depositContractAcc); err != nil {
		panic(err)
	}
	genesis := &core.Genesis{
		Config:     chainConfig,
		Nonce:      0,
		Timestamp:  genesisTime,
		ExtraData:  nil,
		GasLimit:   30_000_000,
		Difficulty: big.NewInt(0),
		BaseFee:    initialBaseFee,
		Mixhash:    common.Hash{},
		Coinbase:   common.Address{},
		Alloc: core.GenesisAlloc{
			depositContractAddr: depositContractAcc,
		},
	}

	for addr, acc := range genesisExecAccounts {
		acc := acc
		if acc.Balance == nil {
			acc.Balance = common.Big0
		}
		genesis.Alloc[addr] = acc
	}

	if chainConfig.CancunTime != nil {
		var beaconRootContractAcc core.GenesisAccount
		if err := json.Unmarshal([]byte(embeddedBeaconRootContract), &beaconRootContractAcc); err != nil {
			panic(err)
		}
		genesis.Alloc[beaconRootContractAddress] = beaconRootContractAcc

		if genesis.Timestamp >= *chainConfig.CancunTime {
			if genesis.BlobGasUsed == nil {
				genesis.BlobGasUsed = new(uint64)
			}
			if genesis.ExcessBlobGas == nil {
				genesis.ExcessBlobGas = new(uint64)
			}
		}
	}

	wrappedGenesis := &ExecutionGenesis{
		Genesis:        genesis,
		DepositAddress: depositContractAddr,
	}

	// Configure consensus
	if err := consensus.Configure(wrappedGenesis); err != nil {
		return nil, err
	}

	wrappedGenesis.Block = genesis.ToBlock()
	wrappedGenesis.Hash = wrappedGenesis.Block.Hash()

	return wrappedGenesis, nil
}

func (genesis *ExecutionGenesis) NetworkID() uint64 {
	return 7
}

func (genesis *ExecutionGenesis) ChainID() uint64 {
	return genesis.Genesis.Config.ChainID.Uint64()
}

func (genesis *ExecutionGenesis) IsPostMerge() bool {
	return genesis.Block.Difficulty().Cmp(genesis.Genesis.Config.TerminalTotalDifficulty) >= 0
}

func (conf *ExecutionGenesis) ToParams(
	depositAddress [20]byte,
) hivesim.Params {
	params := hivesim.Params{
		"HIVE_DEPOSIT_CONTRACT_ADDRESS": common.Address(depositAddress).String(),
		"HIVE_NETWORK_ID":               fmt.Sprintf("%d", conf.NetworkID()),
		"HIVE_CHAIN_ID":                 conf.Genesis.Config.ChainID.String(),
		"HIVE_FORK_HOMESTEAD":           conf.Genesis.Config.HomesteadBlock.String(),
		//"HIVE_FORK_DAO_BLOCK":           conf.Genesis.Config.DAOForkBlock.String(),  // nil error, not used anyway
		"HIVE_FORK_TANGERINE":            conf.Genesis.Config.EIP150Block.String(),
		"HIVE_FORK_SPURIOUS":             conf.Genesis.Config.EIP155Block.String(), // also eip558
		"HIVE_FORK_BYZANTIUM":            conf.Genesis.Config.ByzantiumBlock.String(),
		"HIVE_FORK_CONSTANTINOPLE":       conf.Genesis.Config.ConstantinopleBlock.String(),
		"HIVE_FORK_PETERSBURG":           conf.Genesis.Config.PetersburgBlock.String(),
		"HIVE_FORK_ISTANBUL":             conf.Genesis.Config.IstanbulBlock.String(),
		"HIVE_FORK_MUIRGLACIER":          conf.Genesis.Config.MuirGlacierBlock.String(),
		"HIVE_FORK_BERLIN":               conf.Genesis.Config.BerlinBlock.String(),
		"HIVE_FORK_LONDON":               conf.Genesis.Config.LondonBlock.String(),
		"HIVE_FORK_ARROWGLACIER":         conf.Genesis.Config.ArrowGlacierBlock.String(),
		"HIVE_MERGE_BLOCK_ID":            conf.Genesis.Config.MergeNetsplitBlock.String(),
		"HIVE_TERMINAL_TOTAL_DIFFICULTY": conf.Genesis.Config.TerminalTotalDifficulty.String(),
	}
	if conf.Genesis.Config.ShanghaiTime != nil {
		params["HIVE_SHANGHAI_TIMESTAMP"] = fmt.Sprint(*conf.Genesis.Config.ShanghaiTime)
	}
	if conf.Genesis.Config.CancunTime != nil {
		params["HIVE_CANCUN_TIMESTAMP"] = fmt.Sprint(*conf.Genesis.Config.CancunTime)
	}
	if conf.Genesis.Config.Clique != nil {
		params["HIVE_CLIQUE_PERIOD"] = fmt.Sprint(conf.Genesis.Config.Clique.Period)
	}
	return params
}

func ExecutionBundle(genesis *core.Genesis) (hivesim.StartOption, error) {
	out, err := json.Marshal(genesis)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize genesis state: %v", err)
	}
	return hivesim.WithDynamicFile(
		"genesis.json",
		config.BytesSource(out),
	), nil
}

func ChainBundle(chain []*types.Block) (hivesim.StartOption, error) {
	var buf bytes.Buffer
	for _, block := range chain {
		if err := block.EncodeRLP(&buf); err != nil {
			return nil, err
		}
	}
	return hivesim.WithDynamicFile(
		"/chain.rlp",
		config.BytesSource(buf.Bytes()),
	), nil
}
