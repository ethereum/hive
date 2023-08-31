// Copyright 2020 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package devp2p

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/rlpx"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
)

type Message interface {
	Code() int
	ReqID() uint64
}

type Error struct {
	err error
}

func (e *Error) Unwrap() error  { return e.err }
func (e *Error) Error() string  { return e.err.Error() }
func (e *Error) String() string { return e.Error() }

func (e *Error) Code() int     { return -1 }
func (e *Error) ReqID() uint64 { return 0 }

func errorf(format string, args ...interface{}) *Error {
	return &Error{fmt.Errorf(format, args...)}
}

// Hello is the RLP structure of the protocol handshake.
type Hello struct {
	Version    uint64
	Name       string
	Caps       []p2p.Cap
	ListenPort uint64
	ID         []byte // secp256k1 public key

	// Ignore additional fields (for forward compatibility).
	Rest []rlp.RawValue `rlp:"tail"`
}

func (msg Hello) Code() int     { return 0x00 }
func (msg Hello) ReqID() uint64 { return 0 }

// Disconnect is the RLP structure for a disconnect message.
type Disconnect struct {
	Reason p2p.DiscReason
}

func (msg Disconnect) Code() int     { return 0x01 }
func (msg Disconnect) ReqID() uint64 { return 0 }

type Ping struct{}

func (msg Ping) Code() int     { return 0x02 }
func (msg Ping) ReqID() uint64 { return 0 }

type Pong struct{}

func (msg Pong) Code() int     { return 0x03 }
func (msg Pong) ReqID() uint64 { return 0 }

// Status is the network packet for the status message for eth/64 and later.
type Status eth.StatusPacket

func (msg Status) Code() int     { return 16 }
func (msg Status) ReqID() uint64 { return 0 }

// NewBlockHashes is the network packet for the block announcements.
type NewBlockHashes eth.NewBlockHashesPacket

func (msg NewBlockHashes) Code() int     { return 17 }
func (msg NewBlockHashes) ReqID() uint64 { return 0 }

type Transactions eth.TransactionsPacket

func (msg Transactions) Code() int     { return 18 }
func (msg Transactions) ReqID() uint64 { return 18 }

// GetBlockHeaders represents a block header query.
type GetBlockHeaders eth.GetBlockHeadersPacket66

func (msg GetBlockHeaders) Code() int     { return 19 }
func (msg GetBlockHeaders) ReqID() uint64 { return msg.RequestId }

type BlockHeaders eth.BlockHeadersPacket66

func (msg BlockHeaders) Code() int     { return 20 }
func (msg BlockHeaders) ReqID() uint64 { return msg.RequestId }

// GetBlockBodies represents a GetBlockBodies request
type GetBlockBodies eth.GetBlockBodiesPacket66

func (msg GetBlockBodies) Code() int     { return 21 }
func (msg GetBlockBodies) ReqID() uint64 { return msg.RequestId }

// BlockBodies is the network packet for block content distribution.
type BlockBodies eth.BlockBodiesPacket66

func (msg BlockBodies) Code() int     { return 22 }
func (msg BlockBodies) ReqID() uint64 { return msg.RequestId }

// NewBlock is the network packet for the block propagation message.
type NewBlock eth.NewBlockPacket

func (msg NewBlock) Code() int     { return 23 }
func (msg NewBlock) ReqID() uint64 { return 0 }

// NewPooledTransactionHashes66 is the network packet for the tx hash propagation message.
type NewPooledTransactionHashes66 eth.NewPooledTransactionHashesPacket66

func (msg NewPooledTransactionHashes66) Code() int     { return 24 }
func (msg NewPooledTransactionHashes66) ReqID() uint64 { return 0 }

// NewPooledTransactionHashes is the network packet for the tx hash propagation message.
type NewPooledTransactionHashes eth.NewPooledTransactionHashesPacket68

func (msg NewPooledTransactionHashes) Code() int     { return 24 }
func (msg NewPooledTransactionHashes) ReqID() uint64 { return 0 }

type GetPooledTransactions eth.GetPooledTransactionsPacket66

func (msg GetPooledTransactions) Code() int     { return 25 }
func (msg GetPooledTransactions) ReqID() uint64 { return msg.RequestId }

// PooledTransactionsPacket is the network packet for transaction distribution.
// We decode to bytes to be able to check the serialized version.
type PooledTransactionsBytesPacket [][]byte

// PooledTransactionsPacket66 is the network packet for transaction distribution over eth/66.
type PooledTransactionsBytesPacket66 struct {
	RequestId uint64
	PooledTransactionsBytesPacket
}

type PooledTransactions PooledTransactionsBytesPacket66

func (msg PooledTransactions) Code() int     { return 26 }
func (msg PooledTransactions) ReqID() uint64 { return msg.RequestId }

// Conn represents an individual connection with a peer
type Conn struct {
	*rlpx.Conn
	consensusEngine            *clmock.CLMocker
	ourKey                     *ecdsa.PrivateKey
	remoteKey                  *ecdsa.PublicKey
	negotiatedProtoVersion     uint
	negotiatedSnapProtoVersion uint
	ourHighestProtoVersion     uint
	ourHighestSnapProtoVersion uint
	caps                       []p2p.Cap
}

func (c *Conn) RemoteKey() *ecdsa.PublicKey {
	return c.remoteKey
}

// Read reads an eth66 packet from the connection.
func (c *Conn) Read() (Message, error) {
	code, rawData, _, err := c.Conn.Read()
	if err != nil {
		return nil, errorf("could not read from connection: %v", err)
	}

	var msg Message
	switch int(code) {
	case (Hello{}).Code():
		msg = new(Hello)
	case (Ping{}).Code():
		msg = new(Ping)
	case (Pong{}).Code():
		msg = new(Pong)
	case (Disconnect{}).Code():
		msg = new(Disconnect)
	case (Status{}).Code():
		msg = new(Status)
	case (GetBlockHeaders{}).Code():
		ethMsg := new(eth.GetBlockHeadersPacket66)
		if err := rlp.DecodeBytes(rawData, ethMsg); err != nil {
			return nil, errorf("could not rlp decode message: %v, %x", err, rawData)
		}
		return (*GetBlockHeaders)(ethMsg), nil
	case (BlockHeaders{}).Code():
		ethMsg := new(eth.BlockHeadersPacket66)
		if err := rlp.DecodeBytes(rawData, ethMsg); err != nil {
			return nil, errorf("could not rlp decode message: %v, %x", err, rawData)
		}
		return (*BlockHeaders)(ethMsg), nil
	case (GetBlockBodies{}).Code():
		ethMsg := new(eth.GetBlockBodiesPacket66)
		if err := rlp.DecodeBytes(rawData, ethMsg); err != nil {
			return nil, errorf("could not rlp decode message: %v, %x", err, rawData)
		}
		return (*GetBlockBodies)(ethMsg), nil
	case (BlockBodies{}).Code():
		ethMsg := new(eth.BlockBodiesPacket66)
		if err := rlp.DecodeBytes(rawData, ethMsg); err != nil {
			return nil, errorf("could not rlp decode message: %v, %x", err, rawData)
		}
		return (*BlockBodies)(ethMsg), nil
	case (NewBlock{}).Code():
		msg = new(NewBlock)
	case (NewBlockHashes{}).Code():
		msg = new(NewBlockHashes)
	case (Transactions{}).Code():
		msg = new(Transactions)
	case (NewPooledTransactionHashes66{}).Code():
		// Try decoding to eth68
		ethMsg := new(NewPooledTransactionHashes)
		if err := rlp.DecodeBytes(rawData, ethMsg); err == nil {
			return ethMsg, nil
		}
		msg = new(NewPooledTransactionHashes66)
	case (GetPooledTransactions{}.Code()):
		ethMsg := new(eth.GetPooledTransactionsPacket66)
		if err := rlp.DecodeBytes(rawData, ethMsg); err != nil {
			return nil, errorf("could not rlp decode message: %v, %x", err, rawData)
		}
		return (*GetPooledTransactions)(ethMsg), nil
	case (PooledTransactions{}.Code()):
		ethMsg := new(PooledTransactionsBytesPacket66)
		if err := rlp.DecodeBytes(rawData, ethMsg); err != nil {
			return nil, errorf("could not rlp decode message: %v, %x", err, rawData)
		}
		return (*PooledTransactions)(ethMsg), nil
	default:
		return nil, errorf("invalid message code: %d", code)
	}

	if msg != nil {
		if err := rlp.DecodeBytes(rawData, msg); err != nil {
			return nil, errorf("could not rlp decode message: %v", err)
		}
		return msg, nil
	}
	return nil, errorf("invalid message: %s", string(rawData))
}

// Write writes a eth packet to the connection.
func (c *Conn) Write(msg Message) (uint32, error) {
	payload, err := rlp.EncodeToBytes(msg)
	if err != nil {
		return 0, err
	}
	return c.Conn.Write(uint64(msg.Code()), payload)
}

// peer performs both the protocol handshake and the status message
// exchange with the node in order to peer with it.
func (c *Conn) Peer(status *Status) error {
	if err := c.handshake(); err != nil {
		return fmt.Errorf("handshake failed: %v", err)
	}
	if _, err := c.statusExchange(status); err != nil {
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
		Caps:    c.caps,
		ID:      pub0,
	}
	if _, err := c.Write(ourHandshake); err != nil {
		return fmt.Errorf("write to connection failed: %v", err)
	}
	// read hello from client
	msg, err := c.Read()
	if err != nil {
		return err
	}
	switch msg := msg.(type) {
	case *Hello:
		// set snappy if version is at least 5
		if msg.Version >= 5 {
			c.SetSnappy(true)
		}
		c.negotiateEthProtocol(msg.Caps)
		if c.negotiatedProtoVersion == 0 {
			return fmt.Errorf("could not negotiate eth protocol (remote caps: %v, local eth version: %v)", msg.Caps, c.ourHighestProtoVersion)
		}
		// If we require snap, verify that it was negotiated
		if c.ourHighestSnapProtoVersion != c.negotiatedSnapProtoVersion {
			return fmt.Errorf("could not negotiate snap protocol (remote caps: %v, local snap version: %v)", msg.Caps, c.ourHighestSnapProtoVersion)
		}
		return nil
	default:
		return fmt.Errorf("bad handshake: %#v", msg)
	}
}

// negotiateEthProtocol sets the Conn's eth protocol version to highest
// advertised capability from peer.
func (c *Conn) negotiateEthProtocol(caps []p2p.Cap) {
	var highestEthVersion uint
	var highestSnapVersion uint
	for _, capability := range caps {
		switch capability.Name {
		case "eth":
			if capability.Version > highestEthVersion && capability.Version <= c.ourHighestProtoVersion {
				highestEthVersion = capability.Version
			}
		case "snap":
			if capability.Version > highestSnapVersion && capability.Version <= c.ourHighestSnapProtoVersion {
				highestSnapVersion = capability.Version
			}
		}
	}
	c.negotiatedProtoVersion = highestEthVersion
	c.negotiatedSnapProtoVersion = highestSnapVersion
}

// statusExchange performs a `Status` message exchange with the given node.
func (c *Conn) statusExchange(status *Status) (Message, error) {
	defer c.SetDeadline(time.Time{})
	c.SetDeadline(time.Now().Add(20 * time.Second))
	localForkID := c.consensusEngine.ForkID()
	// read status message from client
	var message Message
loop:
	for {
		msg, err := c.Read()
		if err != nil {
			return nil, err
		}
		switch msg := msg.(type) {
		case *Status:
			if have, want := msg.Head, c.consensusEngine.LatestHeader.Hash(); have != want {
				return nil, fmt.Errorf("wrong head block in status, want:  %#x (block %d) have %#x",
					want, c.consensusEngine.LatestHeader.Number.Uint64(), have)
			}
			if have, want := msg.TD.Cmp(c.consensusEngine.ChainTotalDifficulty), 0; have != want {
				return nil, fmt.Errorf("wrong TD in status: have %d want %d", have, want)
			}
			if have, want := msg.ForkID, localForkID; !reflect.DeepEqual(have, want) {
				return nil, fmt.Errorf("wrong fork ID in status: have (hash=%#x, next=%d), want (hash=%#x, next=%d)", have.Hash, have.Next, want.Hash, want.Next)
			}
			if have, want := msg.ProtocolVersion, c.ourHighestProtoVersion; have != uint32(want) {
				return nil, fmt.Errorf("wrong protocol version: have %d, want %d", have, want)
			}
			message = msg
			break loop
		case *Disconnect:
			return nil, fmt.Errorf("disconnect received: %v", msg.Reason)
		case *Ping:
			c.Write(&Pong{}) // TODO (renaynay): in the future, this should be an error
			// (PINGs should not be a response upon fresh connection)
		default:
			return nil, fmt.Errorf("bad status message: %s", pretty.Sdump(msg))
		}
	}
	// make sure eth protocol version is set for negotiation
	if c.negotiatedProtoVersion == 0 {
		return nil, errors.New("eth protocol version must be set in Conn")
	}
	if status == nil {
		// default status message
		status = &Status{
			ProtocolVersion: uint32(c.negotiatedProtoVersion),
			NetworkID:       c.consensusEngine.Genesis.Config.ChainID.Uint64(),
			TD:              c.consensusEngine.ChainTotalDifficulty,
			Head:            c.consensusEngine.LatestHeader.Hash(),
			Genesis:         c.consensusEngine.GenesisBlock().Hash(),
			ForkID:          localForkID,
		}
	}
	if _, err := c.Write(status); err != nil {
		return nil, fmt.Errorf("write to connection failed: %v", err)
	}
	return message, nil
}

// readAndServe serves GetBlockHeaders requests while waiting
// on another message from the node.
func (c *Conn) readAndServe(timeout time.Duration) (Message, error) {
	start := time.Now()
	for time.Since(start) < timeout {
		c.SetReadDeadline(time.Now().Add(10 * time.Second))

		msg, err := c.Read()
		if err != nil {
			return nil, err
		}
		switch msg := msg.(type) {
		case *Ping:
			c.Write(&Pong{})
		case *GetBlockHeaders:
			headers, err := c.consensusEngine.GetHeaders(msg.Amount, msg.Origin.Hash, msg.Origin.Number, msg.Reverse, msg.Skip)
			if err != nil {
				return nil, errorf("could not get headers for inbound header request: %v", err)
			}
			resp := &BlockHeaders{
				RequestId:          msg.ReqID(),
				BlockHeadersPacket: eth.BlockHeadersPacket(headers),
			}
			if _, err := c.Write(resp); err != nil {
				return nil, errorf("could not write to connection: %v", err)
			}
		default:
			return msg, nil
		}
	}
	return nil, errorf("no message received within %v", timeout)
}

// headersRequest executes the given `GetBlockHeaders` request.
func (c *Conn) headersRequest(request *GetBlockHeaders, reqID uint64) ([]*types.Header, error) {
	defer c.SetReadDeadline(time.Time{})
	c.SetReadDeadline(time.Now().Add(20 * time.Second))

	// write request
	request.RequestId = reqID
	if _, err := c.Write(request); err != nil {
		return nil, fmt.Errorf("could not write to connection: %v", err)
	}

	// wait for response
	msg, err := c.WaitForResponse(timeout, request.RequestId)
	if err != nil {
		return nil, err
	}
	resp, ok := msg.(*BlockHeaders)
	if !ok {
		return nil, fmt.Errorf("unexpected message received: %s", pretty.Sdump(msg))
	}
	headers := []*types.Header(resp.BlockHeadersPacket)
	return headers, nil
}

// WaitForResponse reads from the connection until a response with the expected
// request ID is received.
func (c *Conn) WaitForResponse(timeout time.Duration, requestID uint64) (Message, error) {
	start := time.Now()
	for {
		msg, err := c.readAndServe(timeout)
		if err != nil {
			return nil, err
		}
		if msg.ReqID() == requestID {
			return msg, nil
		}
		if time.Since(start) > timeout {
			return nil, errorf("message id %d not received within %v", requestID, timeout)
		}
	}
}
