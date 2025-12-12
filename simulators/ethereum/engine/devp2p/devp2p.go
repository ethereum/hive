// Copyright 2021 The go-ethereum Authors
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
	"fmt"
	"net"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/rlpx"
	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
)

var (
	pretty = spew.ConfigState{
		Indent:                  "  ",
		DisableCapacities:       true,
		DisablePointerAddresses: true,
		SortKeys:                true,
	}
	timeout = 20 * time.Second
)

type P2PDest struct {
	*enode.Node
	ConsensusEngine *clmock.CLMocker
}

func PeerEngineClient(engine client.EngineClient, consensusEngine *clmock.CLMocker) (*Conn, error) {
	enodeURL, err := engine.EnodeURL()
	if err != nil {
		return nil, fmt.Errorf("error getting enode: %v", err)
	}
	clientEnode, err := enode.ParseV4(enodeURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing enode: %v", err)
	}
	p2pDest := P2PDest{
		Node:            clientEnode,
		ConsensusEngine: consensusEngine,
	}
	conn, err := p2pDest.Dial()
	if err != nil {
		return nil, fmt.Errorf("error dialing enode: %v", err)
	}

	if err = conn.Peer(); err != nil {
		return nil, err
	}
	return conn, nil
}

// dial attempts to dial the given node and perform a handshake,
// returning the created Conn if successful.
func (d *P2PDest) Dial() (*Conn, error) {
	// dial
	var err error
	fd, err := net.Dial("tcp", fmt.Sprintf("%v:%d", d.IP(), d.TCP()))
	if err != nil {
		return nil, err
	}
	conn := Conn{
		Conn:            rlpx.NewConn(fd, d.Pubkey()),
		consensusEngine: d.ConsensusEngine,
	}
	// do encHandshake
	conn.ourKey, err = crypto.GenerateKey()
	if err != nil {
		return nil, err
	}
	pubKey, err := conn.Handshake(conn.ourKey)
	if err != nil {
		conn.Close()
		return nil, err
	}
	conn.remoteKey = pubKey
	// set default p2p capabilities
	conn.caps = []p2p.Cap{
		{Name: "eth", Version: 69},
	}
	conn.ourHighestProtoVersion = 69
	return &conn, nil
}

// dialSnap creates a connection with snap/1 capability.
func (d *P2PDest) DialSnap() (*Conn, error) {
	conn, err := d.Dial()
	if err != nil {
		return nil, fmt.Errorf("dial failed: %v", err)
	}
	conn.caps = append(conn.caps, p2p.Cap{Name: "snap", Version: 1})
	conn.ourHighestSnapProtoVersion = 1
	return conn, nil
}

// createSendAndRecvConns creates two connections, one for sending messages to the
// node, and one for receiving messages from the node.
func (d *P2PDest) CreateSendAndRecvConns() (*Conn, *Conn, error) {
	sendConn, err := d.Dial()
	if err != nil {
		return nil, nil, fmt.Errorf("dial failed: %v", err)
	}
	recvConn, err := d.Dial()
	if err != nil {
		sendConn.Close()
		return nil, nil, fmt.Errorf("dial failed: %v", err)
	}
	return sendConn, recvConn, nil
}
