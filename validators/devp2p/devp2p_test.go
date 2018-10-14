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
	listenPort   *string     // udp listen port
	natdesc      *string     //nat mode
	targetnode   *enode.Node // parsed Node
	targetip     net.IP      //targetIP
	nodeKey      *ecdsa.PrivateKey
	err          error
	restrictList *netutil.Netlist
	v4udp        V4Udp
)

func TestMain(m *testing.M) {

	testTarget := flag.String("enodeTarget", "", "Enode address of target")
	testTargetIP := flag.String("targetIP", "", "IP Address of hive container client")
	listenPort = flag.String("listenPort", ":30303", "")
	natdesc = flag.String("nat", "none", "port mapping mechanism (any|none|upnp|pmp|extip:<IP>)")
	flag.Parse()

	//If an enode was supplied, use that
	if *testTarget != "" {
		targetnode, err = enode.ParseV4(*testTarget)
		if err != nil {
			panic(err)
		}
	}

	//If a target ip was supplied, parse it and use it
	if *testTargetIP != "" {
		targetip = net.ParseIP(*testTargetIP)
		//if the target enode was supplied, override the ip address with the target ip supplied, which
		//seems to be useful when the supplied enode ip address is incorrect in some way when reported
		//from a docker container
		if targetnode != nil {
			targetnode = enode.NewV4(targetnode.Pubkey(), targetip, targetnode.TCP(), targetnode.UDP())
		}
	}

	//Exit if no args supplied
	if *testTargetIP == "" && targetnode == nil {
		panic("No target enode or ip supplied")
	}

	os.Exit(m.Run())
}

// TestDiscovery tests the set of discovery protocols
func TestDiscovery(t *testing.T) {
	// discovery v4 test suites
	t.Run("discoveryv4", func(t *testing.T) {
		//setup
		v4udp = setupv4UDP()

		//If the client has a known enode, obtained from an admin API, then run a standard ping
		//Otherwise, run a different ping where we override any enode validation checks
		//The recovered id can be used to set the target node id for any further tests that might want to verify that.
		var pingTest func(t *testing.T)

		if targetnode == nil {
			pingTest = SourceUnknownPingUnknownEnode
		} else {
			pingTest = SourceUnknownPingKnownEnode
		}

		t.Run("v4001", pingTest)
		t.Run("v4002", SourceUnknownPingWrongTo)
		t.Run("v4003", SourceUnknownPingWrongFrom)
		t.Run("v4004", SourceUnknownPingExtraData)
		t.Run("v4005", SourceUnknownPingExtraDataWrongFrom)
		t.Run("v4006", SourceUnknownPingExtraDataWrongTo)
		t.Run("v4007", SourceUnknownFindNeighbours)
		t.Run("v4008", SourceUnknownUnsolicitedNeighbours)
		t.Run("v4009", SourceKnownPingWrongTo)
		t.Run("v4010", SourceKnownPingFromSignatureMismatch)
		t.Run("v4011", SourceKnownSignaturePingFromMismatch)
		t.Run("v4012", FindNeighboursOnRecentlyBondedTarget)
		t.Run("v4013", FindNeighboursOnOldBondedTarget)
		t.Run("v4014", PingPastExpiration)
		t.Run("v4015", FindNeighboursPastExpiration)

	})

	t.Run("discoveryv5", func(t *testing.T) {

		t.Run("ping", func(t *testing.T) {
			//TODO
		})
	})

}

//v4001a
func SourceUnknownPingUnknownEnode(t *testing.T) {
	t.Log("Pinging unknown node id.")
	if err := v4udp.ping(enode.ID{}, &net.UDPAddr{IP: targetip, Port: 30303}, false, func(e *ecdsa.PublicKey) {

		targetnode = enode.NewV4(e, targetip, 30303, 30303)
		t.Log("Discovered node id " + targetnode.String())
	}); err != nil {
		t.Fatalf("Unable to v4 ping: %v", err)
	}
}

//v4001b
func SourceUnknownPingKnownEnode(t *testing.T) {
	t.Log("Pinging known node id.")
	if err := v4udp.ping(targetnode.ID(), &net.UDPAddr{IP: targetnode.IP(), Port: targetnode.UDP()}, true, nil); err != nil {
		t.Fatalf("Unable to v4 ping: %v", err)
	}
}

//v4002
func SourceUnknownPingWrongTo(t *testing.T) {
	t.Log("Pinging with incorrect target endpoint.")

}

//v4003
func SourceUnknownPingWrongFrom(t *testing.T) {
	t.Log("Pinging with incorrect sender.")

}

//v4004
func SourceUnknownPingExtraData(t *testing.T) {

}

//v4005
func SourceUnknownPingExtraDataWrongFrom(t *testing.T) {

}

//v4006
func SourceUnknownPingExtraDataWrongTo(t *testing.T) {

}

//v4007
func SourceUnknownFindNeighbours(t *testing.T) {

}

//v4008
func SourceUnknownUnsolicitedNeighbours(t *testing.T) {

}

//v4009
func SourceKnownPingWrongTo(t *testing.T) {

}

//v4010
func SourceKnownPingFromSignatureMismatch(t *testing.T) {

}

//v4011
func SourceKnownSignaturePingFromMismatch(t *testing.T) {

}

//v4012
func FindNeighboursOnRecentlyBondedTarget(t *testing.T) {

}

//v4013
func FindNeighboursOnOldBondedTarget(t *testing.T) {

}

//v4014
func PingPastExpiration(t *testing.T) {

}

//v4015
func FindNeighboursPastExpiration(t *testing.T) {

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
