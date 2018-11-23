package main

import (
	"crypto/ecdsa"
	"flag"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/nat"
	"github.com/ethereum/go-ethereum/p2p/netutil"
	"github.com/ethereum/hive/simulators/common"
	"github.com/ethereum/hive/simulators/devp2p"
	docker "github.com/fsouza/go-dockerclient"
)

var (
	listenPort   *string //udp listen port
	natdesc      *string //nat mode
	dockerHost   *string //docker host api endpoint
	hostURI      *string //simulator host endpoint
	host         common.SimulatorAPI
	daemon       *docker.Client //docker daemon proxy
	targetID     *string        //docker client id
	nodeKey      *ecdsa.PrivateKey
	err          error
	restrictList *netutil.Netlist
	v4udp        devp2p.V4Udp

	//targetnode       *enode.Node // parsed Node
	//targetIP         net.IP      //targetIP

)

type testCase struct {
	Client string
}

func TestMain(m *testing.M) {

	//Max Concurrency is specified in the parallel flag, which is supplied to the simulator container

	listenPort = flag.String("listenPort", ":30303", "")
	natdesc = flag.String("nat", "none", "port mapping mechanism (any|none|upnp|pmp|extip:<IP>)")
	hostURI = flag.String("simulatorHost", "", "url of simulator host api")
	dockerHost = flag.String("dockerHost", "", "docker host api endpoint")

	flag.Parse()

	//Try to connect to the simulator host and get the client list
	host = &common.SimulatorHost{
		HostURI: hostURI,
	}

	os.Exit(m.Run())
}

func ClientTestRunner(t *testing.T, client string, testName string, testFunc func(common.Logger, *enode.Node) (string, bool)) {

	t.Run(testName, func(t *testing.T) {

		t.Parallel()

		var startTime = time.Now()
		var errorMessage string
		var ok = true

		parms := map[string]string{
			"CLIENT":        client,
			"HIVE_BOOTNODE": "enode://158f8aab45f6d19c6cbf4a089c2670541a8da11978a2f90dbf6a502a4a3bab80d288afdbeb7ec0ef6d92de563767f3b1ea9e8e334ca711e9f8e2df5a0385e8e6@1.2.3.4:30303",
		}

		nodeID, err := host.StartNewNode(parms)
		if err != nil {
			errorMessage = fmt.Sprintf("FATAL: Unable to start node: %v", err)
			ok = false
		}

		if ok {

			ip, err := host.GetClientIP(*nodeID)
			if err != nil {
				errorMessage = fmt.Sprintf("FATAL: Unable to get client IP: %v", err)
				ok = false
			}

			enodeID, err := host.GetClientEnode(*nodeID)
			if err != nil || enodeID == nil || *enodeID == "" {
				errorMessage = fmt.Sprintf("FATAL: Unable to get node: %v", err)
				ok = false
			}
			t.Logf("Got enode for test %s", *enodeID)

			targetNode, err := enode.ParseV4(*enodeID)
			if err != nil {
				errorMessage = fmt.Sprintf("FATAL: Unable to parse enode: %v", err)
				ok = false
			}

			ipAddr := net.ParseIP(*ip)
			if ipAddr == nil {
				errorMessage = fmt.Sprintf("FATAL: Unable to parse IP: %v", err)
				ok = false
			}

			//replace the ip with what docker says it is
			targetNode = enode.NewV4(targetNode.Pubkey(), ipAddr, targetNode.TCP(), 30303) //targetNode.UDP())
			if targetNode == nil {
				errorMessage = fmt.Sprintf("FATAL: Unable to generate targetNode: %v", err)
				ok = false
			}
			if ok {
				errorMessage, ok = testFunc(t, targetNode)
			}
		}

		host.AddResults(ok, *nodeID, testName, errorMessage, time.Since(startTime))

		if !ok {
			t.Errorf("Test failed: %s", errorMessage)
		}

	})

}

// TestDiscovery tests the set of discovery protocols
func TestDiscovery(t *testing.T) {

	// discovery v4 test suites
	t.Run("discoveryv4", func(t *testing.T) {

		//setup
		v4udp = setupv4UDP(t)

		//get all client types required to test
		availableClients, err := host.GetClientTypes()
		if err != nil {
			t.Fatalf("Simulator error. Cannot get client types. %v", err)
		}

		//get all available tests
		availableTests := map[string]func(common.Logger, *enode.Node) (string, bool){
			"pingTest(v4001)":                             SourceUnknownPingKnownEnode,
			"SourceUnknownPingWrongTo(v4002)":             SourceUnknownPingWrongTo,
			"SourceUnknownPingWrongFrom(v4003)":           SourceUnknownPingWrongFrom,
			"SourceUnknownPingExtraData(v4004)":           SourceUnknownPingExtraData,
			"SourceUnknownPingExtraDataWrongFrom(v4005)":  SourceUnknownPingExtraDataWrongFrom,
			"SourceUnknownWrongPacketType(v4006)":         SourceUnknownWrongPacketType,
			"SourceUnknownFindNeighbours(v4007)":          SourceUnknownFindNeighbours,
			"SourceKnownPingFromSignatureMismatch(v4009)": SourceKnownPingFromSignatureMismatch,
			"FindNeighboursOnRecentlyBondedTarget(v4010)": FindNeighboursOnRecentlyBondedTarget,
			"PingPastExpiration(v4011)":                   PingPastExpiration,
			"FindNeighboursPastExpiration(v4012)":         FindNeighboursPastExpiration,
		}

		//for every client type
		for _, i := range availableClients {

			//for every test
			for testName, testFunc := range availableTests {

				//we have a testcase of client-type+test
				//run that testcase with a helper function (client, testfunc)
				//the testcase will be run with max concurrency specified by the test parallel flag
				ClientTestRunner(t, i, testName, testFunc)

			}

		}

	})

}

//v4001a - Temporarily removing as this case no longer acceptable?
// func SourceUnknownPingUnknownEnode(t common.Logger, targetIP net.IP) (bool, string) {

// 	t.Log("Pinging unknown node id.")
// 	if err := v4udp.Ping(enode.ID{}, &net.UDPAddr{IP: targetIP, Port: 30303}, false, func(e *ecdsa.PublicKey) {

// 		targetnode = enode.NewV4(e, targetIP, 30303, 30303)
// 		t.Log("Discovered node id " + targetnode.String())
// 	}); err != nil {
// 		t.Fatalf("Unable to v4 ping: %v", err)
// 	}
// }

//v4001b
func SourceUnknownPingKnownEnode(t common.Logger, targetnode *enode.Node) (string, bool) {
	t.Log("Test v4001")
	if err := v4udp.Ping(targetnode.ID(), &net.UDPAddr{IP: targetnode.IP(), Port: targetnode.UDP()}, true, nil); err != nil {
		return fmt.Sprintf("Ping test failed: %v", err), false
	}
	return "", true
}

//v4002
func SourceUnknownPingWrongTo(t common.Logger, targetnode *enode.Node) (string, bool) {
	t.Log("Test v4002")
	if err := v4udp.PingWrongTo(targetnode.ID(), &net.UDPAddr{IP: targetnode.IP(), Port: targetnode.UDP()}, true, nil); err != nil {
		return fmt.Sprintf("Test failed: %v", err), false
	}
	return "", true
}

//v4003
func SourceUnknownPingWrongFrom(t common.Logger, targetnode *enode.Node) (string, bool) {
	t.Log("Test v4003")
	if err := v4udp.PingWrongFrom(targetnode.ID(), &net.UDPAddr{IP: targetnode.IP(), Port: targetnode.UDP()}, true, nil); err != nil {
		return fmt.Sprintf("Test failed: %v", err), false
	}
	return "", true
}

//v4004
func SourceUnknownPingExtraData(t common.Logger, targetnode *enode.Node) (string, bool) {
	t.Log("Test v4004")
	if err := v4udp.PingExtraData(targetnode.ID(), &net.UDPAddr{IP: targetnode.IP(), Port: targetnode.UDP()}, true, nil); err != nil {
		return fmt.Sprintf("Test failed: %v", err), false
	}
	return "", true
}

//v4005
func SourceUnknownPingExtraDataWrongFrom(t common.Logger, targetnode *enode.Node) (string, bool) {
	t.Log("Test v4005")
	if err := v4udp.PingExtraDataWrongFrom(targetnode.ID(), &net.UDPAddr{IP: targetnode.IP(), Port: targetnode.UDP()}, true, nil); err != nil {
		return fmt.Sprintf("Test failed: %v", err), false
	}
	return "", true
}

//v4006
func SourceUnknownWrongPacketType(t common.Logger, targetnode *enode.Node) (string, bool) {
	t.Log("Test v4006")
	if err := v4udp.PingTargetWrongPacketType(targetnode.ID(), &net.UDPAddr{IP: targetnode.IP(), Port: targetnode.UDP()}, true, nil); err != devp2p.ErrTimeout {
		return fmt.Sprintf("Test failed: %v", err), false
	}
	return "", true
}

//v4007
func SourceUnknownFindNeighbours(t common.Logger, targetnode *enode.Node) (string, bool) {
	t.Log("Test v4007")
	targetEncKey := devp2p.EncodePubkey(targetnode.Pubkey())
	if err := v4udp.FindnodeWithoutBond(targetnode.ID(), &net.UDPAddr{IP: targetnode.IP(), Port: targetnode.UDP()}, targetEncKey); err != devp2p.ErrTimeout {
		return fmt.Sprintf("Test failed: %v", err), false
	}
	return "", true
}

//v4009
func SourceKnownPingFromSignatureMismatch(t common.Logger, targetnode *enode.Node) (string, bool) {

	t.Log("Test v4009")
	if err := v4udp.PingBondedWithMangledFromField(targetnode.ID(), &net.UDPAddr{IP: targetnode.IP(), Port: targetnode.UDP()}, true, nil); err != nil {
		return fmt.Sprintf("Test failed: %v", err), false
	}
	return "", true

}

//v4010
func FindNeighboursOnRecentlyBondedTarget(t common.Logger, targetnode *enode.Node) (string, bool) {
	t.Log("Test v4010")
	targetEncKey := devp2p.EncodePubkey(targetnode.Pubkey())
	if err := v4udp.BondedSourceFindNeighbours(targetnode.ID(), &net.UDPAddr{IP: targetnode.IP(), Port: targetnode.UDP()}, targetEncKey); err != nil {
		return fmt.Sprintf("Test failed: %v", err), false
	}
	return "", true
}

//v4011
func PingPastExpiration(t common.Logger, targetnode *enode.Node) (string, bool) {
	t.Log("Test v4011")
	if err := v4udp.PingPastExpiration(targetnode.ID(), &net.UDPAddr{IP: targetnode.IP(), Port: targetnode.UDP()}, true, nil); err != devp2p.ErrTimeout {
		return fmt.Sprintf("Test failed: %v", err), false
	}
	return "", true
}

//v4012
func FindNeighboursPastExpiration(t common.Logger, targetnode *enode.Node) (string, bool) {
	t.Log("Test v4012")
	targetEncKey := devp2p.EncodePubkey(targetnode.Pubkey())
	if err := v4udp.BondedSourceFindNeighboursPastExpiration(targetnode.ID(), &net.UDPAddr{IP: targetnode.IP(), Port: targetnode.UDP()}, targetEncKey); err != devp2p.ErrTimeout {
		return fmt.Sprintf("Test failed: %v", err), false
	}
	return "", true
}

func setupv4UDP(l common.Logger) devp2p.V4Udp {

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

	//The following just gets the local address, does something with NAT and converts into a
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

		if ext, err := natm.ExternalIP(); err == nil {
			realaddr = &net.UDPAddr{IP: ext, Port: realaddr.Port}
		}
	}

	nodeKey, err = crypto.GenerateKey()

	if err != nil {
		utils.Fatalf("could not generate key: %v", err)
	}

	cfg := devp2p.Config{
		PrivateKey:   nodeKey,
		AnnounceAddr: realaddr,
		NetRestrict:  restrictList,
	}

	var v4UDP *devp2p.V4Udp

	if v4UDP, err = devp2p.ListenUDP(conn, cfg, l); err != nil {
		panic(err)
	}

	return *v4UDP
}
