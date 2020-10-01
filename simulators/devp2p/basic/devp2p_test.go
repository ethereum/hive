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

func TestMain(m *testing.M) {
	listenPort = flag.String("listenPort", ":30303", "")
	natdesc = flag.String("nat", "any", "port mapping mechanism (any|none|upnp|pmp|extip:<IP>)")
	netInterface = flag.String("interface", "eth0", "the network interface name to use for spoofing traffic (eg: eth0 on docker) or the IP address identifying the network adapter")

	flag.Parse()

	host = hive.New()

	os.Exit(RunTestSuite(m))
}

func RunTestSuite(m *testing.M) int {

	return m.Run()
}

type testFn func(common.Logger, *enode.Node) (string, bool)

func ClientTestRunner(t *testing.T, client string, testName string, testDescription string, testFunc testFn) {
	id, _ := os.LookupEnv("HIVE_CONTAINER_ID")
	fmt.Fprintf(os.Stderr, "Environment container id: %s\n", id)

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
	logFile, _ := os.LookupEnv("HIVE_SIMLOG")
	//start the test suite
	testSuite, err := host.StartTestSuite("Devp2p discovery v4 test suite",
		"This suite of tests checks for basic conformity to the discovery v4 protocol and for some known security weaknesses.",
		logFile)
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
		type test struct {
			name        string
			testFunc    testFn
			description string
		}
		availableTests := []test{
			{"SpoofSanityCheck(v4013)",
				SpoofSanityCheck,
				"A sanity check to make sure that the network setup works for spoofing"},
			{"SpoofAmplification(v4014)",
				SpoofAmplificationAttackCheck,
				`v4014 amplification attack test: The test spoofs a ping from a different ip address('victim ip'),and later on sends a spoofed 'pong'. 

The spoofed 'pong' will have an invalid reply-token (since the target will send the 'pong' to 'victim ip'), and should thus be ignored by the target.
If the target fails to verify reply-token, it will believe to now be bonded with a node at 'victim ip'. 

The test then sends a spoofed 'findnode' request. 
If the 'target' responds to the findnode-request, sending a large'neighbours'-packet to 'victim ip', it can be used for DoS attacks.`},
			{"Ping-BasicTest(v4001)",
				SourceUnknownPingKnownEnode,
				"Sends a 'ping' from an unknown source, expects a 'pong' back"},
			{"Ping-SourceUnknownrongTo(v4002)",
				SourceUnknownPingWrongTo,
				"Does a ping with incorrect 'to', expects a pong back"},
			{"Ping-SourceUnknownWrongFrom(v4003)",
				SourceUnknownPingWrongFrom,
				"Sends a 'ping' with incorrect from field. Expect a valid 'pong' back - a bad 'from' should be ignored"},
			{"Ping-SourceUnknownExtraData(v4004)",
				SourceUnknownPingExtraData,
				"Sends a 'ping' with a 'future format' packet containing extra fields. Expects a valid 'pong' back"},
			{"Ping-SourceUnknownExtraDataWrongFrom(v4005)",
				SourceUnknownPingExtraDataWrongFrom,
				"Sends 'ping' with a 'future format' packet containing extra fields and make sure it works even with the wrong 'from' field. Expects a valid 'pong' back"},
			{"Ping-SourceUnknownWrongPacketType(v4006)",
				SourceUnknownWrongPacketType,
				"PingTargetWrongPacketType send a packet (a ping packet, though it could be something else) with an unknown packet type to the client and" +
					"see how the target behaves. Expects the target to not send any kind of response."},
			{"Findnode-UnbondedFindNeighbours(v4007)",
				SourceUnknownFindNeighbours,
				"Queries neighbours, from a node without a previous bond. This tests expects no response to be sent from the target."},
			{"Ping-BondedFromSignatureMismatch(v4009)",
				SourceKnownPingFromSignatureMismatch,
				"Ping node under test, from an already bonded node, but the 'ping' has a bad from-field. " +
					"Expects the target to ignore the bad 'from' and respond with a valid pong."},
			{"FindNode-UnsolicitedPollution(v4010)",
				FindNeighboursOnRecentlyBondedTarget,
				"Node A bonds with the target. The target receives an unsolicited 'neighbours' packet, containing junk information. When node A" +
					"queries neighbours('findnode'), we expect to get back a response which is not polluted."},
			{"Ping-PastExpiration(v4011)",
				PingPastExpiration,
				"Sends a 'ping' with past expiration, expects no response from the target."},
			{"FindNode-PastExpiration(v4012)",
				FindNeighboursPastExpiration,
				"Sends a 'findnode' with past expiration, expects no response from the target."},
		}

		//for every client type
		for _, i := range availableClients {
			//for every test
			for _, test := range availableTests {
				//the testcase will be run with max concurrency specified by the test parallel flag
				ClientTestRunner(t, i, test.name, test.description, test.testFunc)
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
