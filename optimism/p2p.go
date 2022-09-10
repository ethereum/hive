package optimism

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"github.com/ethereum-optimism/optimism/op-node/cmd/p2p"
)

func asMultiAddr(ip string, privKey *ecdsa.PrivateKey, port int) (string, error) {
	keyB := []byte(hex.EncodeToString(EncodePrivKey(privKey)))
	peerID, err := p2p.Priv2PeerID(bytes.NewReader(keyB))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("/ip4/%s/tcp/%d/p2p/%s", ip, port, peerID), nil
}
