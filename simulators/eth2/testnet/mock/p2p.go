// Copyright 2020 The go-ethereum Authors
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

package mock

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"net"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/forkid"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/rlpx"
	"github.com/ethereum/go-ethereum/rlp"
)

var pretty = spew.ConfigState{
	Indent:                  "  ",
	DisableCapacities:       true,
	DisablePointerAddresses: true,
	SortKeys:                true,
}

type Pong struct{}

func (p Pong) Name() string { return "pong" }
func (p Pong) Kind() int    { return 0x03 }

// Dial attempts to dial the given node and perform a handshake,
// returning the created Conn if successful.
func Dial(e *enode.Node) (*Conn, error) {
	// dial
	fd, err := net.Dial("tcp", fmt.Sprintf("%v:%d", e.IP(), e.TCP()))
	if err != nil {
		return nil, err
	}
	conn := Conn{Conn: rlpx.NewConn(fd, e.Pubkey())}
	// do encHandshake
	conn.ourKey, _ = crypto.GenerateKey()
	_, err = conn.Handshake(conn.ourKey)
	if err != nil {
		conn.Close()
		return nil, err
	}
	// set default p2p capabilities
	conn.caps = []p2p.Cap{
		{Name: "eth", Version: 66},
	}
	conn.ourHighestProtoVersion = 66
	return &conn, nil
}

// peer performs both the protocol handshake and the status message
// exchange with the node in order to peer with it.
func (c *Conn) Peer(chain *core.BlockChain, status *eth.StatusPacket) error {
	if err := c.handshake(); err != nil {
		return fmt.Errorf("handshake failed: %v", err)
	}
	if _, err := c.statusExchange(chain, status); err != nil {
		return fmt.Errorf("status exchange failed: %v", err)
	}
	return nil
}

// handshake performs a protocol handshake with the node.
func (c *Conn) handshake() error {
	defer c.SetDeadline(time.Time{})
	c.SetDeadline(time.Now().Add(10 * time.Second))
	// write hello to client
	pub0 := crypto.FromECDSAPub(&c.ourKey.PublicKey)[1:]
	ourHandshake := &Hello{
		Version: 5,
		Name2:   "mergemock",
		Caps:    c.caps,
		ID:      pub0,
	}
	if err := c.Write(ourHandshake); err != nil {
		return fmt.Errorf("write to connection failed: %v", err)
	}
	// read hello from client
	code, raw, _, err := c.Conn.Read()
	if err != nil {
		return fmt.Errorf("fail to read msg from peer: %v", err)
	}
	switch code {
	case 0x00:
		msg := new(Hello)
		if err := rlp.DecodeBytes(raw, msg); err != nil {
			return fmt.Errorf("could not rlp decode message: %v", err)
		}
		// set snappy if version is at least 5
		if msg.Version >= 5 {
			c.SetSnappy(true)
		}
		c.negotiateEthProtocol(msg.Caps)
		if c.negotiatedProtoVersion == 0 {
			return fmt.Errorf("unexpected eth protocol version")
		}
		return nil
	default:
		return fmt.Errorf("bad handshake: %#v", raw)
	}
}

// negotiateEthProtocol sets the Conn's eth protocol version to highest
// advertised capability from peer.
func (c *Conn) negotiateEthProtocol(caps []p2p.Cap) {
	var highestEthVersion uint
	for _, capability := range caps {
		if capability.Name != "eth" {
			continue
		}
		if capability.Version > highestEthVersion && capability.Version <= c.ourHighestProtoVersion {
			highestEthVersion = capability.Version
		}
	}
	c.negotiatedProtoVersion = highestEthVersion
}

// statusExchange performs a `Status` message exchange with the given node.
func (c *Conn) statusExchange(chain *core.BlockChain, status *eth.StatusPacket) (eth.Packet, error) {
	// defer c.SetDeadline(time.Time{})
	// c.SetDeadline(time.Now().Add(20 * time.Second))

	// read status message from client
	var message eth.Packet
loop:
	for {
		code, raw, _, err := c.Conn.Read()
		if err != nil {
			return nil, fmt.Errorf("fail to read msg from peer: %v", err)
		}

		switch code {
		case 16:
			msg := new(eth.StatusPacket)
			if err := rlp.DecodeBytes(raw, msg); err != nil {
				return nil, fmt.Errorf("could not rlp decode message: %v", err)
			}

			if have, want := msg.Head, chain.CurrentHeader().Hash(); have != want {
				return nil, fmt.Errorf("wrong head block in status, want:  %#x (block %d) have %#x",
					want, chain.CurrentHeader().Number.Uint64(), have)
			}
			// if have, want := msg.TD.Cmp(chain.GetTd(chain.CurrentHeader().Hash(), chain.CurrentHeader().Number.Uint64())), 0; have != want {
			//         return nil, fmt.Errorf("wrong TD in status: have %v want %v", have, want)
			// }
			// if have, want := msg.ForkID, chain.ForkID(); !reflect.DeepEqual(have, want) {
			//         return nil, fmt.Errorf("wrong fork ID in status: have %v, want %v", have, want)
			// }

			if have, want := msg.ProtocolVersion, c.ourHighestProtoVersion; have != uint32(want) {
				return nil, fmt.Errorf("wrong protocol version: have %v, want %v", have, want)
			}
			message = msg
			break loop
		case 0x01:
			msg := new(p2p.DiscReason)
			if err := rlp.DecodeBytes(raw, msg); err != nil {
				return nil, fmt.Errorf("could not rlp decode message: %v", err)
			}
			return nil, fmt.Errorf("disconnect received: %v", msg)
		case 0x02:
			_, err = c.Conn.Write(3, nil)
			return nil, err
			// (PINGs should not be a response upon fresh connection)
		default:
			return nil, fmt.Errorf("bad status message: code: %d, raw: %s", code, pretty.Sdump(raw))
		}
	}
	// make sure eth protocol version is set for negotiation
	if c.negotiatedProtoVersion == 0 {
		return nil, fmt.Errorf("eth protocol version must be set in Conn")
	}
	if status == nil {
		// default status message
		status = &eth.StatusPacket{
			ProtocolVersion: uint32(c.negotiatedProtoVersion),
			NetworkID:       chain.Config().ChainID.Uint64(),
			TD:              chain.GetTd(chain.CurrentHeader().Hash(), chain.CurrentHeader().Number.Uint64()),
			Head:            chain.CurrentHeader().Hash(),
			Genesis:         chain.Genesis().Hash(),
			ForkID:          forkid.NewID(chain.Config(), chain.Genesis().Hash(), chain.CurrentHeader().Number.Uint64()),
		}
	}
	if err := c.Write66(status, 16); err != nil {
		return nil, fmt.Errorf("write to connection failed: %v", err)
	}
	return message, nil
}

func (c *Conn) Ping() error {
	_, err := c.Conn.Write(2, nil)
	return err
}

func (c *Conn) KeepAlive(ctx context.Context) {
	ticker := time.NewTicker(20 * time.Second)
	for {
		select {
		case <-ctx.Done():
			// log.Info("closing keep-alive")
			return
		case <-ticker.C:
			// log.Trace("Pinging peer")
			err := c.Ping()
			if err != nil {
				// log.WithField("err", err).Error("Unable to ping peer")
			}
		}
	}
}

type Error struct {
	err error
}

func (e *Error) Unwrap() error  { return e.err }
func (e *Error) Error() string  { return e.err.Error() }
func (e *Error) Code() int      { return -1 }
func (e *Error) String() string { return e.Error() }

func errorf(format string, args ...interface{}) *Error {
	return &Error{fmt.Errorf(format, args...)}
}

// Hello is the RLP structure of the protocol handshake.
type Hello struct {
	Version    uint64
	Name2      string
	Caps       []p2p.Cap
	ListenPort uint64
	ID         []byte // secp256k1 public key

	// Ignore additional fields (for forward compatibility).
	Rest []rlp.RawValue `rlp:"tail"`
}

func (h Hello) Name() string { return "hello" }
func (h Hello) Kind() byte   { return 0x00 }

// Conn represents an individual connection with a peer
type Conn struct {
	*rlpx.Conn
	ourKey                 *ecdsa.PrivateKey
	negotiatedProtoVersion uint
	ourHighestProtoVersion uint
	caps                   []p2p.Cap
}

// Write writes a eth packet to the connection.
func (c *Conn) Write(msg eth.Packet) error {
	// check if message is eth protocol message
	var (
		payload []byte
		err     error
	)
	payload, err = rlp.EncodeToBytes(msg)
	if err != nil {
		return err
	}
	_, err = c.Conn.Write(uint64(msg.Kind()), payload)
	return err
}

// Write66 writes an eth66 packet to the connection.
func (c *Conn) Write66(req eth.Packet, code int) error {
	payload, err := rlp.EncodeToBytes(req)
	if err != nil {
		return err
	}
	_, err = c.Conn.Write(uint64(code), payload)
	return err
}
