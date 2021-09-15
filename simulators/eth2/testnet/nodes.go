package main

import (
	"errors"
	"fmt"
	"github.com/ethereum/hive/hivesim"
	"strings"
)

const (
	PortUserRPC = 8545
	PortEngineRPC = 8600
	PortBeaconTCP = 9000
	PortBeaconUDP = 9000
	PortBeaconAPI = 4000
	PortBeaconGRPC = 4001
	PortMetrics = 8080
	PortValidatorAPI = 5000
)

// TODO: we assume the clients were configured with default ports.
// Would be cleaner to run a script in the client to get the address without assumptions

type Eth1Node struct {
	*hivesim.Client
}

func (en *Eth1Node) UserRPCAddress() (string, error) {
	return fmt.Sprintf("http://%v:%d", en.IP, PortUserRPC), nil
}

func (en *Eth1Node) EngineRPCAddress() (string, error) {
	// TODO what will the default port be?
	return fmt.Sprintf("http://%v:%d", en.IP, PortEngineRPC), nil
}

type BeaconNode struct {
	*hivesim.Client
}

func (bn *BeaconNode) BeaconAPI() (string, error) {
	return fmt.Sprintf("http://%s:%d", bn.IP, PortBeaconAPI), nil
}

func (bn *BeaconNode) ENR() (string, error) {
	out, err := bn.Exec("enr.sh")
	if err != nil {
		return "", fmt.Errorf("failed exec: %v", err)
	}
	if out.ExitCode != 0 {
		return "", fmt.Errorf("script exit %d: %s", out.ExitCode, out.Stderr)
	}
	return strings.TrimSpace(out.Stdout), nil
}

func (bn *BeaconNode) EnodeURL() (string, error) {
	return "", errors.New("beacon node does not have an discv4 Enode URL, use ENR or multi-address instead")
}

type ValidatorClient struct {
	*hivesim.Client
}

