package main

import (
	"crypto/ecdsa"
	"flag"
	"net"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/nat"
	"github.com/ethereum/go-ethereum/p2p/netutil"
)

var (
	listenPort *string     // udp listen port
	natdesc    *string     //nat mode
	targetnode *enode.Node // parsed Node

	nodeKey      *ecdsa.PrivateKey
	err          error
	restrictList *netutil.Netlist
)

func TestMain(m *testing.M) {

	//testTarget := flag.String("enodeTarget", "enode://158f8aab45f6d19c6cbf4a089c2670541a8da11978a2f90dbf6a502a4a3bab80d288afdbeb7ec0ef6d92de563767f3b1ea9e8e334ca711e9f8e2df5a0385e8e6@13.75.154.138:30303", "Enode address of target")
	testTarget := flag.String("enodeTarget", "enode://158f8aab45f6d19c6cbf4a089c2670541a8da11978a2f90dbf6a502a4a3bab80d288afdbeb7ec0ef6d92de563767f3b1ea9e8e334ca711e9f8e2df5a0385e8e6@172.17.0.2:30303", "Enode address of target")
	testTargetIP := flag.String("targetIP", "", "IP Address of hive container client")
	natdesc = flag.String("nat", "none", "port mapping mechanism (any|none|upnp|pmp|extip:<IP>)")

	flag.Parse()

	targetnode, err = enode.ParseV4(*testTarget)
	if err != nil {
		panic(err)
	}
	if *testTargetIP != "" {
		ip := net.ParseIP(*testTargetIP)
		targetnode = enode.NewV4(targetnode.Pubkey(), ip, targetnode.TCP(), targetnode.UDP())
	}

	listenPort = flag.String("listenPort", ":30303", "")

	os.Exit(m.Run())
}

// TestDiscovery tests the set of discovery protocols
func TestDiscovery(t *testing.T) {
	// discovery v4 test suites
	t.Run("discoveryv4", func(t *testing.T) {
		//setup
		v4udp := setupv4UDP()

		t.Run("ping", func(t *testing.T) {

			//with the use of helper functions
			//.signal that the other hive client should be reset?
			//TODO

			if err := v4udp.ping(targetnode.ID(), &net.UDPAddr{IP: targetnode.IP(), Port: targetnode.UDP()}); err != nil {
				t.Fatalf("Unable to v4 ping: %v", err)
			}

		})
	})

	t.Run("discoveryv5", func(t *testing.T) {

		t.Run("ping", func(t *testing.T) {
			//TODO
		})
	})

}

// TestRLPx checks the RLPx handshaking
func TestRLPx(t *testing.T) {
	// discovery v4 test suites
	t.Run("connect", func(t *testing.T) {
		//
		t.Run("basic", func(t *testing.T) {

		})
	})

}

func setupv4UDP() V4Udp {
	//Resolve an address (eg: ":port") to a UDP endpoint.
	addr, err := net.ResolveUDPAddr("udp", *listenPort)
	if err != nil {
		panic(err)
	}

	//Create a UDP connection
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		utils.Fatalf("-ListenUDP: %v", err)
	}

	//FS: The following just gets the local address, does something with NAT and converts into a
	//general address type.
	natm, err := nat.Parse(*natdesc)
	if err != nil {
		utils.Fatalf("-nat: %v", err)
	}
	realaddr := conn.LocalAddr().(*net.UDPAddr)
	if natm != nil {
		if !realaddr.IP.IsLoopback() {
			go nat.Map(natm, nil, "udp", realaddr.Port, realaddr.Port, "ethereum discovery")
		}
		// TODO: react to external IP changes over time.
		if ext, err := natm.ExternalIP(); err == nil {
			realaddr = &net.UDPAddr{IP: ext, Port: realaddr.Port}
		}
	}

	nodeKey, err = crypto.GenerateKey()

	if err != nil {
		utils.Fatalf("could not generate key: %v", err)
	}

	cfg := Config{
		PrivateKey:   nodeKey,
		AnnounceAddr: realaddr,
		NetRestrict:  restrictList,
	}

	var v4UDP *V4Udp

	if v4UDP, err = ListenUDP(conn, cfg); err != nil {
		panic(err)
	}

	return *v4UDP
}
