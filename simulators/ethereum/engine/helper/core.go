package helper

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
	"strconv"
)

type GenesisAlloc interface {

	// representation of a map of address to GenesisAccount, is different from each client.

}

type GenesisAccount interface {
	// Balance holds the balance of the account
	Balance() *big.Int
	SetBalance()
	Code() []byte
	SetCode(code []byte)
	SetConstructor(constructor []byte)
	Constructor() []byte
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
	AllocGenesis(address common.Address, account GenesisAccount)
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

//type PricingStruct struct {
//	Price map[string]map[string]uint64 `json:"price,omitempty"`
//}

//type Pricing map[string]PricingStruct

type Builtin struct {
	Name    string                 `json:"name,omitempty"`
	Pricing map[string]interface{} `json:"pricing,omitempty"`
}

type Account map[string]interface{}

//struct {
//	Balance     string  `json:"balance,omitempty"`
//	Constructor string  `json:"constructor,omitempty"`
//	Builtin     Builtin `json:"builtin,omitempty"`
//}

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
	shangai := big.NewInt(0).SetBytes(common.Hex2Bytes(n.Params.Eip4895TransitionTimestamp)).Uint64()
	return &params.ChainConfig{
		ChainID:                 chainID,
		TerminalTotalDifficulty: big.NewInt(ttd),
		ShanghaiTime:            &shangai,
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

func (n *NethermindChainSpec) AllocGenesis(address common.Address, account GenesisAccount) {
	//TODO implement me
	panic("implement me")
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

type ErigonAura struct {
	StepDuration                int `json:"stepDuration"`
	BlockReward                 int `json:"blockReward"`
	MaximumUncleCountTransition int `json:"maximumUncleCountTransition"`
	MaximumUncleCount           int `json:"maximumUncleCount"`
	Validators                  struct {
		Multi map[string]map[string][]string `json:"multi,omitempty"`
	} `json:"validators"`
	BlockRewardContractAddress       string            `json:"blockRewardContractAddress"`
	BlockRewardContractTransition    int               `json:"blockRewardContractTransition"`
	RandomnessContractAddress        map[string]string `json:"randomnessContractAddress"`
	PosdaoTransition                 int               `json:"posdaoTransition"`
	BlockGasLimitContractTransitions map[string]string `json:"blockGasLimitContractTransitions"`
	Registrar                        string            `json:"registrar"`
}

type ErigonConfig struct {
	ChainName                     string     `json:"ChainName"`
	ChainID                       int        `json:"chainId"`
	Consensus                     string     `json:"consensus"`
	HomesteadBlock                int        `json:"homesteadBlock"`
	Eip150Block                   int        `json:"eip150Block"`
	Eip155Block                   int        `json:"eip155Block"`
	ByzantiumBlock                int        `json:"byzantiumBlock"`
	ConstantinopleBlock           int        `json:"constantinopleBlock"`
	PetersburgBlock               int        `json:"petersburgBlock"`
	IstanbulBlock                 int        `json:"istanbulBlock"`
	BerlinBlock                   int        `json:"berlinBlock"`
	LondonBlock                   int        `json:"londonBlock"`
	Eip1559FeeCollectorTransition int        `json:"eip1559FeeCollectorTransition"`
	Eip1559FeeCollector           string     `json:"eip1559FeeCollector"`
	TerminalTotalDifficulty       *big.Int   `json:"terminalTotalDifficulty"`
	TerminalTotalDifficultyPassed bool       `json:"terminalTotalDifficultyPassed"`
	ShanghaiTimestamp             *big.Int   `json:"shanghaiTime"`
	Aura                          ErigonAura `json:"aura"`
}

type ErigonAccount struct {
	Balance     string `json:"balance"`
	Constructor string `json:"constructor,omitempty"`
}

type ErigonGenesis struct {
	ErigonConfig     ErigonConfig             `json:"config"`
	ErigonTimestamp  *big.Int                 `json:"timestamp"`
	AuRaSeal         string                   `json:"auRaSeal"`
	ErigonGasLimit   string                   `json:"gasLimit"`
	ErigonDifficulty string                   `json:"difficulty"`
	ErigonAlloc      map[string]ErigonAccount `json:"alloc"`
}

func (v *ErigonGenesis) Config() *params.ChainConfig {
	chainID := big.NewInt(int64(v.ErigonConfig.ChainID))
	ttd := big.NewInt(0).SetBytes(common.Hex2Bytes(v.ErigonDifficulty))
	shangai := big.NewInt(0).SetBytes(common.Hex2Bytes(v.ErigonTimestamp)).Uint64()
	return &params.ChainConfig{
		ChainID:                 chainID,
		TerminalTotalDifficulty: ttd,
		ShanghaiTime:            &shangai,
	}
}

func (v *ErigonGenesis) SetConfig(config *params.ChainConfig) {
	//TODO implement me
	panic("implement me")
}

func (v *ErigonGenesis) Nonce() uint64 {
	//TODO implement me
	panic("implement me")
}

func (v *ErigonGenesis) SetNonce(nonce uint64) {
	//TODO implement me
	panic("implement me")
}

func (v *ErigonGenesis) Timestamp() uint64 {
	//TODO implement me
	panic("implement me")
}

func (v *ErigonGenesis) SetTimestamp(timestamp int64) {
	v.ErigonConfig.ShanghaiTimestamp = big.NewInt(timestamp)
}

func (v *ErigonGenesis) ExtraData() []byte {
	//TODO implement me
	panic("implement me")
}

func (v *ErigonGenesis) SetExtraData(data []byte) {
	//TODO implement me
	panic("implement me")
}

func (v *ErigonGenesis) GasLimit() uint64 {
	return big.NewInt(0).SetBytes(common.Hex2Bytes(v.ErigonGasLimit)).Uint64()
}

func (v *ErigonGenesis) SetGasLimit(limit uint64) {
	//TODO implement me
	panic("implement me")
}

func (v *ErigonGenesis) Difficulty() *big.Int {
	return big.NewInt(0).SetBytes(common.Hex2Bytes(v.ErigonDifficulty))
}

func (v *ErigonGenesis) SetDifficulty(difficulty *big.Int) {
	v.ErigonDifficulty = common.BigToHash(difficulty).Hex()
}

func (v *ErigonGenesis) MixHash() common.Hash {
	//TODO implement me
	panic("implement me")
}

func (v *ErigonGenesis) SetMixHash(hash common.Hash) {
	//TODO implement me
	panic("implement me")
}

func (v *ErigonGenesis) Coinbase() common.Address {
	//TODO implement me
	panic("implement me")
}

func (v *ErigonGenesis) SetCoinbase(address common.Address) {
	//TODO implement me
	panic("implement me")
}

func (v *ErigonGenesis) Alloc() GenesisAlloc {
	//TODO implement me
	panic("implement me")
}

func (v *ErigonGenesis) AllocGenesis(address common.Address, account GenesisAccount) {
	//TODO implement me
	panic("implement me")
}

func (v *ErigonGenesis) UpdateTimestamp(timestamp string) {
	v.ErigonTimestamp = timestamp
}

func (v *ErigonGenesis) Number() uint64 {
	//TODO implement me
	panic("implement me")
}

func (v *ErigonGenesis) GasUsed() uint64 {
	//TODO implement me
	panic("implement me")
}

func (v *ErigonGenesis) ParentHash() common.Hash {
	//TODO implement me
	panic("implement me")
}

func (v *ErigonGenesis) BaseFee() *big.Int {
	//TODO implement me
	panic("implement me")
}

func (v *ErigonGenesis) ToBlock() *types.Block {
	//TODO implement me
	panic("implement me")
}
