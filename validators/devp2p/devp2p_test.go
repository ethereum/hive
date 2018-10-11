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
)

func TestMain(m *testing.M) {

	testTarget := flag.String("enodeTarget", "", "Enode address of target")
	testTargetIP := flag.String("targetIP", "", "IP Address of hive container client")
	listenPort = flag.String("listenPort", ":30303", "")
	natdesc = flag.String("nat", "none", "port mapping mechanism (any|none|upnp|pmp|extip:<IP>)")
	flag.Parse()

	//testing
	*testTarget = "enode://158f8aab45f6d19c6cbf4a089c2670541a8da11978a2f90dbf6a502a4a3bab80d288afdbeb7ec0ef6d92de563767f3b1ea9e8e334ca711e9f8e2df5a0385e8e6@13.75.154.138:30303"
	//*testTargetIP = "13.75.154.138"

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
		v4udp := setupv4UDP()

		//If the client has a known enode, obtained from an admin API, then run a standard ping
		//Otherwise, run a different ping where we override any enode validation checks
		//The recovered id can be used to set the target node id for any further tests that might want to verify that.
		var pingTest func(t *testing.T)

		if targetnode == nil {
			t.Log("Pinging unknown node id.")
			pingTest = func(t *testing.T) {
				if err := v4udp.ping(enode.ID{}, &net.UDPAddr{IP: targetip, Port: 30303}, false, func(e *ecdsa.PublicKey) {
					targetnode = enode.NewV4(e, targetip, 30303, 30303)
				}); err != nil {
					t.Fatalf("Unable to v4 ping: %v", err)
				}
			}
		} else {
			t.Log("Pinging known node id.")
			pingTest = func(t *testing.T) {
				if err := v4udp.ping(targetnode.ID(), &net.UDPAddr{IP: targetnode.IP(), Port: targetnode.UDP()}, true, nil); err != nil {
					t.Fatalf("Unable to v4 ping: %v", err)
				}
			}
		}

		t.Run("ping", pingTest)
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
