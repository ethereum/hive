package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"

	"github.com/ethereum/hive/hivesim"
	"github.com/shogo82148/go-tap"
)

// Location of the test chain files. They are copied from
// go-ethereum/cmd/devp2p/internal/ethtest/testdata by the simulator dockerfile.
const testChainDir = "/testchain"

func main() {
	discv4 := hivesim.Suite{
		Name:        "discv4",
		Description: "This suite runs Discovery v4 protocol tests.",
		Tests: []hivesim.AnyTest{
			hivesim.ClientTestSpec{
				Role: "eth1",
				Parameters: hivesim.Params{
					"HIVE_NETWORK_ID":     "19763",
					"HIVE_CHAIN_ID":       "19763",
					"HIVE_FORK_HOMESTEAD": "0",
					"HIVE_FORK_TANGERINE": "0",
					"HIVE_FORK_SPURIOUS":  "0",
					"HIVE_FORK_BYZANTIUM": "0",
					"HIVE_LOGLEVEL":       "5",
				},
				AlwaysRun: true,
				Run:       runDiscv4Test,
			},
		},
	}

	discv5 := hivesim.Suite{
		Name:        "discv5",
		Description: "This suite runs Discovery v5 protocol tests.",
		Tests: []hivesim.AnyTest{
			hivesim.ClientTestSpec{
				Role: "eth1",
				Parameters: hivesim.Params{
					"HIVE_NETWORK_ID":     "19763",
					"HIVE_CHAIN_ID":       "19763",
					"HIVE_FORK_HOMESTEAD": "0",
					"HIVE_FORK_TANGERINE": "0",
					"HIVE_FORK_SPURIOUS":  "0",
					"HIVE_FORK_BYZANTIUM": "0",
					"HIVE_LOGLEVEL":       "5",
					"HIVE_DISCV5":         "1",
				},
				AlwaysRun: true,
				Run: func(t *hivesim.T, c *hivesim.Client) {
					runDiscv5Test(t, c, (*hivesim.Client).EnodeURL)
				},
			},
			hivesim.ClientTestSpec{
				Role: "beacon",
				Parameters: hivesim.Params{
					"HIVE_LOGLEVEL":        "5",
					"HIVE_CHECK_LIVE_PORT": "4000",
				},
				Files: map[string]string{
					"/hive/input/genesis.ssz": "./init/beacon/genesis.ssz",
					"/hive/input/config.yaml": "./init/beacon/config.yaml",
				},
				AlwaysRun: true,
				Run: func(t *hivesim.T, c *hivesim.Client) {
					runDiscv5Test(t, c, getBeaconENR)
				},
			},
			hivesim.ClientTestSpec{
				Role: "portal",
				Parameters: hivesim.Params{
					"HIVE_CHECK_LIVE_PORT": "8545",
				},
				AlwaysRun: true,
				Run: func(t *hivesim.T, c *hivesim.Client) {
					runDiscv5Test(t, c, getPortalENR)
				},
			},
		},
	}

	forkenv := loadTestChainConfig()
	eth := hivesim.Suite{
		Name:        "eth",
		Description: "This suite tests a client's ability to accurately respond to basic eth protocol messages.",
		Tests: []hivesim.AnyTest{
			hivesim.ClientTestSpec{
				Role: "eth1",
				Name: "client launch",
				Description: `This test launches the client and runs the test tool.
Results from the test tool are reported as individual sub-tests.`,
				Parameters: forkenv,
				Files: map[string]string{
					"genesis.json": testChainDir + "/genesis.json",
					"chain.rlp":    testChainDir + "/chain.rlp",
				},
				AlwaysRun: true,
				Run:       runEthTest,
			},
		},
	}

	snap := hivesim.Suite{
		Name:        "snap",
		Description: "This suite tests the snap protocol.",
		Tests: []hivesim.AnyTest{
			hivesim.ClientTestSpec{
				Role: "eth1",
				Name: "client launch",
				Description: `This test launches the client and runs the test tool.
Results from the test tool are reported as individual sub-tests.`,
				Parameters: forkenv,
				Files: map[string]string{
					"genesis.json": testChainDir + "/genesis.json",
					"chain.rlp":    testChainDir + "/chain.rlp",
				},
				AlwaysRun: true,
				Run:       runSnapTest,
			},
		},
	}

	snap2 := hivesim.Suite{
		Name:        "snap2",
		Description: "This suite tests the snap/2 protocol (EIP-8189): BAL-based state healing.",
		Tests: []hivesim.AnyTest{
			hivesim.ClientTestSpec{
				Role: "eth1",
				Name: "client launch",
				Description: `This test launches the client and runs the snap/2 test tool.
Results from the test tool are reported as individual sub-tests.`,
				Parameters: forkenv,
				Files: map[string]string{
					"genesis.json": testChainDir + "/genesis.json",
					"chain.rlp":    testChainDir + "/chain.rlp",
				},
				AlwaysRun: true,
				Run:       runSnap2Test,
			},
		},
	}

	eip8288env := loadChainConfig(eip8288ChainDir)
	// A node that never selects anything to aggregate must still admit and relay wrappers.
	eip8288NoAggregation := eip8288env.Set("HIVE_EIP8288_AGGREGATION_PROBABILITY", "0")
	eip8288PreFork := eip8288env.Set("HIVE_EIP8288_TIMESTAMP", eip8288PreForkTimestamp)
	eth75 := hivesim.Suite{
		Name:        "eth75",
		Description: "This suite tests eth/75 (EIP-8437) propagation of EIP-8288 proof wrappers.",
		Tests: []hivesim.AnyTest{
			hivesim.ClientTestSpec{
				Role: "eth1",
				Name: "client launch",
				Description: `This test launches the client on a chain with EIP-8288 active and runs the test tool.
Results from the test tool are reported as individual sub-tests.`,
				Parameters: eip8288NoAggregation,
				Files: map[string]string{
					"genesis.json": eip8288ChainDir + "/genesis.json",
					"chain.rlp":    eip8288ChainDir + "/chain.rlp",
				},
				AlwaysRun: true,
				Run:       runRLPxTest("eth75-test"),
			},
			hivesim.ClientTestSpec{
				Role: "eth1",
				Name: "client launch before fork",
				Description: `This test launches the client on a chain that schedules EIP-8288 in the future
and runs the test tool. Results from the test tool are reported as individual sub-tests.`,
				Parameters: eip8288PreFork,
				Files: map[string]string{
					"genesis.json": eip8288ChainDir + "/genesis.json",
				},
				AlwaysRun: true,
				Run:       runRLPxTest("eth75-prefork-test"),
			},
		},
	}

	eth76 := hivesim.Suite{
		Name:        "eth76",
		Description: "This suite tests eth/76 relay and aggregation of EIP-8288 proof wrappers.",
		Tests: []hivesim.AnyTest{
			hivesim.ClientTestSpec{
				Role: "eth1",
				Name: "client launch",
				Description: `This test launches the client on a chain with EIP-8288 active, aggregating every
transaction-hash bucket, and runs the test tool. Results from the test tool are reported
as individual sub-tests.`,
				Parameters: eip8288env.Set("HIVE_EIP8288_AGGREGATION_PROBABILITY", "1"),
				Files: map[string]string{
					"genesis.json": eip8288ChainDir + "/genesis.json",
					"chain.rlp":    eip8288ChainDir + "/chain.rlp",
				},
				AlwaysRun: true,
				Run:       runRLPxTest("eth76-test"),
			},
			hivesim.ClientTestSpec{
				Role: "eth1",
				Name: "client launch without aggregation",
				Description: `This test launches the client on a chain with EIP-8288 active, aggregating no
transaction-hash bucket, and checks that it still admits and relays received aggregates.`,
				Parameters: eip8288NoAggregation,
				Files: map[string]string{
					"genesis.json": eip8288ChainDir + "/genesis.json",
					"chain.rlp":    eip8288ChainDir + "/chain.rlp",
				},
				AlwaysRun: true,
				Run:       runRLPxTest("eth76-test", "AggregateRelay"),
			},
		},
	}

	hivesim.MustRun(hivesim.New(), discv4, discv5, eth, snap, snap2, eth75, eth76)
}

// The EIP-8288 test chain: a genesis with EIP-8288 active and one block built on it. Its
// fork environment sets HIVE_EIP8288_TIMESTAMP, which the pre-fork test moves to 2100.
const (
	eip8288ChainDir         = testChainDir + "/eip8288"
	eip8288PreForkTimestamp = "4102444800"
)

func loadTestChainConfig() hivesim.Params {
	return loadChainConfig(testChainDir)
}

func loadChainConfig(dir string) hivesim.Params {
	content, err := os.ReadFile(dir + "/forkenv.json")
	if err != nil {
		panic(err)
	}
	var p hivesim.Params
	if err := json.Unmarshal(content, &p); err != nil {
		panic(err)
	}
	return p
}

func runEthTest(t *hivesim.T, c *hivesim.Client) {
	enode, err := c.EnodeURL()
	if err != nil {
		t.Fatal(err)
	}

	_, pattern := t.Sim.TestPattern()
	cmd := exec.Command("./devp2p", "rlpx", "eth-test",
		"--tap",
		"--run", pattern,
		"--node", enode,
		"--chain", testChainDir,
		"--engineapi", fmt.Sprintf("http://%s:8551", c.IP),
		"--jwtsecret", "0x7365637265747365637265747365637265747365637265747365637265747365",
	)
	if err := runTAP(t, c.Type, cmd); err != nil {
		t.Fatal(err)
	}
}

func runSnapTest(t *hivesim.T, c *hivesim.Client) {
	enode, err := c.EnodeURL()
	if err != nil {
		t.Fatal(err)
	}

	_, pattern := t.Sim.TestPattern()
	cmd := exec.Command("./devp2p", "rlpx", "snap-test",
		"--tap",
		"--run", pattern,
		"--node", enode,
		"--chain", testChainDir,
		"--engineapi", fmt.Sprintf("http://%s:8551", c.IP),
		"--jwtsecret", "0x7365637265747365637265747365637265747365637265747365637265747365",
	)
	if err := runTAP(t, c.Type, cmd); err != nil {
		t.Fatal(err)
	}
}

// runRLPxTest returns a test running the given rlpx subcommand of the test tool, limited
// to the tests matching only, if given.
func runRLPxTest(command string, only ...string) func(*hivesim.T, *hivesim.Client) {
	return func(t *hivesim.T, c *hivesim.Client) {
		enode, err := c.EnodeURL()
		if err != nil {
			t.Fatal(err)
		}
		_, pattern := t.Sim.TestPattern()
		if len(only) > 0 {
			pattern = only[0]
		}
		cmd := exec.Command("./devp2p", "rlpx", command,
			"--tap",
			"--run", pattern,
			"--node", enode,
			"--chain", testChainDir,
			"--engineapi", fmt.Sprintf("http://%s:8551", c.IP),
			"--jwtsecret", "0x7365637265747365637265747365637265747365637265747365637265747365",
		)
		if err := runTAP(t, c.Type, cmd); err != nil {
			t.Fatal(err)
		}
	}
}

func runSnap2Test(t *hivesim.T, c *hivesim.Client) {
	enode, err := c.EnodeURL()
	if err != nil {
		t.Fatal(err)
	}

	_, pattern := t.Sim.TestPattern()
	cmd := exec.Command("./devp2p", "rlpx", "snap2-test",
		"--tap",
		"--run", pattern,
		"--node", enode,
		"--chain", testChainDir,
		"--engineapi", fmt.Sprintf("http://%s:8551", c.IP),
		"--jwtsecret", "0x7365637265747365637265747365637265747365637265747365637265747365",
	)
	if err := runTAP(t, c.Type, cmd); err != nil {
		t.Fatal(err)
	}
}

const network = "network1"

var networkCreated = make(map[hivesim.SuiteID]bool)

// createNetwork ensures there is a separate network to be able to send the client traffic
// from two separate IP addrs.
func createTestNetwork(t *hivesim.T) (bridgeIP, net1IP string) {
	if !networkCreated[t.SuiteID] {
		if err := t.Sim.CreateNetwork(t.SuiteID, network); err != nil {
			t.Fatal("can't create network:", err)
		}
		if err := t.Sim.ConnectContainer(t.SuiteID, network, "simulation"); err != nil {
			t.Fatal("can't connect simulation to network1:", err)
		}
		networkCreated[t.SuiteID] = true
	}
	// Find our IPs on the bridge network and network1.
	var err error
	bridgeIP, err = t.Sim.ContainerNetworkIP(t.SuiteID, "bridge", "simulation")
	if err != nil {
		t.Fatal("can't get IP of simulation container:", err)
	}
	net1IP, err = t.Sim.ContainerNetworkIP(t.SuiteID, network, "simulation")
	if err != nil {
		t.Fatal("can't get IP of simulation container on network1:", err)
	}
	return bridgeIP, net1IP
}

func runDiscv5Test(t *hivesim.T, c *hivesim.Client, getENR func(*hivesim.Client) (string, error)) {
	bridgeIP, net1IP := createTestNetwork(t)

	// Connect client to the test network.
	if err := t.Sim.ConnectContainer(t.SuiteID, network, c.Container); err != nil {
		t.Fatal("can't connect client to network1:", err)
	}

	nodeURL, err := getENR(c)
	if err != nil {
		t.Fatal("can't get client enode URL:", err)
	}
	t.Log("ENR:", nodeURL)

	// Run the test tool.
	_, pattern := t.Sim.TestPattern()
	cmd := exec.Command("./devp2p", "discv5", "test", "--run", pattern, "--tap", "--listen1", bridgeIP, "--listen2", net1IP, nodeURL)
	if err := runTAP(t, c.Type, cmd); err != nil {
		t.Fatal(err)
	}
}

func runDiscv4Test(t *hivesim.T, c *hivesim.Client) {
	bridgeIP, net1IP := createTestNetwork(t)

	nodeURL, err := c.EnodeURL()
	if err != nil {
		t.Fatal("can't get client enode URL:", err)
	}
	// Connect client to the test network.
	if err := t.Sim.ConnectContainer(t.SuiteID, network, c.Container); err != nil {
		t.Fatal("can't connect client to network1:", err)
	}

	// Run the test tool.
	_, pattern := t.Sim.TestPattern()
	cmd := exec.Command("./devp2p", "discv4", "test", "--run", pattern, "--tap", "--remote", nodeURL, "--listen1", bridgeIP, "--listen2", net1IP)
	if err := runTAP(t, c.Type, cmd); err != nil {
		t.Fatal(err)
	}
}

func runTAP(t *hivesim.T, clientName string, cmd *exec.Cmd) error {
	// Set up output streams.
	cmd.Stderr = os.Stderr
	output, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("can't set up test command stdout pipe: %v", err)
	}
	defer output.Close()

	// Forward TAP output to the simulator log.
	outputTee := io.TeeReader(output, os.Stdout)

	// Run the test command.
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("can't start test command: %v", err)
	}
	if err := reportTAP(t, clientName, outputTee); err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		return err
	}
	return cmd.Wait()
}

func reportTAP(t *hivesim.T, clientName string, output io.Reader) error {
	// Parse the output.
	parser, err := tap.NewParser(output)
	if err != nil {
		return fmt.Errorf("error parsing TAP: %v", err)
	}
	for {
		test, err := parser.Next()
		if test == nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		// Forward result to hive.
		name := fmt.Sprintf("%s (%s)", test.Description, clientName)
		testID, err := t.Sim.StartTest(t.SuiteID, hivesim.TestStartInfo{Name: name})
		if err != nil {
			return fmt.Errorf("can't report sub-test result: %v", err)
		}
		result := hivesim.TestResult{Pass: test.Ok, Details: test.Diagnostic}
		t.Sim.EndTest(t.SuiteID, testID, result)
	}
	return nil
}

func getBeaconENR(c *hivesim.Client) (string, error) {
	url := fmt.Sprintf("http://%v:4000/eth/v1/node/identity", c.IP)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP request to get ENR failed with status code %d", resp.StatusCode)
	}
	var responseJSON struct {
		Data struct {
			ENR string `json:"enr"`
		} `json:"data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&responseJSON)
	if err != nil {
		return "", err
	}
	return responseJSON.Data.ENR, nil
}

func getPortalENR(c *hivesim.Client) (string, error) {
	var nodeInfoResult struct {
		ENR string `json:"enr"`
	}
	if err := c.RPC().Call(&nodeInfoResult, "discv5_nodeInfo"); err != nil {
		return "", err
	}
	if nodeInfoResult.ENR == "" {
		return "", fmt.Errorf("missing 'enr' in discv5_nodeInfo response")
	}
	return nodeInfoResult.ENR, nil
}
