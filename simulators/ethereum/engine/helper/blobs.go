package helper

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"

	"github.com/holiman/uint256"
	"github.com/pkg/errors"

	gokzg4844 "github.com/crate-crypto/go-kzg-4844"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
)

var gCryptoCtx gokzg4844.Context
var initCryptoCtx sync.Once

//go:embed trusted_setup.json
var content embed.FS

// InitializeCryptoCtx initializes the global context object returned via CryptoCtx
func InitializeCryptoCtx() {
	initCryptoCtx.Do(func() {
		// Initialize context to match the configurations that the
		// specs are using.
		config, err := content.ReadFile("trusted_setup.json")
		if err != nil {
			panic(err)
		}
		params := new(gokzg4844.JSONTrustedSetup)
		if err = json.Unmarshal(config, params); err != nil {
			panic(err)
		}
		ctx, err := gokzg4844.NewContext4096(params)
		if err != nil {
			panic(fmt.Sprintf("could not create context, err : %v", err))
		}
		gCryptoCtx = *ctx
		// Initialize the precompile return value
	})
}

// CryptoCtx returns a context object stores all of the necessary configurations
// to allow one to create and verify blob proofs.
// This function is expensive to run if the crypto context isn't initialized, so it is recommended to
// pre-initialize by calling InitializeCryptoCtx
func CryptoCtx() gokzg4844.Context {
	InitializeCryptoCtx()
	return gCryptoCtx
}

type BlobID uint64

type BlobIDs []BlobID

func GetBlobList(startId BlobID, count uint64) BlobIDs {
	blobList := make(BlobIDs, count)
	for i := uint64(0); i < count; i++ {
		blobList[i] = startId + BlobID(i)
	}
	return blobList
}

func GetBlobListByIndex(startIndex BlobID, endIndex BlobID) BlobIDs {
	count := uint64(0)
	if endIndex > startIndex {
		count = uint64(endIndex - startIndex + 1)
	} else {
		count = uint64(startIndex - endIndex + 1)
	}
	blobList := make(BlobIDs, count)
	if endIndex > startIndex {
		for i := uint64(0); i < count; i++ {
			blobList[i] = startIndex + BlobID(i)
		}
	} else {
		for i := uint64(0); i < count; i++ {
			blobList[i] = endIndex - BlobID(i)
		}
	}

	return blobList
}

// Blob transaction creator
type BlobTransactionCreator struct {
	To         *common.Address
	GasLimit   uint64
	GasFee     *big.Int
	GasTip     *big.Int
	BlobGasFee *big.Int
	BlobID     BlobID
	BlobCount  uint64
	Value      *big.Int
	Data       []byte
}

var uint256BlsModulus = new(uint256.Int).SetBytes32(gokzg4844.BlsModulus[:])

func (blobId BlobID) VerifyBlob(blob *typ.Blob) (bool, error) {
	if blob == nil {
		return false, errors.New("nil blob")
	}
	if blobId == 0 {
		// Blob zero is empty blob
		emptyFieldElem := [32]byte{}
		for chunkIdx := 0; chunkIdx < typ.FieldElementsPerBlob; chunkIdx++ {
			if !bytes.Equal(blob[chunkIdx*32:(chunkIdx+1)*32], emptyFieldElem[:]) {
				return false, nil
			}
		}
		return true, nil
	}

	// Check the blob against the deterministic data
	// First 32 bytes are the sha256 hash of the blob ID
	var currentU256 *uint256.Int
	{
		blobIdBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(blobIdBytes, uint64(blobId))
		currentHashedBuffer := sha256.Sum256(blobIdBytes)
		currentU256 = new(uint256.Int).SetBytes(currentHashedBuffer[:])
	}

	// Calculate modulus before starting
	currentU256.Mod(currentU256, uint256BlsModulus)
	for chunkIdx := 0; chunkIdx < typ.FieldElementsPerBlob; chunkIdx++ {
		currentBlobFieldElem := new(uint256.Int).SetBytes32(blob[chunkIdx*32 : (chunkIdx+1)*32])
		if currentBlobFieldElem.Cmp(currentU256) != 0 {
			return false, nil
		}
		// Add the current hash
		currentU256.AddMod(currentU256, currentU256, uint256BlsModulus)
	}
	return true, nil
}

func (blobId BlobID) FillBlob(blob *typ.Blob) error {
	if blob == nil {
		return errors.New("nil blob")
	}
	if blobId == 0 {
		// Blob zero is empty blob, so leave as is
		return nil
	}
	// Fill the blob with deterministic data
	// First 32 bytes are the sha256 hash of the blob ID
	var currentU256 *uint256.Int
	{
		blobIdBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(blobIdBytes, uint64(blobId))
		currentHashedBuffer := sha256.Sum256(blobIdBytes)
		currentU256 = new(uint256.Int).SetBytes(currentHashedBuffer[:])
	}

	// Calculate modulus before starting
	currentU256.Mod(currentU256, uint256BlsModulus)
	for chunkIdx := 0; chunkIdx < typ.FieldElementsPerBlob; chunkIdx++ {
		bytes32 := currentU256.Bytes32()
		copy(blob[chunkIdx*32:(chunkIdx+1)*32], bytes32[:])
		// Add the current hash
		currentU256.AddMod(currentU256, currentU256, uint256BlsModulus)
	}

	return nil
}

func (blobId BlobID) GenerateBlobNoKZGCache() (*typ.Blob, *typ.KZGCommitment, *typ.KZGProof, error) {
	blob := typ.Blob{}
	if err := blobId.FillBlob(&blob); err != nil {
		return nil, nil, nil, errors.Wrap(err, "GenerateBlob: Filling Blob")
	}
	ctx_4844 := CryptoCtx()

	kzgCommitment, err := ctx_4844.BlobToKZGCommitment(gokzg4844.Blob(blob), 0)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "GenerateBlob: Generating commitment")
	}
	typesKzgCommitment := typ.KZGCommitment(kzgCommitment)

	proof, err := ctx_4844.ComputeBlobKZGProof(gokzg4844.Blob(blob), kzgCommitment, 1)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "GenerateBlob: Generating proof")
	}
	typesKzgProof := typ.KZGProof(proof)

	return &blob, &typesKzgCommitment, &typesKzgProof, nil
}

//go:embed kzg_commitments.bin
var kzgCommitments []byte

//go:embed kzg_proofs.bin
var kzgProofs []byte

func GetPrecomputedKZG(blobId BlobID) (*typ.KZGCommitment, *typ.KZGProof) {
	if int(blobId) >= len(kzgCommitments)/48 || int(blobId) >= len(kzgProofs)/48 {
		return nil, nil
	}
	var (
		commitment typ.KZGCommitment
		proof      typ.KZGProof
	)
	copy(commitment[:], kzgCommitments[int(blobId)*48:(int(blobId)+1)*48])
	copy(proof[:], kzgProofs[int(blobId)*48:(int(blobId)+1)*48])
	return &commitment, &proof

}

//go:generate go run github.com/ethereum/hive/simulators/ethereum/engine/helper/kzg_precomputer 10000
func (blobId BlobID) GenerateBlob() (*typ.Blob, *typ.KZGCommitment, *typ.KZGProof, error) {
	blob := typ.Blob{}
	if err := blobId.FillBlob(&blob); err != nil {
		return nil, nil, nil, errors.Wrap(err, "GenerateBlob: Filling Blob")
	}
	// Use precomputed KZG commitments and proofs if available
	if preComputedKZGCommitment, preComputedProof := GetPrecomputedKZG(blobId); preComputedKZGCommitment != nil && preComputedProof != nil {
		return &blob, preComputedKZGCommitment, preComputedProof, nil
	} else {
		fmt.Printf("INFO: No precomputed KZG for blob %d\n", blobId)
	}
	ctx_4844 := CryptoCtx()

	kzgCommitment, err := ctx_4844.BlobToKZGCommitment(gokzg4844.Blob(blob), 0)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "GenerateBlob: Generating commitment")
	}
	typesKzgCommitment := typ.KZGCommitment(kzgCommitment)

	proof, err := ctx_4844.ComputeBlobKZGProof(gokzg4844.Blob(blob), kzgCommitment, 1)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "GenerateBlob: Generating proof")
	}
	typesKzgProof := typ.KZGProof(proof)

	return &blob, &typesKzgCommitment, &typesKzgProof, nil
}

func (blobId BlobID) GetVersionedHash(commitmentVersion byte) (common.Hash, error) {
	_, kzgCommitment, _, err := blobId.GenerateBlob()
	if err != nil {
		return common.Hash{}, errors.Wrap(err, "GetVersionedHash")
	}
	if kzgCommitment == nil {
		return common.Hash{}, errors.New("nil kzgCommitment")
	}
	sha256Hash := sha256.Sum256((*kzgCommitment)[:])
	versionedHash := common.BytesToHash(append([]byte{commitmentVersion}, sha256Hash[1:]...))
	return versionedHash, nil
}

func BlobDataGenerator(startBlobId BlobID, blobCount uint64) ([]common.Hash, *typ.BlobTxWrapData, error) {
	blobData := typ.BlobTxWrapData{
		Blobs:       make(typ.Blobs, blobCount),
		Commitments: make([]typ.KZGCommitment, blobCount),
		Proofs:      make(typ.KZGProofs, blobCount),
	}
	for i := uint64(0); i < blobCount; i++ {
		if blob, kzgCommitment, kzgProof, err := (startBlobId + BlobID(i)).GenerateBlob(); err != nil {
			return nil, nil, err
		} else {
			blobData.Blobs[i] = *blob
			blobData.Commitments[i] = *kzgCommitment
			blobData.Proofs[i] = *kzgProof
		}
	}
	var hashes []common.Hash
	for i := 0; i < len(blobData.Commitments); i++ {
		hashes = append(hashes, blobData.Commitments[i].ComputeVersionedHash())
	}
	return hashes, &blobData, nil
}

func (tc *BlobTransactionCreator) MakeTransaction(sender SenderAccount, nonce uint64, blockTimestamp uint64) (typ.Transaction, error) {
	// Need tx wrap data that will pass blob verification
	hashes, blobData, err := BlobDataGenerator(tc.BlobID, tc.BlobCount)
	if err != nil {
		return nil, err
	}

	if tc.To == nil {
		return nil, errors.New("nil to address")
	}

	// Collect fields for transaction
	var (
		address    = *tc.To
		chainID    = uint256.MustFromBig(globals.ChainID)
		gasFeeCap  *uint256.Int
		gasTipCap  *uint256.Int
		value      *uint256.Int
		blobGasFee *uint256.Int
	)
	if tc.GasFee != nil {
		gasFeeCap = uint256.MustFromBig(tc.GasFee)
	} else {
		gasFeeCap = uint256.MustFromBig(globals.GasPrice)
	}
	if tc.GasTip != nil {
		gasTipCap = uint256.MustFromBig(tc.GasTip)
	} else {
		gasTipCap = uint256.MustFromBig(globals.GasTipPrice)
	}
	if tc.Value != nil {
		value = uint256.MustFromBig(tc.Value)
	}
	if tc.BlobGasFee != nil {
		blobGasFee = uint256.MustFromBig(tc.BlobGasFee)
	}

	sbtx := &types.BlobTx{
		ChainID:    chainID,
		Nonce:      nonce,
		GasTipCap:  gasTipCap,
		GasFeeCap:  gasFeeCap,
		Gas:        tc.GasLimit,
		To:         address,
		Value:      value,
		Data:       tc.Data,
		AccessList: nil,
		BlobFeeCap: blobGasFee,
		BlobHashes: hashes,
	}

	key := sender.GetKey()

	signedTx, err := types.SignNewTx(key, types.NewCancunSigner(globals.ChainID), sbtx)
	if err != nil {
		return nil, err
	}
	return &typ.TransactionWithBlobData{
		Tx:       signedTx,
		BlobData: blobData,
	}, nil
}
