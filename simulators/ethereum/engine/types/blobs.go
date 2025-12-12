package types

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	gokzg4844 "github.com/crate-crypto/go-kzg-4844"
	beacon "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// Blob Types

const (
	BlobCommitmentVersionKZG uint8 = 0x01
	FieldElementsPerBlob     int   = 4096
)

type KZGCommitment [48]byte

func (p KZGCommitment) MarshalText() ([]byte, error) {
	return []byte("0x" + hex.EncodeToString(p[:])), nil
}

func (p KZGCommitment) String() string {
	return "0x" + hex.EncodeToString(p[:])
}

func (p *KZGCommitment) UnmarshalText(text []byte) error {
	return hexutil.UnmarshalFixedText("KZGCommitment", text, p[:])
}

// KZGToVersionedHash implements kzg_to_versioned_hash from EIP-4844
func KZGToVersionedHash(kzg gokzg4844.KZGCommitment) common.Hash {
	h := sha256.Sum256(kzg[:])
	h[0] = BlobCommitmentVersionKZG

	return h
}

func (c KZGCommitment) ComputeVersionedHash() common.Hash {
	return common.Hash(KZGToVersionedHash(gokzg4844.KZGCommitment(c)))
}

type KZGProof [48]byte

func (p KZGProof) MarshalText() ([]byte, error) {
	return []byte("0x" + hex.EncodeToString(p[:])), nil
}

func (p KZGProof) String() string {
	return "0x" + hex.EncodeToString(p[:])
}

func (p *KZGProof) UnmarshalText(text []byte) error {
	return hexutil.UnmarshalFixedText("KZGProof", text, p[:])
}

type BLSFieldElement [32]byte

func (p BLSFieldElement) String() string {
	return "0x" + hex.EncodeToString(p[:])
}

func (p *BLSFieldElement) UnmarshalText(text []byte) error {
	return hexutil.UnmarshalFixedText("BLSFieldElement", text, p[:])
}

type Blob [FieldElementsPerBlob * 32]byte

func (blob *Blob) MarshalText() ([]byte, error) {
	out := make([]byte, 2+FieldElementsPerBlob*32*2)
	copy(out[:2], "0x")
	hex.Encode(out[2:], blob[:])

	return out, nil
}

func (blob *Blob) String() string {
	v, err := blob.MarshalText()
	if err != nil {
		return "<invalid-blob>"
	}
	return string(v)
}

func (blob *Blob) UnmarshalText(text []byte) error {
	if blob == nil {
		return errors.New("cannot decode text into nil Blob")
	}
	l := 2 + FieldElementsPerBlob*32*2
	if len(text) != l {
		return fmt.Errorf("expected %d characters but got %d", l, len(text))
	}
	if !(text[0] == '0' && text[1] == 'x') {
		return fmt.Errorf("expected '0x' prefix in Blob string")
	}
	if _, err := hex.Decode(blob[:], text[2:]); err != nil {
		return fmt.Errorf("blob is not formatted correctly: %v", err)
	}

	return nil
}

type BlobKzgs []KZGCommitment

type KZGProofs []KZGProof

type Blobs []Blob

type BlobTxWrapData struct {
	Blobs       Blobs
	Commitments BlobKzgs
	Proofs      KZGProofs
}

// BlobsBundle holds the blobs of an execution payload
type BlobsBundle struct {
	Commitments []KZGCommitment `json:"commitments" gencodec:"required"`
	Blobs       []Blob          `json:"blobs"       gencodec:"required"`
	Proofs      []KZGProof      `json:"proofs"      gencodec:"required"`
}

func (bb *BlobsBundle) FromBeaconBlobsBundle(src *beacon.BlobsBundle) error {
	if src == nil {
		return errors.New("nil blobs bundle")
	}
	if src.Commitments == nil {
		return errors.New("nil commitments")
	}
	if src.Blobs == nil {
		return errors.New("nil blobs")
	}
	if src.Proofs == nil {
		return errors.New("nil proofs")
	}
	bb.Commitments = make([]KZGCommitment, len(src.Commitments))
	bb.Blobs = make([]Blob, len(src.Blobs))
	bb.Proofs = make([]KZGProof, len(src.Proofs))
	for i, commitment := range src.Commitments {
		copy(bb.Commitments[i][:], commitment[:])
	}
	for i, blob := range src.Blobs {
		copy(bb.Blobs[i][:], blob[:])
	}
	for i, proof := range src.Proofs {
		copy(bb.Proofs[i][:], proof[:])
	}
	return nil
}

func (bb *BlobsBundle) VersionedHashes(commitmentVersion byte) (*[]common.Hash, error) {
	if bb == nil {
		return nil, errors.New("nil blob bundle")
	}
	if bb.Commitments == nil {
		return nil, errors.New("nil commitments")
	}
	versionedHashes := make([]common.Hash, len(bb.Commitments))
	for i, commitment := range bb.Commitments {
		sha256Hash := sha256.Sum256(commitment[:])
		versionedHashes[i] = common.BytesToHash(append([]byte{commitmentVersion}, sha256Hash[1:]...))
	}
	return &versionedHashes, nil
}
