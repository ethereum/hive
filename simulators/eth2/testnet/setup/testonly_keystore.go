// Copyright © 2019 Weald Technology Trading
// Licensed under the Apache License, Version 2.0.
//
// Original: https://github.com/wealdtech/go-eth2-wallet-encryptor-keystorev4
// This is a minimal copy, allowing modification of the security parameters, for testing purposes.

package setup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/scrypt"
)

const (
	// Scrypt parameters
	scryptr      = 8
	scryptp      = 1
	scryptKeyLen = 32

	// PBKDF2 parameters
	pbkdf2KeyLen = 32
	pbkdf2PRF    = "hmac-sha256"
)

type ksKDFParams struct {
	// Shared parameters
	Salt  string `json:"salt"`
	DKLen int    `json:"dklen"`
	// Scrypt-specific parameters
	N int `json:"n,omitempty"`
	P int `json:"p,omitempty"`
	R int `json:"r,omitempty"`
	// PBKDF2-specific parameters
	C   int    `json:"c,omitempty"`
	PRF string `json:"prf,omitempty"`
}
type ksKDF struct {
	Function string       `json:"function"`
	Params   *ksKDFParams `json:"params"`
	Message  string       `json:"message"`
}
type ksChecksum struct {
	Function string                 `json:"function"`
	Params   map[string]interface{} `json:"params"`
	Message  string                 `json:"message"`
}
type ksCipherParams struct {
	// AES-128-CTR-specific parameters
	IV string `json:"iv,omitempty"`
}
type ksCipher struct {
	Function string          `json:"function"`
	Params   *ksCipherParams `json:"params"`
	Message  string          `json:"message"`
}
type keystoreV4 struct {
	KDF      *ksKDF      `json:"kdf"`
	Checksum *ksChecksum `json:"checksum"`
	Cipher   *ksCipher   `json:"cipher"`
}

func encrypt(secret []byte, normedPassphrase []byte, cipherType string, iterN int) (*keystoreV4, error) {
	if secret == nil {
		return nil, errors.New("no secret")
	}

	// Random salt
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}

	// Create the decryption key
	var decryptionKey []byte
	var err error
	switch cipherType {
	case "scrypt":
		decryptionKey, err = scrypt.Key(normedPassphrase, salt, iterN, scryptr, scryptp, scryptKeyLen)
	case "pbkdf2":
		decryptionKey = pbkdf2.Key(normedPassphrase, salt, iterN, pbkdf2KeyLen, sha256.New)
	default:
		return nil, fmt.Errorf("unknown cipher %q", cipherType)
	}
	if err != nil {
		return nil, err
	}

	// Generate the cipher message
	cipherMsg := make([]byte, len(secret))
	aesCipher, err := aes.NewCipher(decryptionKey[:16])
	if err != nil {
		return nil, err
	}
	// Random IV
	iv := make([]byte, 16)
	if _, err := rand.Read(iv); err != nil {
		return nil, err
	}
	stream := cipher.NewCTR(aesCipher, iv)
	stream.XORKeyStream(cipherMsg, secret)

	// Generate the checksum
	h := sha256.New()
	if _, err := h.Write(decryptionKey[16:32]); err != nil {
		return nil, err
	}
	if _, err := h.Write(cipherMsg); err != nil {
		return nil, err
	}
	checksumMsg := h.Sum(nil)

	var kdf *ksKDF
	switch cipherType {
	case "scrypt":
		kdf = &ksKDF{
			Function: "scrypt",
			Params: &ksKDFParams{
				DKLen: scryptKeyLen,
				N:     iterN,
				P:     scryptp,
				R:     scryptr,
				Salt:  hex.EncodeToString(salt),
			},
			Message: "",
		}
	case "pbkdf2":
		kdf = &ksKDF{
			Function: "pbkdf2",
			Params: &ksKDFParams{
				DKLen: pbkdf2KeyLen,
				C:     iterN,
				PRF:   pbkdf2PRF,
				Salt:  hex.EncodeToString(salt),
			},
			Message: "",
		}
	}

	// Build the output
	return &keystoreV4{
		KDF: kdf,
		Checksum: &ksChecksum{
			Function: "sha256",
			Params:   make(map[string]interface{}),
			Message:  hex.EncodeToString(checksumMsg),
		},
		Cipher: &ksCipher{
			Function: "aes-128-ctr",
			Params: &ksCipherParams{
				IV: hex.EncodeToString(iv),
			},
			Message: hex.EncodeToString(cipherMsg),
		},
	}, nil
}