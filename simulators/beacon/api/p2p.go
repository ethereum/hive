package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"strings"
	"time"

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
	"github.com/protolambda/zrnt/eth2/beacon/common"
	ssz "github.com/prysmaticlabs/fastssz"
	"github.com/prysmaticlabs/prysm/v4/beacon-chain/p2p/encoder"
	"github.com/prysmaticlabs/prysm/v4/consensus-types/primitives"
	ethpb "github.com/prysmaticlabs/prysm/v4/proto/prysm/v1alpha1"
	"github.com/sirupsen/logrus"
)

var sszNetworkEncoder = encoder.SszNetworkEncoder{}

type Goodbye = primitives.SSZUint64

type PingData = primitives.SSZUint64

type TestP2P struct {
	Host       host.Host
	PubSub     *pubsub.PubSub
	PrivateKey crypto.PrivKey
	PublicKey  crypto.PubKey
	LocalNode  *enode.LocalNode
	Digest     [4]byte

	BeaconAPITest *BeaconAPITest
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

func NewTestP2P(beaconAPITest *BeaconAPITest, ip net.IP, port int64) (*TestP2P, error) {
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
		Host:          h,
		PubSub:        ps,
		PrivateKey:    priv,
		PublicKey:     pub,
		LocalNode:     localNode,
		BeaconAPITest: beaconAPITest,
	}, nil
}

func (p *TestP2P) WaitForP2PConnection(ctx context.Context) error {
	// TODO: Actually wait for connection
	if len(p.Host.Network().Peers()) > 0 {
		return nil
	}
	return errors.New("no peers connected")
}

func (p *TestP2P) SetupStreams() error {
	// Prepare stream responses for the basic Req/Resp protocols.

	// Status
	protocolID := protocol.ID("/eth2/beacon_chain/req/status/1/" + encoder.ProtocolSuffixSSZSnappy)
	p.Host.SetStreamHandler(protocolID, func(s network.Stream) {
		// Read the incoming message into the appropriate struct.
		var out Status
		if err := sszNetworkEncoder.DecodeWithMaxLength(s, &out); err != nil {
			log.WithError(err).Error("Failed to decode incoming message")
			return
		}
		// Log received data
		log.WithFields(logrus.Fields{
			"protocol":        s.Protocol(),
			"peer":            s.Conn().RemotePeer().String(),
			"fork_digest":     fmt.Sprintf("%x", out.ForkDigest),
			"finalized_root":  fmt.Sprintf("%x", out.FinalizedRoot),
			"finalized_epoch": fmt.Sprintf("%d", out.FinalizedEpoch),
			"head_root":       fmt.Sprintf("%x", out.HeadRoot),
			"head_slot":       fmt.Sprintf("%d", out.HeadSlot),
		}).Info("Received data")

		// Construct response
		resp := Status{
			ForkDigest:     p.BeaconAPITest.State.CurrentForkDigest[:],
			FinalizedRoot:  p.BeaconAPITest.State.FinalizedCheckpoint.Root[:],
			FinalizedEpoch: uint64(p.BeaconAPITest.State.FinalizedCheckpoint.Epoch),
			HeadRoot:       p.BeaconAPITest.State.CurrentHead.Root[:],
			HeadSlot:       uint64(p.BeaconAPITest.State.CurrentSlot),
		}

		// Log received data
		log.WithFields(logrus.Fields{
			"protocol":        s.Protocol(),
			"peer":            s.Conn().RemotePeer().String(),
			"fork_digest":     fmt.Sprintf("%x", resp.ForkDigest),
			"finalized_root":  fmt.Sprintf("%x", resp.FinalizedRoot),
			"finalized_epoch": fmt.Sprintf("%d", resp.FinalizedEpoch),
			"head_root":       fmt.Sprintf("%x", resp.HeadRoot),
			"head_slot":       fmt.Sprintf("%d", resp.HeadSlot),
		}).Info("Response data")

		// Send response
		if _, err := s.Write([]byte{0x00}); err != nil {
			log.WithError(err).Error("Failed to send status response")
			return
		}
		if n, err := sszNetworkEncoder.EncodeWithMaxLength(s, &resp); err != nil {
			log.WithError(err).Error("Failed to encode outgoing message")
			return
		} else {
			log.WithField("bytes", n).Info("Sent data")
		}
		if err := s.Close(); err != nil {
			log.WithError(err).Error("Failed to close stream")
			return
		}
	})

	// Goodbye
	protocolID = protocol.ID("/eth2/beacon_chain/req/goodbye/1/" + encoder.ProtocolSuffixSSZSnappy)
	p.Host.SetStreamHandler(protocolID, func(s network.Stream) {
		// Read the incoming message into the appropriate struct.
		var out Goodbye
		if err := sszNetworkEncoder.DecodeWithMaxLength(s, &out); err != nil {
			log.WithError(err).Error("Failed to decode incoming message")
			return
		}
		// Log received data
		log.WithFields(logrus.Fields{
			"protocol": s.Protocol(),
			"peer":     s.Conn().RemotePeer().String(),
			"reason":   fmt.Sprintf("%d", out),
		}).Info("Received data")

		// Construct response
		var resp Goodbye

		// Send response
		if _, err := s.Write([]byte{0x00}); err != nil {
			log.WithError(err).Error("Failed to send status response")
			return
		}
		if _, err := sszNetworkEncoder.EncodeWithMaxLength(s, &resp); err != nil {
			log.WithError(err).Error("Failed to encode outgoing message")
			return
		}

		if err := s.Close(); err != nil {
			log.WithError(err).Error("Failed to close stream")
			return
		}
	})

	// Ping
	protocolID = protocol.ID("/eth2/beacon_chain/req/ping/1/" + encoder.ProtocolSuffixSSZSnappy)
	p.Host.SetStreamHandler(protocolID, func(s network.Stream) {
		log.WithFields(logrus.Fields{
			"protocol": s.Protocol(),
			"peer":     s.Conn().RemotePeer().String(),
		}).Info("Got a new stream")
		// Read the incoming message into the appropriate struct.
		var out PingData
		if err := sszNetworkEncoder.DecodeWithMaxLength(s, &out); err != nil {
			log.WithError(err).Error("Failed to decode incoming message")
			return
		}
		// Log received data
		log.WithFields(logrus.Fields{
			"protocol":  s.Protocol(),
			"peer":      s.Conn().RemotePeer().String(),
			"ping_data": fmt.Sprintf("%d", out),
		}).Info("Received data")

		// Construct response
		resp := PingData(p.BeaconAPITest.State.MetaData.SeqNumber)
		// Send response
		if _, err := s.Write([]byte{0x00}); err != nil {
			log.WithError(err).Error("Failed to send status response")
			return
		}
		if _, err := sszNetworkEncoder.EncodeWithMaxLength(s, &resp); err != nil {
			log.WithError(err).Error("Failed to encode outgoing message")
			return
		}

		if err := s.Close(); err != nil {
			log.WithError(err).Error("Failed to close stream")
			return
		}
	})

	return nil
}

func (p *TestP2P) BeaconBlobcksByRange(ctx context.Context, startSlot, count, step, version int) ([]common.Root, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := p.WaitForP2PConnection(timeoutCtx); err != nil {
		return nil, err
	}

	var protocolID protocol.ID
	switch version {
	case 2:
		protocolID = protocol.ID("/eth2/beacon_chain/req/beacon_blocks_by_range/2/" + encoder.ProtocolSuffixSSZSnappy)
	default:
		return nil, errors.New("not implemented")
	}

	peers := p.Host.Network().Peers()
	if len(peers) == 0 {
		return nil, errors.New("no peers")
	}

	// Get peer
	peer := peers[0]

	// Open stream
	s, err := p.Host.NewStream(ctx, peer, protocolID)
	if err != nil {
		return nil, err
	}

	// Construct request
	req := BlocksByRangeRequest{
		StartSlot: uint64(startSlot),
		Count:     uint64(count),
		Step:      uint64(step),
	}

	// Log sent request
	log.WithFields(logrus.Fields{
		"protocol":  s.Protocol(),
		"peer":      s.Conn().RemotePeer().String(),
		"startSlot": fmt.Sprintf("%d", req.StartSlot),
		"count":     fmt.Sprintf("%d", req.Count),
		"step":      fmt.Sprintf("%d", req.Step),
	}).Info("Sending data")

	// Send request
	if _, err := sszNetworkEncoder.EncodeWithMaxLength(s, &req); err != nil {
		return nil, err
	}
	// Done sending request
	if err := s.CloseWrite(); err != nil {
		return nil, err
	}

	time.Sleep(1 * time.Second)
	roots := make([]common.Root, 0, count)

	SetStreamReadDeadline(s, 5*time.Second)

	for {
		// Read response chunks
		result := make([]byte, 1)
		if r, err := s.Read(result); err != nil {
			if err == io.EOF {
				log.Info("Got EOF")
				break
			}
			return nil, err
		} else if r != len(result) {
			return nil, errors.New("unexpected response size")
		} else if result[0] != 0 {
			return nil, errors.New("got unexpected response")
		}

		digest := make([]byte, 4)
		if r, err := s.Read(digest); err != nil {
			return nil, err
		} else if r != len(digest) {
			return nil, errors.New("unexpected response size")
		}

		type HashTreeRootUnmarshaler interface {
			ssz.Unmarshaler
			ssz.HashRoot
		}
		var resp HashTreeRootUnmarshaler
		switch p.BeaconAPITest.ForkDecoder.ForkFromDigest(digest) {
		case "capella":
			resp = new(ethpb.SignedBeaconBlockCapella)
		case "bellatrix":
			resp = new(ethpb.SignedBeaconBlockBellatrix)
		case "altair":
			resp = new(ethpb.SignedBeaconBlockAltair)
		case "genesis":
			resp = new(ethpb.SignedBeaconBlock)
		}

		// Read the incoming message into the appropriate struct.
		if err := sszNetworkEncoder.DecodeWithMaxLength(s, resp); err != nil {
			return nil, err
		}

		root, err := resp.HashTreeRoot()
		if err != nil {
			return nil, err
		}

		roots = append(roots, root)

		// Log received data
		log.WithFields(logrus.Fields{
			"protocol": s.Protocol(),
			"peer":     s.Conn().RemotePeer().String(),
			"root":     fmt.Sprintf("0x%x", root),
		}).Info("Received data")

	}

	return roots, nil
}

func SetStreamReadDeadline(stream network.Stream, duration time.Duration) {
	if err := stream.SetReadDeadline(time.Now().Add(duration)); err != nil &&
		!strings.Contains(err.Error(), "stream closed") {
		log.WithError(err).WithFields(logrus.Fields{
			"peer":      stream.Conn().RemotePeer(),
			"protocol":  stream.Protocol(),
			"direction": stream.Stat().Direction,
		}).Debug("Could not set stream deadline")
	}
}

func (p *TestP2P) Close() error {
	return p.Host.Close()
}
