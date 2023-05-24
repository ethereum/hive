package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"math/big"
	"net"

	gcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/muxer/mplex"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/prysmaticlabs/prysm/v3/beacon-chain/p2p/encoder"
	"github.com/sirupsen/logrus"
)

var sszNetworkEncoder = encoder.SszNetworkEncoder{}

type TestP2P struct {
	Host       host.Host
	PubSub     *pubsub.PubSub
	PrivateKey crypto.PrivKey
	PublicKey  crypto.PubKey
	LocalNode  *enode.LocalNode
	Digest     [4]byte
}

func createLocalNode(
	privKey *ecdsa.PrivateKey,
	ipAddr net.IP,
	udpPort, tcpPort int,
) (*enode.LocalNode, error) {
	db, err := enode.OpenDB("")
	if err != nil {
		return nil, err
	}
	localNode := enode.NewLocalNode(db, privKey)

	ipEntry := enr.IP(ipAddr)
	udpEntry := enr.UDP(udpPort)
	tcpEntry := enr.TCP(tcpPort)
	localNode.Set(ipEntry)
	localNode.Set(udpEntry)
	localNode.Set(tcpEntry)
	localNode.SetFallbackIP(ipAddr)
	localNode.SetFallbackUDP(udpPort)

	return localNode, nil
}

func ConvertFromInterfacePrivKey(privkey crypto.PrivKey) (*ecdsa.PrivateKey, error) {
	secpKey, ok := privkey.(*crypto.Secp256k1PrivateKey)
	if !ok {
		return nil, fmt.Errorf("could not cast to Secp256k1PrivateKey")
	}
	rawKey, err := secpKey.Raw()
	if err != nil {
		return nil, err
	}
	privKey := new(ecdsa.PrivateKey)
	k := new(big.Int).SetBytes(rawKey)
	privKey.D = k
	privKey.Curve = gcrypto.S256() // Temporary hack, so libp2p Secp256k1 is recognized as geth Secp256k1 in disc v5.1.
	privKey.X, privKey.Y = gcrypto.S256().ScalarBaseMult(rawKey)
	return privKey, nil
}

func NewTestP2P(ip net.IP, port int64) (*TestP2P, error) {
	ctx := context.Background()

	// Generate a new private key pair for this host.
	priv, pub, err := crypto.GenerateSecp256k1Key(rand.Reader)
	if err != nil {
		return nil, err
	}

	libp2pOptions := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%d", ip.String(), port)), libp2p.UserAgent("HiveSim/0.1.0"),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Muxer("/mplex/6.7.0", mplex.DefaultTransport),
		libp2p.DefaultMuxers,
		libp2p.Security(noise.ID, noise.New),
		libp2p.Ping(false),
		libp2p.Identity(priv),
	}

	h, err := libp2p.New(libp2pOptions...)
	if err != nil {
		return nil, err
	}

	ps, err := pubsub.NewGossipSub(ctx, h,
		pubsub.WithMessageSigning(false),
		pubsub.WithStrictSignatureVerification(false),
	)
	if err != nil {
		return nil, err
	}

	pk, err := ConvertFromInterfacePrivKey(priv)
	if err != nil {
		return nil, err
	}
	localNode, err := createLocalNode(pk, ip, int(port), int(port))
	if err != nil {
		return nil, err
	}

	return &TestP2P{
		Host:       h,
		PubSub:     ps,
		PrivateKey: priv,
		PublicKey:  pub,
		LocalNode:  localNode,
	}, nil
}

func (p *TestP2P) SetupStreams() error {
	protocolID := protocol.ID("/eth2/beacon_chain/req/status/1/" + encoder.ProtocolSuffixSSZSnappy)
	buffer_size := 1048576
	p.Host.SetStreamHandler(protocolID, func(s network.Stream) {
		log.WithFields(logrus.Fields{
			"protocol": s.Protocol(),
			"peer":     s.Conn().RemotePeer().String(),
		}).Info("Got a new stream")
		// Read the incoming connection into the buffer.
		buf := make([]byte, buffer_size)
		n, err := s.Read(buf)
		if err != nil {
			log.WithError(err).Error("Failed to read from stream")
			return
		}
		// Log received data in hexadecimal
		log.WithFields(logrus.Fields{
			"protocol": s.Protocol(),
			"peer":     s.Conn().RemotePeer().String(),
			"length":   n,
			"data":     fmt.Sprintf("%x", buf[:n]),
		}).Info("Received data")
		s.Close()
	})
	return nil
}

func (p *TestP2P) Close() error {
	return p.Host.Close()
}
