package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/ethereum/hive/hivesim"
	"github.com/shogo82148/go-tap"
)

func main() {
	suite := hivesim.Suite{
		Name:        "eth protocol",
		Description: "This suite tests a client's ability to accurately respond to basic eth protocol messages.",
	}
	suite.Add(hivesim.ClientTestSpec{
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
			"HIVE_LOGLEVEL":       "5",
		},
		Files: map[string]string{
			"genesis.json": "./init/genesis.json",
			"chain.rlp":    "./init/halfchain.rlp",
		},
		Run: runEthTest,
	})
	hivesim.MustRunSuite(hivesim.New(), suite)
}

func runEthTest(t *hivesim.T, c *hivesim.Client) {
	enode, err := c.EnodeURL()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/devp2p", "rlpx", "eth-test", "--tap", enode, "./init/fullchain.rlp", "./init/genesis.json")
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
	suite, err := parser.Suite()
	if err != nil {
		return fmt.Errorf("error parsing TAP: %v", err)
	}
	// Forward results to hive.
	for _, test := range suite.Tests {
		name := fmt.Sprintf("%s (%s)", test.Description, clientName)
		testID, err := t.Sim.StartTest(t.SuiteID, name, "")
		if err != nil {
			return fmt.Errorf("can't report sub-test result: %v", err)
		}
		result := hivesim.TestResult{Pass: test.Ok, Details: test.Diagnostic}
		t.Sim.EndTest(t.SuiteID, testID, result)
	}
	return nil
}
