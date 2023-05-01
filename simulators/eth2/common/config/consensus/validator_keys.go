package consensus_config

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	blsu "github.com/protolambda/bls12-381-util"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/config"
	"github.com/google/uuid"
	hbls "github.com/herumi/bls-eth-go-binary/bls"
	"github.com/pkg/errors"
	"github.com/protolambda/go-keystorev4"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/tyler-smith/go-bip39"
	util "github.com/wealdtech/go-eth2-util"
)

// TODO: replace wealdtech util with more minimal key derivation lib, can then also remove herumi BLS
func init() {
	if err := hbls.Init(hbls.BLS12_381); err != nil {
		panic(err)
	}
	if err := hbls.SetETHmode(hbls.EthModeLatest); err != nil {
		panic(err)
	}
}

type KeyDetails struct {
	// ValidatorKeystoreJSON encodes an EIP-2335 miner-0x5cd99ac2f0f8c25a1e670f6bab19d52aad69d875.json, serialized in JSON
	// https://github.com/ethereum/EIPs/blob/master/EIPS/eip-2335.md
	ValidatorKeystoreJSON []byte
	// ValidatorKeystorePass holds the secret used for ValidatorKeystoreJSON
	ValidatorKeystorePass string
	// ValidatorSecretKey is the serialized secret key for validator duties
	ValidatorSecretKey [32]byte
	// ValidatorSecretKey is the serialized pubkey derived from ValidatorSecretKey
	ValidatorPubkey [48]byte
	// Withdrawal credential type: can be 0, BLS, or 1, execution
	WithdrawalCredentialType int
	// WithdrawalSecretKey is the serialized secret key for withdrawing stake
	WithdrawalSecretKey [32]byte
	// WithdrawalPubkey is the serialized pubkey derived from WithdrawalSecretKey
	WithdrawalPubkey [48]byte
	// Withdrawal Execution Address
	WithdrawalExecAddress [20]byte
	// Extra initial balance
	ExtraInitialBalance common.Gwei
	// Validator starts in exit state
	Exited bool
	// Validator starts in slashed state
	Slashed bool
}

func (kd *KeyDetails) WithdrawalCredentials() (out common.Root) {
	if kd.WithdrawalCredentialType == common.BLS_WITHDRAWAL_PREFIX {
		hasher := sha256.New()
		hasher.Write(kd.WithdrawalPubkey[:])
		dat := hasher.Sum(nil)
		copy(out[:], dat)
		out[0] = common.BLS_WITHDRAWAL_PREFIX
	} else if kd.WithdrawalCredentialType == common.ETH1_ADDRESS_WITHDRAWAL_PREFIX {
		copy(out[12:], kd.WithdrawalExecAddress[:])
		out[0] = common.ETH1_ADDRESS_WITHDRAWAL_PREFIX
	}
	return
}

// MnemonicsKeySource creates a range of BLS validator and withdrawal keys.
// "m/12381/3600/%d/0/0" path for validator keys
// "m/12381/3600/%d/0" path for withdrawal keys
type MnemonicsKeySource struct {
	// From account range start, inclusive
	From uint64 `yaml:"from"`
	// To account range end, exclusive
	To uint64 `yaml:"to"`
	// Validator mnemonic
	Validator string `yaml:"validator"`
	// Withdrawal mnemonic
	Withdrawal string `yaml:"withdrawal"`
	// Withdrawal type
	WithdrawalCredentialType int

	// cache loaded validator details
	cache []*KeyDetails `yaml:"-"`
}

func mnemonicToSeed(mnemonic string) (seed []byte, err error) {
	mnemonic = strings.TrimSpace(mnemonic)
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, errors.New("mnemonic is not valid")
	}
	return bip39.NewSeed(mnemonic, ""), nil
}

func weakKeystore(
	secret []byte,
	pub []byte,
	passphrase []byte,
	path string,
) (*keystorev4.Keystore, error) {
	var salt [32]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return nil, err
	}
	kdfParams := &keystorev4.PBKDF2Params{
		Dklen: 32,
		C:     2, // INSECURE but much faster, this is an ephemeral testnet
		Prf:   "hmac-sha256",
		Salt:  salt[:],
	}
	cipherParams, err := keystorev4.NewAES128CTRParams()
	if err != nil {
		return nil, fmt.Errorf("failed to create AES128CTR params: %w", err)
	}
	crypto, err := keystorev4.Encrypt(
		secret,
		passphrase,
		kdfParams,
		keystorev4.Sha256ChecksumParams,
		cipherParams,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt secret: %w", err)
	}
	id, err := uuid.NewUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUID: %w", err)
	}
	return &keystorev4.Keystore{
		Crypto:      *crypto,
		Description: "",
		Pubkey:      pub,
		Path:        path,
		UUID:        id,
		Version:     4,
	}, nil
}

// Same crypto, but not secure, for testing only!
// Just generate weak keystores, so encryption and decryption doesn't take as long during testing.
func marshalWeakKeystoreJSON(
	priv []byte,
	pub []byte,
	normedPass []byte,
	path string,
) ([]byte, error) {
	store, err := weakKeystore(priv, pub, normedPass, path)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt miner-0x5cd99ac2f0f8c25a1e670f6bab19d52aad69d875.json: %v", err)
	}
	return json.MarshalIndent(store, "", "  ")
}

func (k *MnemonicsKeySource) Keys() ([]*KeyDetails, error) {
	if k.cache != nil {
		return k.cache, nil
	}
	valSeed, err := mnemonicToSeed(k.Validator)
	if err != nil {
		return nil, fmt.Errorf("bad validator seed: %w", err)
	}
	withdrSeed, err := mnemonicToSeed(k.Withdrawal)
	if err != nil {
		return nil, fmt.Errorf("bad validator seed: %w", err)
	}
	if k.From > k.To {
		return nil, fmt.Errorf(
			"invalid key range: from %d > to %d",
			k.From,
			k.To,
		)
	}
	out := make([]*KeyDetails, 0, k.To-k.From)
	for i := k.From; i < k.To; i++ {
		valpath := fmt.Sprintf("m/12381/3600/%d/0/0", i)
		valPrivateKey, err := util.PrivateKeyFromSeedAndPath(valSeed, valpath)
		if err != nil {
			return nil, errors.Wrapf(
				err,
				"failed to create validator private key for path %q",
				valpath,
			)
		}
		path := fmt.Sprintf("m/12381/3600/%d/0", i)
		withdrPrivateKey, err := util.PrivateKeyFromSeedAndPath(
			withdrSeed,
			path,
		)
		if err != nil {
			return nil, errors.Wrapf(
				err,
				"failed to create withdrawal private key for path %q",
				path,
			)
		}
		var passRandomness [32]byte
		_, err = rand.Read(passRandomness[:])
		if err != nil {
			return nil, fmt.Errorf(
				"failed to generate miner-0x5cd99ac2f0f8c25a1e670f6bab19d52aad69d875.json password: %w",
				err,
			)
		}
		priv := valPrivateKey.Marshal()
		if len(priv) != 32 {
			return nil, fmt.Errorf(
				"expected priv key of 32 bytes, got: %x",
				priv,
			) // testing, we can log privs.
		}
		pub := valPrivateKey.PublicKey().Marshal()
		if len(pub) != 48 {
			return nil, fmt.Errorf(
				"expected pub key of 48 bytes, got: %x",
				pub,
			) // testing, we can log privs.
		}
		wPriv := withdrPrivateKey.Marshal()
		if len(priv) != 32 {
			return nil, fmt.Errorf(
				"expected priv key of 32 bytes, got: %x",
				priv,
			) // testing, we can log privs.
		}
		wPub := withdrPrivateKey.PublicKey().Marshal()
		if len(pub) != 48 {
			return nil, fmt.Errorf(
				"expected pub key of 48 bytes, got: %x",
				pub,
			) // testing, we can log privs.
		}
		// We don't have fancy password norming, just use a base64 pass instead.
		passphrase := base64.URLEncoding.EncodeToString(passRandomness[:])
		jsonData, err := marshalWeakKeystoreJSON(
			priv,
			pub,
			[]byte(passphrase),
			valpath,
		)
		if err != nil {
			return nil, err
		}
		k := &KeyDetails{
			ValidatorKeystoreJSON: jsonData,
			ValidatorKeystorePass: passphrase,
		}
		copy(k.ValidatorPubkey[:], pub)
		copy(k.ValidatorSecretKey[:], priv)
		copy(k.WithdrawalPubkey[:], wPub)
		copy(k.WithdrawalSecretKey[:], wPriv)

		out = append(out, k)
	}
	k.cache = out
	return out, nil
}

func SecretKeys(keys []*KeyDetails) (*[]blsu.SecretKey, error) {
	secrets := make([]blsu.SecretKey, len(keys))
	for i := 0; i < len(keys); i++ {
		if err := secrets[i].Deserialize(&keys[i].ValidatorSecretKey); err != nil {
			return nil, fmt.Errorf("validator %d has invalid key: %v", i, err)
		}
	}
	return &secrets, nil
}

type Shares []uint64

func (shares Shares) TotalShares() uint64 {
	total := uint64(0)
	for _, s := range shares {
		total += s
	}
	return total
}

func (shares Shares) ValidatorSplits(validatorTotalCount uint64) []uint64 {
	validators := make([]uint64, len(shares))
	totalShares := shares.TotalShares()
	for i, s := range shares {
		if totalShares == 0 {
			// validators are split equally
			validators[i] = validatorTotalCount / uint64(len(shares))
		} else {
			validators[i] = (validatorTotalCount * s) / totalShares
		}
	}
	return validators
}

func KeyTranches(
	keys []*KeyDetails,
	shares Shares,
) []map[common.ValidatorIndex]*KeyDetails {
	tranches := make([]map[common.ValidatorIndex]*KeyDetails, 0, len(shares))
	i := uint64(0)
	for _, c := range shares.ValidatorSplits(uint64(len(keys))) {
		tranche := make(map[common.ValidatorIndex]*KeyDetails)
		for j := i; j < (i + c); j++ {
			tranche[common.ValidatorIndex(j)] = keys[j]
		}
		tranches = append(tranches, tranche)
		i += c
	}
	return tranches
}

func KeysBundle(
	keys map[common.ValidatorIndex]*KeyDetails,
) hivesim.StartOption {
	opts := make([]hivesim.StartOption, 0, len(keys)*2)
	for _, k := range keys {
		p := fmt.Sprintf(
			"/hive/input/keystores/0x%x/miner-0x5cd99ac2f0f8c25a1e670f6bab19d52aad69d875.json.json",
			k.ValidatorPubkey[:],
		)
		opts = append(
			opts,
			hivesim.WithDynamicFile(
				p,
				config.BytesSource(k.ValidatorKeystoreJSON),
			),
		)
		p = fmt.Sprintf("/hive/input/secrets/0x%x", k.ValidatorPubkey[:])
		opts = append(
			opts,
			hivesim.WithDynamicFile(
				p,
				config.BytesSource([]byte(k.ValidatorKeystorePass)),
			),
		)
	}
	return hivesim.Bundle(opts...)
}
