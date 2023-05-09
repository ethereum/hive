package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/ethereum/hive/hivesim"
	"github.com/shogo82148/go-tap"
)

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
				Run:       runDiscoveryTest,
			},
		},
	}

	eth := hivesim.Suite{
		Name:        "eth",
		Description: "This suite tests a client's ability to accurately respond to basic eth protocol messages.",
		Tests: []hivesim.AnyTest{
			hivesim.ClientTestSpec{
				Role: "eth1",
				Name: "client launch",
				Description: `This test launches the client and runs the test tool.
Results from the test tool are reported as individual sub-tests.`,
				Parameters: hivesim.Params{
					"HIVE_NETWORK_ID":     "19763",
					"HIVE_CHAIN_ID":       "19763",
					"HIVE_FORK_HOMESTEAD": "0",
					"HIVE_FORK_TANGERINE": "0",
					"HIVE_FORK_SPURIOUS":  "0",
					"HIVE_FORK_BYZANTIUM": "0",
					"HIVE_LOGLEVEL":       "4",
				},
				Files: map[string]string{
					"genesis.json": "./init/genesis.json",
					"chain.rlp":    "./init/halfchain.rlp",
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
				Parameters: hivesim.Params{
					"HIVE_NETWORK_ID":     "19763",
					"HIVE_CHAIN_ID":       "19763",
					"HIVE_FORK_HOMESTEAD": "0",
					"HIVE_FORK_TANGERINE": "0",
					"HIVE_FORK_SPURIOUS":  "0",
					"HIVE_FORK_BYZANTIUM": "0",
					"HIVE_LOGLEVEL":       "4",
				},
				Files: map[string]string{
					"genesis.json": "./init/genesis.json",
					"chain.rlp":    "./init/halfchain.rlp",
				},
				AlwaysRun: true,
				Run:       runSnapTest,
			},
		},
	}

	hivesim.MustRun(hivesim.New(), discv4, eth, snap)
}

func runEthTest(t *hivesim.T, c *hivesim.Client) {
	enode, err := c.EnodeURL()
	if err != nil {
		t.Fatal(err)
	}
	_, pattern := t.Sim.TestPattern()
	cmd := exec.Command("./devp2p", "rlpx", "eth-test", "--run", pattern, "--tap", enode, "./init/fullchain.rlp", "./init/genesis.json")
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
	cmd := exec.Command("./devp2p", "rlpx", "snap-test", "--run", pattern, "--tap", enode, "./init/fullchain.rlp", "./init/genesis.json")
	if err := runTAP(t, c.Type, cmd); err != nil {
		t.Fatal(err)
	}
}

var createNetworkOnce sync.Once

func runDiscoveryTest(t *hivesim.T, c *hivesim.Client) {
	nodeURL, err := c.EnodeURL()
	if err != nil {
		t.Fatal("can't get client enode URL:", err)
	}

	// Create a separate network to be able to send the client traffic from two separate IP addrs.
	const network = "network1"
	createNetworkOnce.Do(func() {
		if err := t.Sim.CreateNetwork(t.SuiteID, network); err != nil {
			t.Fatal("can't create network:", err)
		}
		if err := t.Sim.ConnectContainer(t.SuiteID, network, "simulation"); err != nil {
			t.Fatal("can't connect simulation to network1:", err)
		}
	})

	// Connect both simulation and client to this network.
	if err := t.Sim.ConnectContainer(t.SuiteID, network, c.Container); err != nil {
		t.Fatal("can't connect client to network1:", err)
	}
	// Find our IPs on the bridge network and network1.
	bridgeIP, err := t.Sim.ContainerNetworkIP(t.SuiteID, "bridge", "simulation")
	if err != nil {
		t.Fatal("can't get IP of simulation container:", err)
	}
	net1IP, err := t.Sim.ContainerNetworkIP(t.SuiteID, network, "simulation")
	if err != nil {
		t.Fatal("can't get IP of simulation container on network1:", err)
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
	// Forward results to hive.
	for {
		test, err := parser.Next()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		name := fmt.Sprintf("%s (%s)", test.Description, clientName)
		testID, err := t.Sim.StartTest(t.SuiteID, name, "")
		if err != nil {
			return fmt.Errorf("can't report sub-test result: %v", err)
		}
		result := hivesim.TestResult{Pass: test.Ok, Details: test.Diagnostic}
		t.Sim.EndTest(t.SuiteID, testID, result)
	}
}
