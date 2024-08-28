package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"context"
	"time"

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

	hivesim.MustRun(hivesim.New(), discv4, discv5, eth, snap)
}

func loadTestChainConfig() hivesim.Params {
	content, err := os.ReadFile(testChainDir + "/forkenv.json")
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

	// Run the test tool.
	_, pattern := t.Sim.TestPattern()
	cmd := exec.CommandContext(ctx, "./devp2p", "discv5", "test", "--run", pattern, "--tap", "--listen1", bridgeIP, "--listen2", net1IP, nodeURL)
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run the test tool.
	_, pattern := t.Sim.TestPattern()
	cmd := exec.CommandContext(ctx, "./devp2p", "discv4", "test", "--run", pattern, "--tap", "--remote", nodeURL, "--listen1", bridgeIP, "--listen2", net1IP)
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
