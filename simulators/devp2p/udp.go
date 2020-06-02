// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package devp2p

import (
	"bytes"
	"container/list"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/nat"
	"github.com/ethereum/go-ethereum/p2p/netutil"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/hive/simulators/common"
	"github.com/google/gopacket/pcap"
)

// Errors
var (
	ErrPacketTooSmall   = errors.New("too small")
	ErrBadHash          = errors.New("bad hash")
	ErrExpired          = errors.New("expired")
	ErrUnsolicitedReply = errors.New("unsolicited reply")
	ErrUnknownNode      = errors.New("unknown node")
	ErrTimeout          = errors.New("RPC timeout")
	ErrClockWarp        = errors.New("reply deadline too far in the future")
	ErrClosed           = errors.New("socket closed")
	ErrResponseReceived = errors.New("response received")
	ErrPacketMismatch   = errors.New("packet mismatch")
	ErrCorruptDHT       = errors.New("corrupt neighbours data")
	UnexpectedPacket    = false
)

// Timeouts
const (
	respTimeout    = 500 * time.Millisecond
	expiration     = 20 * time.Second
	bondExpiration = 24 * time.Hour

	ntpFailureThreshold = 32               // Continuous timeouts after which to check NTP
	ntpWarningCooldown  = 10 * time.Minute // Minimum amount of time to pass before repeating NTP warning
	driftThreshold      = 10 * time.Second // Allowed clock drift before warning user
)

// RPC packet types
const (
	pingPacket = iota + 1 // zero is 'reserved'
	pongPacket
	findnodePacket
	neighborsPacket
	garbagePacket1
	garbagePacket2
	garbagePacket3
	garbagePacket4
	garbagePacket5
	garbagePacket6
	garbagePacket7
	garbagePacket8
)

// RPC request structures
type (
	ping struct {
		Version    uint
		From, To   rpcEndpoint
		Expiration uint64
		// Ignore additional fields (for forward compatibility).
		Rest []rlp.RawValue `rlp:"tail"`
	}

	pingExtra struct {
		Version    uint
		From, To   rpcEndpoint
		Expiration uint64
		JunkData1  uint
		JunkData2  []byte
		// Ignore additional fields (for forward compatibility).
		Rest []rlp.RawValue `rlp:"tail"`
	}

	// pong is the reply to ping.
	pong struct {
		// This field should mirror the UDP envelope address
		// of the ping packet, which provides a way to discover the
		// the external address (after NAT).
		To rpcEndpoint

		ReplyTok   []byte // This contains the hash of the ping packet.
		Expiration uint64 // Absolute timestamp at which the packet becomes invalid.
		// Ignore additional fields (for forward compatibility).
		Rest []rlp.RawValue `rlp:"tail"`
	}

	// findnode is a query for nodes close to the given target.
	findnode struct {
		Target     encPubkey
		Expiration uint64
		// Ignore additional fields (for forward compatibility).
		Rest []rlp.RawValue `rlp:"tail"`
	}

	// reply to findnode
	neighbors struct {
		Nodes      []rpcNode
		Expiration uint64
		// Ignore additional fields (for forward compatibility).
		Rest []rlp.RawValue `rlp:"tail"`
	}

	incomingPacket struct {
		packet      interface{}
		recoveredID encPubkey
	}

	rpcNode struct {
		IP  net.IP // len 4 for IPv4 or 16 for IPv6
		UDP uint16 // for discovery protocol
		TCP uint16 // for RLPx protocol
		ID  encPubkey
	}

	rpcEndpoint struct {
		IP  net.IP // len 4 for IPv4 or 16 for IPv6
		UDP uint16 // for discovery protocol
		TCP uint16 // for RLPx protocol
	}
)

func makeEndpoint(addr *net.UDPAddr, tcpPort uint16) rpcEndpoint {
	ip := addr.IP.To4()
	if ip == nil {
		ip = addr.IP.To16()
	}
	return rpcEndpoint{IP: ip, UDP: uint16(addr.Port), TCP: tcpPort}
}

func (t *V4Udp) nodeFromRPC(sender *net.UDPAddr, rn rpcNode) (*node, error) {
	if rn.UDP <= 1024 {
		return nil, errors.New("low port")
	}
	if err := netutil.CheckRelayIP(sender.IP, rn.IP); err != nil {
		return nil, err
	}
	if t.netrestrict != nil && !t.netrestrict.Contains(rn.IP) {
		return nil, errors.New("not contained in netrestrict whitelist")
	}
	key, err := decodePubkey(rn.ID)
	if err != nil {
		return nil, err
	}
	n := wrapNode(enode.NewV4(key, rn.IP, int(rn.TCP), int(rn.UDP)))
	err = n.ValidateComplete()
	return n, err
}

func nodeToRPC(n *node) rpcNode {
	var key ecdsa.PublicKey
	var ekey encPubkey
	if err := n.Load((*enode.Secp256k1)(&key)); err == nil {
		ekey = EncodePubkey(&key)
	}
	return rpcNode{ID: ekey, IP: n.IP(), UDP: uint16(n.UDP()), TCP: uint16(n.TCP())}
}

type packet interface {
	handle(t *V4Udp, from *net.UDPAddr, fromKey encPubkey, mac []byte) error
	name() string
}

type conn interface {
	ReadFromUDP(b []byte) (n int, addr *net.UDPAddr, err error)
	WriteToUDP(b []byte, addr *net.UDPAddr) (n int, err error)
	Close() error
	LocalAddr() net.Addr
}

//V4Udp is the v4UDP test class
type V4Udp struct {
	conn        conn
	netrestrict *netutil.Netlist
	priv        *ecdsa.PrivateKey
	OurEndpoint rpcEndpoint

	addpending chan *pending
	gotreply   chan reply

	closing chan struct{}
	nat     nat.Interface
	l       common.Logger
}

// pending represents a pending reply.
//
// some implementations of the protocol wish to send more than one
// reply packet to findnode. in general, any neighbors packet cannot
// be matched up with a specific findnode packet.
//
// our implementation handles this by storing a callback function for
// each pending reply. incoming packets from a node are dispatched
// to all the callback functions for that node.
type pending struct {
	// these fields must match in the reply.
	from enode.ID

	// time when the request must complete
	deadline time.Time

	//callback is called when a packet is received. if it returns nil,
	//the callback is removed from the pending reply queue (handled successfully and expected by test case).
	//if it returns a mismatch error, (ignored by callback, further 'pendings' may be in the test case)
	//if it returns any other error, that error is considered the outcome of the
	//'pending' operation

	//callback func(resp interface{}) (done error)
	callback func(resp reply) (done error)

	// errc receives nil when the callback indicates completion or an
	// error if no further reply is received within the timeout.
	errc chan<- error
}

type reply struct {
	from  enode.ID
	ptype byte
	data  interface{}
	// loop indicates whether there was
	// a matching request by sending on this channel.
	matched chan<- bool
}

// ReadPacket is sent to the unhandled channel when it could not be processed
type ReadPacket struct {
	Data []byte
	Addr *net.UDPAddr
}

// Config holds Table-related settings.
type Config struct {
	// These settings are required and configure the UDP listener:
	PrivateKey *ecdsa.PrivateKey

	// These settings are optional:
	AnnounceAddr *net.UDPAddr      // local address announced in the DHT
	NodeDBPath   string            // if set, the node database is stored at this filesystem location
	NetRestrict  *netutil.Netlist  // network whitelist
	Bootnodes    []*enode.Node     // list of bootstrap nodes
	Unhandled    chan<- ReadPacket // unhandled packets are sent on this channel
}

// ListenUDP returns a new table that listens for UDP packets on laddr.
func ListenUDP(c conn, cfg Config, l common.Logger) (*V4Udp, error) {
	v4Udp, err := newUDP(c, cfg, l)

	if err != nil {
		return nil, err
	}
	log.Info("UDP listener up", "self")
	return v4Udp, nil
}

func newUDP(c conn, cfg Config, l common.Logger) (*V4Udp, error) {
	realaddr := c.LocalAddr().(*net.UDPAddr)

	if cfg.AnnounceAddr != nil {
		realaddr = cfg.AnnounceAddr
	}

	udp := &V4Udp{
		conn:        c,
		priv:        cfg.PrivateKey,
		netrestrict: cfg.NetRestrict,
		closing:     make(chan struct{}),
		gotreply:    make(chan reply),
		addpending:  make(chan *pending),
		l:           l,
	}

	udp.OurEndpoint = makeEndpoint(realaddr, uint16(realaddr.Port))

	go udp.loop()
	go udp.readLoop(cfg.Unhandled)
	return udp, nil
}

func (t *V4Udp) close() {
	close(t.closing)
	t.conn.Close()
	//t.db.Close()

}

//SpoofedPing - verify that the faked udp packets are being sent, received, and responses relayed correctly.
func (t *V4Udp) SpoofedPing(toid enode.ID, tomac string, toaddr *net.UDPAddr, fromaddr *net.UDPAddr, validateEnodeID bool, recoveryCallback func(e *ecdsa.PublicKey), netInterface string) error {

	to := makeEndpoint(toaddr, 0)

	req := &ping{
		Version:    4,
		From:       t.OurEndpoint,
		To:         to, // TODO: maybe use known TCP port from DB
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}

	packet, hash, err := encodePacket(t.priv, pingPacket, req)
	if err != nil {
		return err
	}

	t.l.Log("Establishing criteria: Will succeed only if a valid pong is received.")
	callback := func(p reply) error {

		if p.ptype == pongPacket {
			inPacket := p.data.(incomingPacket)

			if !bytes.Equal(inPacket.packet.(*pong).ReplyTok, hash) {
				return ErrUnsolicitedReply
			}

			if validateEnodeID && toid != inPacket.recoveredID.id() {
				return ErrUnknownNode
			}

			if recoveryCallback != nil {
				key, err := decodePubkey(inPacket.recoveredID)
				if err != nil {
					recoveryCallback(key)
				}
			}
			return nil

		}
		return ErrPacketMismatch

	}

	return <-t.sendSpoofedPacket(toid, toaddr, fromaddr, req.name(), packet, tomac, callback, netInterface)

}

//SpoofingFindNodeCheck tests if a client is susceptible to being used
//as an attack vector for findnode amplification attacks
//
// The test spoofs a ping from a different ip address('victim ip'), and later on sends a spoofed 'pong'. The spoofed 'pong' will have
// an invalid reply-token (since the target will send the 'pong' to 'victim ip'), and should thus be ignored by the target.
// If the target fails to verify reply-token, it will believe to now be bonded with a node at 'victim ip'.
// The test then sends a spoofed 'findnode' request. If the 'target' responds to the findnode-request, sending a large
// 'neighbours'-packet to 'victim ip', it can be used for DoS attacks.
func (t *V4Udp) SpoofingFindNodeCheck(toid enode.ID, tomac string, toaddr *net.UDPAddr, fromaddr *net.UDPAddr, validateEnodeID bool, netInterface string) error {

	//send ping
	err := t.SpoofedPing(toid, tomac, toaddr, fromaddr, false, nil, netInterface)
	if err != nil {
		return err
	}

	//the 'victim' source

	//wait for a tiny bit
	//NB: in a real scenario the 'victim' will have responded with a v4 pong
	//message to our ping recipient. in the attack scenario, the pong
	//will have been ignored because the source id is different than
	//expected. (to be more authentic, an improvement to this test
	//could be to send a fake pong from the node id - but this is not
	//essential because the following pong may be received prior to the
	//real pong)
	time.Sleep(200 * time.Millisecond)

	//send spoofed pong from this node id but with junk replytok
	//because the replytok will not be available to a real attacker
	//TODO- send a best reply tok guess?
	to := makeEndpoint(toaddr, 0)
	pongreq := &pong{

		To:         to,
		ReplyTok:   make([]byte, macSize),
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}
	packet, _, err := encodePacket(t.priv, pongPacket, pongreq)
	if err != nil {
		return err
	}
	t.spoofedWrite(toaddr, fromaddr, pongreq.name(), packet, tomac, netInterface)

	//consider the target 'bonded' , as it has received the expected pong
	//send a findnode request for a random 'target' (target there being the
	//node to find)
	var fakeKey *ecdsa.PrivateKey
	if fakeKey, err = crypto.GenerateKey(); err != nil {
		return err
	}
	fakePub := fakeKey.PublicKey
	lookupTarget := EncodePubkey(&fakePub)

	findreq := &findnode{
		Target:     lookupTarget,
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}

	findpacket, _, err := encodePacket(t.priv, findnodePacket, findreq)
	if err != nil {
		return err
	}

	//if we receive a neighbours request, then the attack worked and the test should fail
	t.l.Log("Establishing criteria: Fail if any packet received. Succeed if nothing received within timeouts.")
	callback := func(p reply) error {
		if p.ptype == neighborsPacket {
			return ErrUnsolicitedReply
		}
		return ErrTimeout
	}

	return <-t.sendSpoofedPacket(toid, toaddr, fromaddr, findreq.name(), findpacket, tomac, callback, netInterface)

}

//Ping sends a ping message to the given node and waits for a reply.
func (t *V4Udp) Ping(toid enode.ID, toaddr *net.UDPAddr, validateEnodeID bool, recoveryCallback func(e *ecdsa.PublicKey)) error {

	to := makeEndpoint(toaddr, 0)

	req := &ping{
		Version:    4,
		From:       t.OurEndpoint,
		To:         to, // TODO: maybe use known TCP port from DB
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}

	packet, hash, err := encodePacket(t.priv, pingPacket, req)
	if err != nil {
		return err
	}

	t.l.Log("Establishing criteria: Will succeed only if a valid pong is received.")
	callback := func(p reply) error {

		if p.ptype == pongPacket {
			inPacket := p.data.(incomingPacket)

			if !bytes.Equal(inPacket.packet.(*pong).ReplyTok, hash) {
				return ErrUnsolicitedReply
			}

			if validateEnodeID && toid != inPacket.recoveredID.id() {
				return ErrUnknownNode
			}

			if recoveryCallback != nil {
				key, err := decodePubkey(inPacket.recoveredID)
				if err != nil {
					recoveryCallback(key)
				}
			}
			return nil

		}
		return ErrPacketMismatch

	}

	return <-t.sendPacket(toid, toaddr, req.name(), packet, callback)

}

//PingWrongFrom pings with incorrect from field
func (t *V4Udp) PingWrongFrom(toid enode.ID, toaddr *net.UDPAddr, validateEnodeID bool, recoveryCallback func(e *ecdsa.PublicKey)) error {

	to := makeEndpoint(toaddr, 0)

	from := makeEndpoint(&net.UDPAddr{IP: []byte{0, 1, 2, 3}, Port: 1}, 0) //this is a garbage endpoint

	req := &ping{
		Version:    4,
		From:       from,
		To:         to, // TODO: maybe use known TCP port from DB
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}

	packet, hash, err := encodePacket(t.priv, pingPacket, req)
	if err != nil {
		return err
	}

	//expect the usual ping stuff - a bad 'from' should be ignored
	t.l.Log("Establishing criteria: Will succeed only if a valid pong is received.")
	callback := func(p reply) error {
		if p.ptype == pongPacket {
			inPacket := p.data.(incomingPacket)

			if !bytes.Equal(inPacket.packet.(*pong).ReplyTok, hash) {
				return ErrUnsolicitedReply
			}

			if validateEnodeID && toid != inPacket.recoveredID.id() {
				return ErrUnknownNode
			}

			if recoveryCallback != nil {
				key, err := decodePubkey(inPacket.recoveredID)
				if err != nil {
					recoveryCallback(key)
				}
			}
			return nil
		}
		return ErrPacketMismatch

	}
	return <-t.sendPacket(toid, toaddr, req.name(), packet, callback)

}

//PingWrongTo pings with incorrect to
func (t *V4Udp) PingWrongTo(toid enode.ID, toaddr *net.UDPAddr, validateEnodeID bool, recoveryCallback func(e *ecdsa.PublicKey)) error {

	to := makeEndpoint(&net.UDPAddr{IP: []byte{0, 1, 2, 3}, Port: 1}, 0)

	req := &ping{
		Version:    4,
		From:       t.OurEndpoint,
		To:         to, // TODO: maybe use known TCP port from DB
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}

	packet, _, err := encodePacket(t.priv, pingPacket, req)
	if err != nil {
		return err
	}
	t.l.Log("Establishing criteria: Will succeed if a pong packet is received.")
	callback := func(p reply) error {
		if p.ptype == pongPacket {
			return nil
		}

		return ErrPacketMismatch
	}
	return <-t.sendPacket(toid, toaddr, req.name(), packet, callback)

}

//PingExtraData ping with a 'future format' packet containing extra fields
func (t *V4Udp) PingExtraData(toid enode.ID, toaddr *net.UDPAddr, validateEnodeID bool, recoveryCallback func(e *ecdsa.PublicKey)) error {

	to := makeEndpoint(toaddr, 0)

	req := &pingExtra{
		Version:   4,
		From:      t.OurEndpoint,
		To:        to,
		JunkData1: 42,
		JunkData2: []byte{9, 8, 7, 6, 5, 4, 3, 2, 1},

		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}

	packet, hash, err := encodePacket(t.priv, pingPacket, req)
	if err != nil {
		return err
	}

	//expect the usual ping responses
	t.l.Log("Establishing criteria: Will succeed if a valid pong packet is received.")
	callback := func(p reply) error {
		if p.ptype == pongPacket {
			inPacket := p.data.(incomingPacket)

			if !bytes.Equal(inPacket.packet.(*pong).ReplyTok, hash) {
				return ErrUnsolicitedReply
			}

			if validateEnodeID && toid != inPacket.recoveredID.id() {
				return ErrUnknownNode
			}

			if recoveryCallback != nil {
				key, err := decodePubkey(inPacket.recoveredID)
				if err != nil {
					recoveryCallback(key)
				}
			}
			return nil
		}
		return ErrPacketMismatch

	}
	dummyPing := ping{} // just to get the name
	return <-t.sendPacket(toid, toaddr, dummyPing.name(), packet, callback)

}

//PingExtraDataWrongFrom ping with a 'future format' packet containing extra fields and make sure it works even with the wrong 'from' field
func (t *V4Udp) PingExtraDataWrongFrom(toid enode.ID, toaddr *net.UDPAddr, validateEnodeID bool, recoveryCallback func(e *ecdsa.PublicKey)) error {

	to := makeEndpoint(toaddr, 0)

	from := makeEndpoint(&net.UDPAddr{IP: []byte{0, 1, 2, 3}, Port: 1}, 0) //this is a garbage endpoint

	req := &pingExtra{
		Version:   4,
		From:      from,
		To:        to,
		JunkData1: 42,
		JunkData2: []byte{9, 8, 7, 6, 5, 4, 3, 2, 1},

		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}

	packet, hash, err := encodePacket(t.priv, pingPacket, req)
	if err != nil {
		return err
	}

	//expect the usual ping reponses
	t.l.Log("Establishing criteria: Will succeed if a valid pong packet is received.")
	callback := func(p reply) error {
		if p.ptype == pongPacket {
			inPacket := p.data.(incomingPacket)

			if !bytes.Equal(inPacket.packet.(*pong).ReplyTok, hash) {
				return ErrUnsolicitedReply
			}

			if validateEnodeID && toid != inPacket.recoveredID.id() {
				return ErrUnknownNode
			}

			if recoveryCallback != nil {
				key, err := decodePubkey(inPacket.recoveredID)
				if err != nil {
					recoveryCallback(key)
				}
			}
			return nil
		}
		return ErrPacketMismatch

	}
	dummyPing := ping{} //the dummy ping is just to get the name
	return <-t.sendPacket(toid, toaddr, dummyPing.name(), packet, callback)

}

// PingTargetWrongPacketType send a packet (a ping packet, though it could be something else) with an unknown packet type to the client and
// see how the target behaves. If the target responds to the ping, then fail.
func (t *V4Udp) PingTargetWrongPacketType(toid enode.ID, toaddr *net.UDPAddr, validateEnodeID bool, recoveryCallback func(e *ecdsa.PublicKey)) error {

	to := makeEndpoint(toaddr, 0)

	req := &ping{
		Version:    4,
		From:       t.OurEndpoint,
		To:         to, // TODO: maybe use known TCP port from DB
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}

	packet, _, err := encodePacket(t.priv, garbagePacket8, req)
	if err != nil {
		return err
	}

	//expect anything but a ping or pong
	t.l.Log("Establishing criteria: Fail immediately if a ping or pong is received. Succeed if nothing occurs within timeout.")
	callback := func(p reply) error {
		if p.ptype == pongPacket {
			return ErrUnsolicitedReply
		}

		if p.ptype == pingPacket {
			return ErrUnsolicitedReply
		}

		return ErrPacketMismatch
	}
	return <-t.sendPacket(toid, toaddr, req.name(), packet, callback)

}

// FindnodeWithoutBond tries to find a node without a previous bond
func (t *V4Udp) FindnodeWithoutBond(toid enode.ID, toaddr *net.UDPAddr, target encPubkey) error {

	req := &findnode{
		Target:     target,
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}

	packet, _, err := encodePacket(t.priv, findnodePacket, req)
	if err != nil {
		return err
	}

	//expect nothing
	t.l.Log("Establishing criteria: Fail if any packet received. Succeed if nothing received within timeouts.")
	callback := func(p reply) error {
		if p.ptype == pingPacket {
			t.l.Log("Warning: Node attempting to bond in response to FindNode.")
			return ErrTimeout
		}
		return ErrUnsolicitedReply

	}

	return <-t.sendPacket(toid, toaddr, req.name(), packet, callback)

}

//PingBondedWithMangledFromField ping a bonded node with bad from fields
func (t *V4Udp) PingBondedWithMangledFromField(toid enode.ID, toaddr *net.UDPAddr, validateEnodeID bool, recoveryCallback func(e *ecdsa.PublicKey)) error {

	//try to bond with the target using normal ping data
	err := t.Ping(toid, toaddr, false, nil)
	if err != nil {
		return err
	}
	//hang around for a bit (we don't know if the target was already bonded or not)
	time.Sleep(2 * time.Second)

	to := makeEndpoint(toaddr, 0)

	from := makeEndpoint(&net.UDPAddr{IP: []byte{0, 1, 2, 3}, Port: 1}, 0) //this is a garbage endpoint

	req := &ping{
		Version:    4,
		From:       from,
		To:         to, // TODO: maybe use known TCP port from DB
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}

	packet, hash, err := encodePacket(t.priv, pingPacket, req)
	if err != nil {
		return err
	}

	//expect the usual ping stuff - a bad 'from' should be ignored
	t.l.Log("Establishing criteria: Succeed if valid pong received.")
	callback := func(p reply) error {
		if p.ptype == pongPacket {
			inPacket := p.data.(incomingPacket)

			if !bytes.Equal(inPacket.packet.(*pong).ReplyTok, hash) {
				return ErrUnsolicitedReply
			}

			if validateEnodeID && toid != inPacket.recoveredID.id() {
				return ErrUnknownNode
			}

			if recoveryCallback != nil {
				key, err := decodePubkey(inPacket.recoveredID)
				if err != nil {
					recoveryCallback(key)
				}
			}
			return nil
		}
		return ErrPacketMismatch

	}
	return <-t.sendPacket(toid, toaddr, req.name(), packet, callback)

}

//BondedSourceFindNeighbours basic find neighbours tests
func (t *V4Udp) BondedSourceFindNeighbours(toid enode.ID, toaddr *net.UDPAddr, target encPubkey) error {
	//try to bond with the target
	err := t.Ping(toid, toaddr, false, nil)
	if err != nil {
		return err
	}
	//hang around for a bit (we don't know if the target was already bonded or not)
	time.Sleep(2 * time.Second)

	//send an unsolicited neighbours packet
	req := neighbors{Expiration: uint64(time.Now().Add(expiration).Unix())}
	var fakeKey *ecdsa.PrivateKey
	if fakeKey, err = crypto.GenerateKey(); err != nil {
		return err
	}
	fakePub := fakeKey.PublicKey
	encFakeKey := EncodePubkey(&fakePub)
	fakeNeighbour := rpcNode{ID: encFakeKey, IP: net.IP{1, 2, 3, 4}, UDP: 123, TCP: 123}
	req.Nodes = []rpcNode{fakeNeighbour}

	t.send(toaddr, neighborsPacket, &req)

	//now call find neighbours
	findReq := &findnode{
		Target:     target,
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}

	packet, _, err := encodePacket(t.priv, findnodePacket, findReq)
	if err != nil {
		return err
	}

	//expect good neighbours response with no junk
	t.l.Log("Establishing criteria: Succeed if a neighbours packet is received that is not polluted.")
	callback := func(p reply) error {

		if p.ptype == neighborsPacket {
			//got a response.
			//we assume the target is not connected to a public or populated bootnode
			//so we assume the target does not have any other neighbours in the DHT
			inPacket := p.data.(incomingPacket)

			for _, neighbour := range inPacket.packet.(*neighbors).Nodes {
				if neighbour.ID == encFakeKey {
					return ErrCorruptDHT
				}
			}
			return nil

		}
		return ErrUnsolicitedReply
	}

	return <-t.sendPacket(toid, toaddr, findReq.name(), packet, callback)

}

//PingPastExpiration check past expirations are handled correctly
func (t *V4Udp) PingPastExpiration(toid enode.ID, toaddr *net.UDPAddr, validateEnodeID bool, recoveryCallback func(e *ecdsa.PublicKey)) error {

	to := makeEndpoint(toaddr, 0)

	req := &ping{
		Version:    4,
		From:       t.OurEndpoint,
		To:         to, // TODO: maybe use known TCP port from DB
		Expiration: uint64(time.Now().Add(-expiration).Unix()),
	}

	packet, _, err := encodePacket(t.priv, pingPacket, req)
	if err != nil {
		return err
	}

	//expect no pong
	t.l.Log("Establishing criteria: Fail if a pong is received. Succeed if nothing received within timeout.")
	callback := func(p reply) error {
		if p.ptype == pongPacket {
			return ErrUnsolicitedReply
		}
		return ErrPacketMismatch

	}
	return <-t.sendPacket(toid, toaddr, req.name(), packet, callback)

}

//BondedSourceFindNeighboursPastExpiration -
func (t *V4Udp) BondedSourceFindNeighboursPastExpiration(toid enode.ID, toaddr *net.UDPAddr, target encPubkey) error {
	//try to bond with the target
	err := t.Ping(toid, toaddr, false, nil)
	if err != nil {
		return err
	}
	//hang around for a bit (we don't know if the target was already bonded or not)
	time.Sleep(2 * time.Second)

	//now call find neighbours
	findReq := &findnode{
		Target:     target,
		Expiration: uint64(time.Now().Add(-expiration).Unix()),
	}

	packet, _, err := encodePacket(t.priv, findnodePacket, findReq)
	if err != nil {
		return err
	}

	//expect good neighbours response with no junk
	t.l.Log("Establishing criteria: Fail if a neighbours packet is received. Succeed if nothing received within timeouts.")
	callback := func(p reply) error {

		if p.ptype == neighborsPacket {
			return ErrUnsolicitedReply

		}
		return ErrPacketMismatch
	}

	return <-t.sendPacket(toid, toaddr, findReq.name(), packet, callback)

}

func (t *V4Udp) sendPacket(toid enode.ID, toaddr *net.UDPAddr, reqName string, packet []byte, callback func(reply) error) <-chan error {

	t.l.Logf("Sending packet %s to enode %s with target endpoint %v", reqName, toid, toaddr)
	errc := t.pending(toid, callback)
	t.write(toaddr, reqName, packet)
	return errc
}

func (t *V4Udp) sendSpoofedPacket(toid enode.ID, toaddr *net.UDPAddr, fromaddr *net.UDPAddr, reqName string, packet []byte, macaddr string, callback func(reply) error, netInterface string) <-chan error {

	t.l.Logf("Sending spoofed packet %s to enode %s with target endpoint %v from %v", reqName, toid, toaddr, fromaddr)
	errc := t.pending(toid, callback)
	t.spoofedWrite(toaddr, fromaddr, reqName, packet, macaddr, netInterface)

	return errc
}

// pending adds a reply callback to the pending reply queue.
// see the documentation of type pending for a detailed explanation.
func (t *V4Udp) pending(id enode.ID, callback func(reply) error) <-chan error {
	ch := make(chan error, 1)
	p := &pending{from: id, callback: callback, errc: ch}
	select {
	case t.addpending <- p:
		// loop will handle it
	case <-t.closing:
		ch <- ErrClosed
	}
	return ch
}

func (t *V4Udp) handleReply(from enode.ID, ptype byte, req incomingPacket) bool {
	matched := make(chan bool, 1)
	select {
	case t.gotreply <- reply{from, ptype, req, matched}:
		// loop will handle it
		return <-matched
	case <-t.closing:
		return false
	}
}

// loop runs in its own goroutine. it keeps track of
// the refresh timer and the pending reply queue.
func (t *V4Udp) loop() {
	var (
		plist        = list.New()
		timeout      = time.NewTimer(0)
		nextTimeout  *pending // head of plist when timeout was last reset
		contTimeouts = 0      // number of continuous timeouts to do NTP checks
	//	ntpWarnTime  = time.Unix(0, 0)
	)
	<-timeout.C // ignore first timeout
	defer timeout.Stop()

	resetTimeout := func() {
		if plist.Front() == nil || nextTimeout == plist.Front().Value {
			return
		}
		// Start the timer so it fires when the next pending reply has expired.
		now := time.Now()
		for el := plist.Front(); el != nil; el = el.Next() {
			nextTimeout = el.Value.(*pending)
			if dist := nextTimeout.deadline.Sub(now); dist < 2*respTimeout {
				timeout.Reset(dist)
				return
			}
			// Remove pending replies whose deadline is too far in the
			// future. These can occur if the system clock jumped
			// backwards after the deadline was assigned.
			nextTimeout.errc <- ErrClockWarp
			plist.Remove(el)
		}
		nextTimeout = nil
		timeout.Stop()
	}

	for {
		resetTimeout()

		select {
		case <-t.closing:
			for el := plist.Front(); el != nil; el = el.Next() {
				el.Value.(*pending).errc <- ErrClosed
			}
			return

		case p := <-t.addpending:
			p.deadline = time.Now().Add(respTimeout)
			plist.PushBack(p)

		case r := <-t.gotreply:
			var matched bool
			for el := plist.Front(); el != nil; el = el.Next() {
				p := el.Value.(*pending)
				if p.from == r.from {

					// Remove the matcher if its callback indicates
					// that all replies have been received. This is
					// required for packet types that expect multiple
					// reply packets.

					cbres := p.callback(r)
					if cbres != ErrPacketMismatch {
						matched = true
						if cbres == nil {
							plist.Remove(el)
							p.errc <- nil
						} else {
							plist.Remove(el)
							p.errc <- cbres
						}
					}

					// Reset the continuous timeout counter (time drift detection)
					contTimeouts = 0
				}
			}
			r.matched <- matched

		case now := <-timeout.C:
			nextTimeout = nil

			// Notify and remove callbacks whose deadline is in the past.
			for el := plist.Front(); el != nil; el = el.Next() {
				p := el.Value.(*pending)
				if now.After(p.deadline) || now.Equal(p.deadline) {
					t.l.Log("Timing out pending packet.")
					p.errc <- ErrTimeout
					plist.Remove(el)
					contTimeouts++
				}
			}

		}
	}
}

const (
	macSize  = 256 / 8
	sigSize  = 520 / 8
	headSize = macSize + sigSize // space of packet frame data
)

var (
	headSpace = make([]byte, headSize)

	// Neighbors replies are sent across multiple packets to
	// stay below the 1280 byte limit. We compute the maximum number
	// of entries by stuffing a packet until it grows too large.
	maxNeighbors int
)

func init() {
	p := neighbors{Expiration: ^uint64(0)}
	maxSizeNode := rpcNode{IP: make(net.IP, 16), UDP: ^uint16(0), TCP: ^uint16(0)}
	for n := 0; ; n++ {
		p.Nodes = append(p.Nodes, maxSizeNode)
		size, _, err := rlp.EncodeToReader(p)
		if err != nil {
			// If this ever happens, it will be caught by the unit tests.
			panic("cannot encode: " + err.Error())
		}
		if headSize+size+1 >= 1280 {
			maxNeighbors = n
			break
		}
	}
}

func (t *V4Udp) send(toaddr *net.UDPAddr, ptype byte, req packet) ([]byte, error) {
	packet, hash, err := encodePacket(t.priv, ptype, req)
	if err != nil {
		return hash, err
	}
	return hash, t.write(toaddr, req.name(), packet)
}

func (t *V4Udp) write(toaddr *net.UDPAddr, what string, packet []byte) error {
	_, err := t.conn.WriteToUDP(packet, toaddr)
	//_, err := t.conn.WriteToUDPSpoofed(packet, toaddr, toaddr)
	log.Trace(">> "+what, "addr", toaddr, "err", err)
	return err
}

//GetNetworkInterface - Get the docker image's network interface
func GetNetworkInterface(netInterface string) (*net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var iface *net.Interface
	ip := net.ParseIP(netInterface)
	if ip == nil {
		//look for eth0 or the specified override
		for _, i := range ifaces {
			if i.Name == netInterface {
				iface = &i
			}
		}
	} else {

		for _, i := range ifaces {
			addrs, err := i.Addrs()
			if err == nil && addrs != nil {
				for _, addr := range addrs {
					ip, _, err = net.ParseCIDR(addr.String())
					fmt.Println(ip.String())
					if err == nil && ip.String() == netInterface {

						iface = &i
					}
				}
			}
		}
	}

	return iface, nil
}

//GetInterfaceIP returns the IP address of the (usually eth0) interface (see above)
func GetInterfaceIP(iface *net.Interface) (*net.IP, error) {
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		switch v := addr.(type) {
		case *net.IPNet:
			return &v.IP, nil
		case *net.IPAddr:
			return &v.IP, nil
		}

	}
	return nil, nil
}

func (t *V4Udp) spoofedWrite(toaddr *net.UDPAddr, fromaddr *net.UDPAddr, what string, packet []byte, macAddr string, netInterface string) error {

	mac, err := net.ParseMAC(macAddr)
	if err != nil {
		return err
	}

	iface, err := GetNetworkInterface(netInterface)
	if err != nil {
		return err
	}

	if nil == iface {
		return errors.New("interface not found: " + netInterface)
	}

	opts := udpFrameOptions{
		sourceIP:     fromaddr.IP.To4(),
		destIP:       toaddr.IP.To4(),
		sourcePort:   uint16(fromaddr.Port),
		destPort:     uint16(toaddr.Port),
		sourceMac:    iface.HardwareAddr,
		destMac:      mac,
		isIPv6:       false,
		payloadBytes: packet,
	}

	handle, err := pcap.OpenLive(iface.Name, 65536, true, pcap.BlockForever)
	if err != nil {
		return err
	}

	defer handle.Close()

	rawPacket, err := createSerializedUDPFrame(opts)
	if err != nil {
		return err
	}

	if err := handle.WritePacketData(rawPacket); err != nil {
		return err
	}

	log.Trace(">> "+what, "from", fromaddr, "addr", toaddr, "err", err)
	return err
}

func encodePacket(priv *ecdsa.PrivateKey, ptype byte, req interface{}) (packet, hash []byte, err error) {
	b := new(bytes.Buffer)
	b.Write(headSpace)
	b.WriteByte(ptype)
	if err := rlp.Encode(b, req); err != nil {
		log.Error("Can't encode discv4 packet", "err", err)
		return nil, nil, err
	}
	packet = b.Bytes()
	sig, err := crypto.Sign(crypto.Keccak256(packet[headSize:]), priv)
	if err != nil {
		log.Error("Can't sign discv4 packet", "err", err)
		return nil, nil, err
	}
	copy(packet[macSize:], sig)
	// add the hash to the front. Note: this doesn't protect the
	// packet in any way. Our public key will be part of this hash in
	// The future.
	hash = crypto.Keccak256(packet[macSize:])
	copy(packet, hash)
	return packet, hash, nil
}

// readLoop runs in its own goroutine. it handles incoming UDP packets.
func (t *V4Udp) readLoop(unhandled chan<- ReadPacket) {
	defer t.conn.Close()
	if unhandled != nil {
		defer close(unhandled)
	}
	// Discovery packets are defined to be no larger than 1280 bytes.
	// Packets larger than this size will be cut at the end and treated
	// as invalid because their hash won't match.
	buf := make([]byte, 1280)
	for {
		nbytes, from, err := t.conn.ReadFromUDP(buf)
		if netutil.IsTemporaryError(err) {
			// Ignore temporary read errors.
			log.Debug("Temporary UDP read error", "err", err)
			continue
		} else if err != nil {
			// Shut down the loop for permament errors.
			log.Debug("UDP read error", "err", err)
			return
		}
		if t.handlePacket(from, buf[:nbytes]) != nil && unhandled != nil {
			select {
			case unhandled <- ReadPacket{buf[:nbytes], from}:
			default:
			}
		}
	}
}

func (t *V4Udp) handlePacket(from *net.UDPAddr, buf []byte) error {
	inpacket, fromKey, hash, err := decodePacket(buf)
	if err != nil {
		log.Debug("Bad discv4 packet", "addr", from, "err", err)
		return err
	}
	t.l.Logf("Receiving packet %s from %v", inpacket.name(), from)
	err = inpacket.handle(t, from, fromKey, hash)

	log.Trace("<< "+inpacket.name(), "addr", from, "err", err)
	return err
}

func decodePacket(buf []byte) (packet, encPubkey, []byte, error) {

	if len(buf) < headSize+1 {
		return nil, encPubkey{}, nil, ErrPacketTooSmall
	}
	hash, sig, sigdata := buf[:macSize], buf[macSize:headSize], buf[headSize:]
	shouldhash := crypto.Keccak256(buf[macSize:])
	if !bytes.Equal(hash, shouldhash) {
		return nil, encPubkey{}, nil, ErrBadHash
	}
	fromKey, err := recoverNodeKey(crypto.Keccak256(buf[headSize:]), sig)
	if err != nil {
		return nil, fromKey, hash, err
	}

	var req packet
	switch ptype := sigdata[0]; ptype {
	case pingPacket:
		req = new(ping)
	case pongPacket:
		req = new(pong)
	case findnodePacket:
		req = new(findnode)
	case neighborsPacket:
		req = new(neighbors)
	default:
		return req, fromKey, hash, fmt.Errorf("unknown type: %d", ptype)
	}
	s := rlp.NewStream(bytes.NewReader(sigdata[1:]), 0)
	err = s.Decode(req)

	return req, fromKey, hash, err
}

func (req *ping) handle(t *V4Udp, from *net.UDPAddr, fromKey encPubkey, mac []byte) error {
	if expired(req.Expiration) {
		return ErrExpired
	}
	key, err := decodePubkey(fromKey)
	if err != nil {
		return fmt.Errorf("invalid public key: %v", err)
	}
	t.send(from, pongPacket, &pong{
		To:         makeEndpoint(from, req.From.TCP),
		ReplyTok:   mac,
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	})
	n := wrapNode(enode.NewV4(key, from.IP, int(req.From.TCP), from.Port))
	t.handleReply(n.ID(), pingPacket, incomingPacket{packet: req, recoveredID: fromKey})

	return nil
}

func (req *ping) name() string { return "PING/v4" }

func (req *pong) handle(t *V4Udp, from *net.UDPAddr, fromKey encPubkey, mac []byte) error {
	if expired(req.Expiration) {
		return ErrExpired
	}
	fromID := fromKey.id()
	t.handleReply(fromID, pongPacket, incomingPacket{packet: req, recoveredID: fromKey})

	return nil
}

func (req *pong) name() string { return "PONG/v4" }

func (req *findnode) handle(t *V4Udp, from *net.UDPAddr, fromKey encPubkey, mac []byte) error {
	if expired(req.Expiration) {
		return ErrExpired
	}

	return nil
}

func (req *findnode) name() string { return "FINDNODE/v4" }

func (req *neighbors) handle(t *V4Udp, from *net.UDPAddr, fromKey encPubkey, mac []byte) error {
	if expired(req.Expiration) {
		return ErrExpired
	}
	if !t.handleReply(fromKey.id(), neighborsPacket, incomingPacket{packet: req, recoveredID: fromKey}) {
		return ErrUnsolicitedReply
	}
	return nil
}

func (req *neighbors) name() string { return "NEIGHBORS/v4" }

func expired(ts uint64) bool {
	return time.Unix(int64(ts), 0).Before(time.Now())
}
