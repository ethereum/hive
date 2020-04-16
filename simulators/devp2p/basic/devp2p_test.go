package main

import (
	"crypto/ecdsa"
	"flag"
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethereum/go-ethereum/p2p/nat"
	"github.com/ethereum/go-ethereum/p2p/netutil"
	"github.com/ethereum/hive/simulators/common"
	"github.com/ethereum/hive/simulators/common/providers/hive"
	"github.com/ethereum/hive/simulators/common/providers/local"
	"github.com/ethereum/hive/simulators/devp2p"
)

var (
	listenPort *string //udp listen port
	natdesc    *string //nat mode
	//dockerHost   *string //docker host api endpoint
	//hostURI      *string //simulator host endpoint
	host common.TestSuiteHost

	targetID     *string //docker client id
	nodeKey      *ecdsa.PrivateKey
	err          error
	restrictList *netutil.Netlist
	v4udp        devp2p.V4Udp
	relayIP      net.IP //the ip address of the relay node, used for relaying spoofed traffic
	testSuite    common.TestSuiteID
	iFace        *net.Interface
	localIP      *net.IP
	netInterface *string
)

func init() {
	hive.Support()
	local.Support()
}

func TestMain(m *testing.M) {

	listenPort = flag.String("listenPort", ":30303", "")
	natdesc = flag.String("nat", "any", "port mapping mechanism (any|none|upnp|pmp|extip:<IP>)")
	netInterface = flag.String("interface", "eth0", "the network interface name to use for spoofing traffic (eg: eth0 on docker) or the IP address identifying the network adapter")
	simProviderType := flag.String("simProvider", "", "the simulation provider type (local|hive)")
	providerConfigFile := flag.String("providerConfig", "", "the config json file for the provider")

	flag.Parse()

	host, err = common.InitProvider(*simProviderType, *providerConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to initialise provider %s", err.Error())
		os.Exit(1)
	}

	os.Exit(RunTestSuite(m))
}

func RunTestSuite(m *testing.M) int {

	return m.Run()
}

func ClientTestRunner(t *testing.T, client string, testName string, testDescription string, testFunc func(common.Logger, *enode.Node) (string, bool)) {

	t.Run(testName, func(t *testing.T) {

		t.Parallel()

		// Ask the host to start a new test
		testID, err := host.StartTest(testSuite, testName, testDescription)
		if err != nil {
			t.Fatalf("Unable to start test: %s", err.Error())
		}
		// declare empty test results
		var summaryResult common.TestResult
		var clientResults map[string]*common.TestResult
		//make sure the test ends
		defer func() {
			host.EndTest(testSuite, testID, &summaryResult, clientResults)
			//send test failures to standard outputs too
			if !summaryResult.Pass {
				t.Errorf("Test failed %s", summaryResult.Details)
			}
		}()

		// get a relay pseudo-client
		parms := map[string]string{
			"CLIENT":         "relay",
			"HIVE_RELAY_IP":  localIP.String(),
			"HIVE_RELAY_UDP": "30303",
		}
		_, relayIP, _, err = host.GetPseudo(testSuite, testID, parms)
		if err != nil {
			summaryResult.Pass = false
			summaryResult.AddDetail(fmt.Sprintf("Unable to get pseudo: %s", err.Error()))
			return
		}
		// get a client node
		parms = map[string]string{
			"CLIENT":        client,
			"HIVE_BOOTNODE": "enode://158f8aab45f6d19c6cbf4a089c2670541a8da11978a2f90dbf6a502a4a3bab80d288afdbeb7ec0ef6d92de563767f3b1ea9e8e334ca711e9f8e2df5a0385e8e6@1.2.3.4:30303",
		}
		nodeID, ipAddr, macAddr, err := host.GetNode(testSuite, testID, parms, nil)
		if err != nil {
			summaryResult.Pass = false
			summaryResult.AddDetail(fmt.Sprintf("Unable to get node: %s", err.Error()))
			return
		}

		enodeID, err := host.GetClientEnode(testSuite, testID, nodeID)
		if err != nil {
			summaryResult.Pass = false
			summaryResult.AddDetail(fmt.Sprintf("Unable to get enode: %s", err.Error()))
			return
		}
		if enodeID == nil || *enodeID == "" {
			summaryResult.Pass = false
			summaryResult.AddDetail(fmt.Sprintf("Unable to get enode - client reported blank enodeID"))

		}
		targetNode, err := enode.ParseV4(*enodeID)
		if err != nil {
			summaryResult.Pass = false
			summaryResult.AddDetail(fmt.Sprintf("Unable to get enode: %s", err.Error()))
			return
		}

		if targetNode == nil {
			summaryResult.Pass = false
			summaryResult.AddDetail(fmt.Sprintf("Unable to generate targetNode %s", err.Error()))
			return
		}

		//replace the ip with what docker says it is
		targetNode = MakeNode(targetNode.Pubkey(), ipAddr, targetNode.TCP(), 30303, macAddr)
		resultMessage, ok := testFunc(t, targetNode)
		summaryResult.Pass = ok
		summaryResult.AddDetail(resultMessage)

	})

}

type v4CompatID struct {
	enode.V4ID
}

func (v4CompatID) Verify(r *enr.Record, sig []byte) error {
	var pubkey enode.Secp256k1
	return r.Load(&pubkey)
}

//ripped out from the urlv4 code
func signV4Compat(r *enr.Record, pubkey *ecdsa.PublicKey) {
	r.Set((*enode.Secp256k1)(pubkey))
	if err := r.SetSig(v4CompatID{}, []byte{}); err != nil {
		panic(err)
	}
}

//Make a v4 node based on some info
func MakeNode(pubkey *ecdsa.PublicKey, ip net.IP, tcp, udp int, mac *string) *enode.Node {
	var r enr.Record
	if ip != nil {
		r.Set(enr.IP(ip))
	}
	if udp != 0 {
		r.Set(enr.UDP(udp))
	}
	if tcp != 0 {
		r.Set(enr.TCP(tcp))
	}
	if mac != nil {
		r.Set(common.MacENREntry(*mac))
	}

	signV4Compat(&r, pubkey)
	n, err := enode.New(v4CompatID{}, &r)
	if err != nil {
		panic(err)
	}
	return n

}

// TestDiscovery tests the set of discovery protocols
func TestDiscovery(t *testing.T) {
	//start the test suite
	testSuite, err := host.StartTestSuite("devp2p discovery v4 test suite", "This suite of tests checks for basic conformity to the discovery v4 protocol and for some known security weaknesses.")
	if err != nil {
		t.Fatalf("Simulator error. Failed to start test suite. %v ", err)
	}
	defer host.EndTestSuite(testSuite)
	// discovery v4 test suites
	t.Run("discoveryv4", func(t *testing.T) {

		//setup
		v4udp = setupv4UDP(t)

		//get all client types required to test
		availableClients, err := host.GetClientTypes()
		if err != nil {
			t.Fatalf("Simulator error. Cannot get client types. %v", err)
		}

		//configure a UDP relay for spoof tests
		//the UDP relay plays the role of a 'victim' of an attack
		//where we impersonate their IP. Responses from other nodes, sent to spoofed source IPs
		//are relayed back to us so we can know how other nodes are communicating.

		iFace, err = devp2p.GetNetworkInterface(*netInterface)
		if err != nil {
			t.Fatalf("Simulator error. Cannot get local interface. %v ", err)
		}

		localIP, err = devp2p.GetInterfaceIP(iFace)
		if err != nil {
			t.Fatalf("Simulator error. Cannot get local ip. %v ", err)
		}

		//get all available tests
		availableTests := map[string]func(common.Logger, *enode.Node) (string, bool){
			"spoofTest(v4013)":                            SpoofSanityCheck,
			"spoofAmplificiation(v4014)":                  SpoofAmplificationAttackCheck,
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
				ClientTestRunner(t, i, testName, "TODO", testFunc)

			}

		}

	})

}

//v4013 just makes sure that the network setup works for spoofing
func SpoofSanityCheck(t common.Logger, targetnode *enode.Node) (string, bool) {
	t.Log("Test v4013")
	var mac common.MacENREntry
	targetnode.Load(&mac)
	if err := v4udp.SpoofedPing(targetnode.ID(), string(mac), &net.UDPAddr{IP: targetnode.IP(), Port: targetnode.UDP()}, &net.UDPAddr{IP: relayIP, Port: 30303}, true, nil, *netInterface); err != nil {
		return fmt.Sprintf("Spoofing sanity check failed: %v", err), false
	}
	return "", true
}

//v4014 amplification attack test
func SpoofAmplificationAttackCheck(t common.Logger, targetnode *enode.Node) (string, bool) {
	t.Log("Test v4014")
	var mac common.MacENREntry
	targetnode.Load(&mac)
	if err := v4udp.SpoofingFindNodeCheck(targetnode.ID(), string(mac), &net.UDPAddr{IP: targetnode.IP(), Port: targetnode.UDP()}, &net.UDPAddr{IP: relayIP, Port: 30303}, true, *netInterface); err != devp2p.ErrTimeout {
		return fmt.Sprintf(" test failed: %v", err), false
	}
	return "", true
}

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

	//wrap this 'listener' into a conn
	//but
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
