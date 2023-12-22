package taiko

import (
	"crypto/ecdsa"
	"encoding/json"
	"io/ioutil"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/taikoxyz/taiko-client/bindings"
)

type taikoConfig struct {
	L1NetworkID    uint64 `json:"l1_network_id"`
	L1CliquePeriod uint64 `json:"l1_clique_period"`
	DeployPrivKey  string `json:"deploy_private_key"`
	ProverPrivKey  string `json:"prover_private_key"`
	JWTSecret      string `json:"jwt_secret"`
}

func DefaultConfig() (*Config, error) {
	data, err := ioutil.ReadFile("config.json")
	if err != nil {
		return nil, err
	}
	tc := new(taikoConfig)
	if err := json.Unmarshal(data, tc); err != nil {
		return nil, err
	}
	deployAccount, err := NewAccount(tc.DeployPrivKey)
	if err != nil {
		return nil, err
	}
	proverAccount, err := NewAccount(tc.ProverPrivKey)
	if err != nil {
		return nil, err
	}
	throwawayAccount, err := NewAccount(bindings.GoldenTouchPrivKey[2:])
	if err != nil {
		return nil, err
	}
	return &Config{
		L1: &L1Config{
			ChainID:      big.NewInt(int64(tc.L1NetworkID)),
			NetworkID:    tc.L1NetworkID,
			Deployer:     deployAccount,
			CliquePeriod: tc.L1CliquePeriod,
		},
		L2: &L2Config{
			ChainID:   params.TaikoAlpha1NetworkID,
			NetworkID: params.TaikoAlpha1NetworkID.Uint64(),
			JWTSecret: tc.JWTSecret,

			Proposer:              deployAccount,
			ProposeInterval:       time.Second,
			SuggestedFeeRecipient: deployAccount,

			Prover: proverAccount,

			Throwawayer: throwawayAccount,
		},
	}, nil
}

type Account struct {
	PrivateKeyHex string
	PrivateKey    *ecdsa.PrivateKey
	Address       common.Address
}

func NewAccount(privKeyHex string) (*Account, error) {
	privKey, err := crypto.HexToECDSA(privKeyHex)
	if err != nil {
		return nil, err
	}
	addr := crypto.PubkeyToAddress(privKey.PublicKey)
	return &Account{
		PrivateKeyHex: privKeyHex,
		PrivateKey:    privKey,
		Address:       addr,
	}, nil
}

type L1Config struct {
	ChainID      *big.Int
	NetworkID    uint64
	Deployer     *Account
	CliquePeriod uint64
}

type L2Config struct {
	ChainID   *big.Int
	NetworkID uint64

	Throwawayer           *Account // L2 driver account for throwaway invalid block
	SuggestedFeeRecipient *Account // suggested fee recipient account
	Prover                *Account // L1 prover account for prove zk proof
	Proposer              *Account // L1 proposer account for propose L1 txList

	ProposeInterval time.Duration
	JWTSecret       string
}

type Config struct {
	L1 *L1Config
	L2 *L2Config
}
