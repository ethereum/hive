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
	flag.Parse()

	testTarget := flag.String("enodeTarget", "enode://3f1d12044546b76342d59d4a05532c14b85aa669704bfe1f864fe079415aa2c02d743e03218e57a33fb94523adb54032871a6c51b2cc5514cb7c7e35b3ed0a99@13.93.211.84:30303", "Enode address of target")
	natdesc = flag.String("nat", "none", "port mapping mechanism (any|none|upnp|pmp|extip:<IP>)")
	targetnode, err = enode.ParseV4(*testTarget)
	if err != nil {
		panic(err)
	}
	listenPort = flag.String("listenPort", ":30303", "")

	os.Exit(m.Run())
}

// TestDiscovery tests the set of discovery protocols
func TestDiscovery(t *testing.T) {
	// discovery v4 test suites
	t.Run("discoveryv4", func(t *testing.T) {
		//setup
		setupv4UDP()

		t.Run("ping", func(t *testing.T) {

			//with the use of helper functions

			//.signal that the other hive client should be reset
			//TODO

			//.get the endpoint of the other hive client
			//TODO

			//.generate a ping message and send to the hive client
			//connect to remote endpoint

			//v4UDP.ping(...)
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
