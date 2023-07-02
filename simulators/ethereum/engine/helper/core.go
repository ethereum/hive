package helper

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

var (
	_ GenesisAccount = (Account)(nil)
	// _ Genesis        = (*NethermindChainSpec)(nil)
)

type GenesisAlloc interface {

	// representation of a map of address to GenesisAccount, is different from each client.

}

type GenesisAccount interface {
	// Balance holds the balance of the account
	Balance() *big.Int
	SetBalance(balance *big.Int)
	Code() string
	SetCode(code []byte)
	SetConstructor(constructor []byte)
	Constructor() string
}

type Genesis interface {
	Config() *params.ChainConfig
	SetConfig(config *params.ChainConfig)
	Nonce() uint64
	SetNonce(nonce uint64)
	Timestamp() uint64
	SetTimestamp(timestamp int64)
	ExtraData() []byte
	SetExtraData(data []byte)
	GasLimit() uint64
	SetGasLimit(limit uint64)
	Difficulty() *big.Int
	SetDifficulty(difficulty *big.Int)
	MixHash() common.Hash
	SetMixHash(hash common.Hash)
	Coinbase() common.Address
	SetCoinbase(address common.Address)
	Alloc() GenesisAlloc
	AllocGenesis(address common.Address, account Account)
	UpdateTimestamp(timestamp string)

	// Used for testing
	Number() uint64
	GasUsed() uint64
	ParentHash() common.Hash
	BaseFee() *big.Int

	ToBlock() *types.Block

	// Marshalling and Unmarshalling interfaces
	json.Unmarshaler
	json.Marshaler
}

type Builtin struct {
	Name    string                 `json:"name,omitempty"`
	Pricing map[string]interface{} `json:"pricing,omitempty"`
}

type Account map[string]interface{}

func NewAccount() Account {
	return make(Account, 0)
}

// GetCode returns theaccount balance if it was set,
// otherwise returns common.Big0
func (a Account) Balance() *big.Int {
	hexBalance, ok := a["balance"]
	if !ok {
		return common.Big0
	}
	hexStr := hexBalance.(string)
	balance := common.Big0
	_ = balance.FillBytes(common.Hex2Bytes(hexStr))
	return balance
}

func (a Account) SetBalance(balance *big.Int) {
	a["balance"] = common.BigToHash(balance)
}

// GetCode returns the hex representation of code if it was set,
// otherwise returns ""
func (a Account) Code() string {
	code, ok := a["code"]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%x", code.(string))
}

func (a Account) SetCode(code []byte) {
	a["code"] = common.Bytes2Hex(code)
}

// GetConstructor returns the hex representation of constructor if it was set,
// otherwise returns ""
func (a Account) Constructor() string {
	constructor, ok := a["constructor"]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%x", constructor.(string))
}

func (a Account) SetConstructor(constructor []byte) {
	a["constructor"] = common.Bytes2Hex(constructor)
}

type NethermindGenesis struct {
	Seal struct {
		AuthorityRound struct {
			Step      string `json:"step,omitempty"`
			Signature string `json:"signature,omitempty"`
		} `json:"authorityRound,omitempty"`
	} `json:"seal,omitempty"`
	BaseFeePerGas string `json:"baseFeePerGas,omitempty"`
	Difficulty    string `json:"difficulty,omitempty"`
	GasLimit      string `json:"gasLimit,omitempty"`
}

type NethermindParams struct {
	NetworkID                               int    `json:"networkID,omitempty"`
	GasLimitBoundDivisor                    string `json:"gasLimitBoundDivisor,omitempty"`
	MaximumExtraDataSize                    string `json:"maximumExtraDataSize,omitempty"`
	MaxCodeSize                             string `json:"maxCodeSize,omitempty"`
	MaxCodeSizeTransition                   string `json:"maxCodeSizeTransition,omitempty"`
	MinGasLimit                             string `json:"minGasLimit,omitempty"`
	Eip140Transition                        string `json:"eip140Transition,omitempty"`
	Eip211Transition                        string `json:"eip211Transition,omitempty"`
	Eip214Transition                        string `json:"eip214Transition,omitempty"`
	Eip658Transition                        string `json:"eip658Transition,omitempty"`
	Eip145Transition                        string `json:"eip145Transition,omitempty"`
	Eip1014Transition                       string `json:"eip1014Transition,omitempty"`
	Eip1052Transition                       string `json:"eip1052Transition,omitempty"`
	Eip1283Transition                       string `json:"eip1283Transition,omitempty"`
	Eip1344Transition                       string `json:"eip1344Transition,omitempty"`
	Eip1706Transition                       string `json:"eip1706Transition,omitempty"`
	Eip1884Transition                       string `json:"eip1884Transition,omitempty"`
	Eip2028Transition                       string `json:"eip2028Transition,omitempty"`
	Eip2929Transition                       string `json:"eip2929Transition,omitempty"`
	Eip2930Transition                       string `json:"eip2930Transition,omitempty"`
	Eip3198Transition                       string `json:"eip3198Transition,omitempty"`
	Eip3529Transition                       string `json:"eip3529Transition,omitempty"`
	Eip3541Transition                       string `json:"eip3541Transition,omitempty"`
	Eip1559Transition                       string `json:"eip1559Transition,omitempty"`
	Eip4895TransitionTimestamp              string `json:"eip4895TransitionTimestamp,omitempty"`
	Eip3855TransitionTimestamp              string `json:"eip3855TransitionTimestamp,omitempty"`
	Eip3651TransitionTimestamp              string `json:"eip3651TransitionTimestamp,omitempty"`
	Eip3860TransitionTimestamp              string `json:"eip3860TransitionTimestamp,omitempty"`
	Eip1559BaseFeeMaxChangeDenominator      string `json:"eip1559BaseFeeMaxChangeDenominator,omitempty"`
	Eip1559ElasticityMultiplier             string `json:"eip1559ElasticityMultiplier,omitempty"`
	Eip1559FeeCollector                     string `json:"eip1559FeeCollector,omitempty"`
	Eip1559FeeCollectorTransition           int    `json:"eip1559FeeCollectorTransition,omitempty"`
	Registrar                               string `json:"registrar,omitempty"`
	TransactionPermissionContract           string `json:"transactionPermissionContract,omitempty"`
	TransactionPermissionContractTransition string `json:"transactionPermissionContractTransition,omitempty"`
	TerminalTotalDifficulty                 string `json:"terminalTotalDifficulty,omitempty"`
}

type AuthorityParams struct {
	StepDuration                int    `json:"stepDuration,omitempty"`
	BlockReward                 string `json:"blockReward,omitempty"`
	MaximumUncleCountTransition int    `json:"maximumUncleCountTransition,omitempty"`
	MaximumUncleCount           int    `json:"maximumUncleCount,omitempty"`
	Validators                  struct {
		Multi map[string]map[string][]string `json:"multi,omitempty"`
	} `json:"validators,omitempty"`
	BlockRewardContractAddress       string            `json:"blockRewardContractAddress,omitempty"`
	BlockRewardContractTransition    int               `json:"blockRewardContractTransition,omitempty"`
	RandomnessContractAddress        map[string]string `json:"randomnessContractAddress,omitempty"`
	WithdrawalContractAddress        string            `json:"withdrawalContractAddress,omitempty"`
	TwoThirdsMajorityTransition      int               `json:"twoThirdsMajorityTransition,omitempty"`
	PosdaoTransition                 int               `json:"posdaoTransition,omitempty"`
	BlockGasLimitContractTransitions map[string]string `json:"blockGasLimitContractTransitions,omitempty"`
}

type NethermindEngine struct {
	AuthorityRound struct {
		Params AuthorityParams `json:"params,omitempty"`
	} `json:"authorityRound,omitempty"`
}

type NethermindChainSpec struct {
	Name     string             `json:"name,omitempty"`
	Engine   NethermindEngine   `json:"engine,omitempty"`
	Params   NethermindParams   `json:"params,omitempty"`
	Genesis  NethermindGenesis  `json:"genesis,omitempty"`
	Accounts map[string]Account `json:"accounts,omitempty"`
}

func (n *NethermindChainSpec) UpdateTimestamp(timestamp string) {
	n.Params.Eip3651TransitionTimestamp = timestamp
	n.Params.Eip3855TransitionTimestamp = timestamp
	n.Params.Eip3860TransitionTimestamp = timestamp
	n.Params.Eip4895TransitionTimestamp = timestamp
}

func (n *NethermindChainSpec) Config() *params.ChainConfig {
	chainID := big.NewInt(int64(n.Params.NetworkID))
	ttd, err := strconv.ParseInt(n.Params.TerminalTotalDifficulty, 10, 64)
	if err != nil {
		panic(err)
	}
	unixTimestampUint64, err := strconv.ParseUint(n.Params.Eip4895TransitionTimestamp[2:], 16, 64)
	if err != nil {
		fmt.Println("Error parsing hexadecimal timestamp:", err)
		return nil
	}

	return &params.ChainConfig{
		ChainID:                 chainID,
		TerminalTotalDifficulty: big.NewInt(ttd),
		ShanghaiTime:            &unixTimestampUint64,
	}
}

func (n *NethermindChainSpec) SetConfig(config *params.ChainConfig) {
	//TODO implement me
	panic("implement me")
}

func (n *NethermindChainSpec) Nonce() uint64 {
	//TODO implement me
	panic("implement me")
}

func (n *NethermindChainSpec) SetNonce(nonce uint64) {
	//TODO implement me
	panic("implement me")
}

func (n *NethermindChainSpec) Timestamp() uint64 {
	//TODO implement me
	panic("implement me")
}

func (n *NethermindChainSpec) SetTimestamp(timestamp int64) {
	//n.Params.TerminalTotalDifficulty = fmt.Sprintf("%v", timestamp)
	n.Params.Eip3651TransitionTimestamp = fmt.Sprintf("%#x", timestamp)
	n.Params.Eip4895TransitionTimestamp = fmt.Sprintf("%#x", timestamp)
	n.Params.Eip3855TransitionTimestamp = fmt.Sprintf("%#x", timestamp)
	n.Params.Eip3651TransitionTimestamp = fmt.Sprintf("%#x", timestamp)
	n.Params.Eip3860TransitionTimestamp = fmt.Sprintf("%#x", timestamp)
}

func (n *NethermindChainSpec) ExtraData() []byte {
	//TODO implement me
	panic("implement me")
}

func (n *NethermindChainSpec) SetExtraData(data []byte) {
	//TODO implement me
	panic("implement me")
}

func (n *NethermindChainSpec) GasLimit() uint64 {
	return common.HexToHash(n.Genesis.GasLimit).Big().Uint64()
}

func (n *NethermindChainSpec) SetGasLimit(limit uint64) {
	//TODO implement me
	panic("implement me")
}

func (n *NethermindChainSpec) Difficulty() *big.Int {
	return common.HexToHash(n.Genesis.Difficulty).Big()
}

func (n *NethermindChainSpec) SetDifficulty(difficulty *big.Int) {
	n.Genesis.Difficulty = common.BigToHash(difficulty).Hex()
}

func (n *NethermindChainSpec) MixHash() common.Hash {
	return common.Hash{}
}

func (n *NethermindChainSpec) SetMixHash(hash common.Hash) {
}

func (n *NethermindChainSpec) Coinbase() common.Address {
	return common.Address{}
}

func (n *NethermindChainSpec) SetCoinbase(address common.Address) {
	//TODO implement me
	panic("implement me")
}

func (n *NethermindChainSpec) Alloc() GenesisAlloc {
	//TODO implement me
	panic("implement me")
}

func (n *NethermindChainSpec) AllocGenesis(address common.Address, account Account) {
	n.Accounts[address.Hex()] = account
}

func (n *NethermindChainSpec) Number() uint64 {
	//TODO implement me
	panic("implement me")
}

func (n *NethermindChainSpec) GasUsed() uint64 {
	//TODO implement me
	panic("implement me")
}

func (n *NethermindChainSpec) ParentHash() common.Hash {
	//TODO implement me
	panic("implement me")
}

func (n *NethermindChainSpec) BaseFee() *big.Int {
	//TODO implement me
	panic("implement me")
}

func (n *NethermindChainSpec) ToBlock() *types.Block {
	//TODO implement me
	panic("implement me")
}
