package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/ethereum/hive/hivesim"
	"github.com/protolambda/eth2api"
	"github.com/protolambda/eth2api/client/nodeapi"
	"net/http"
	"time"
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
	API *eth2api.Eth2HttpClient
}

func NewBeaconNode(cl *hivesim.Client) *BeaconNode {
	return &BeaconNode{
		Client:         cl,
		API: &eth2api.Eth2HttpClient{
			Addr:  fmt.Sprintf("http://%s:%d", cl.IP, PortBeaconAPI),
			Cli:   &http.Client{},
			Codec: eth2api.JSONCodec{},
		},
	}
}

func (bn *BeaconNode) ENR() (string, error) {
	ctx, _ := context.WithTimeout(context.Background(), time.Second * 10)
	var out eth2api.NetworkIdentity
	if err := nodeapi.Identity(ctx, bn.API, &out); err != nil {
		return "", err
	}
	return out.ENR, nil
}

func (bn *BeaconNode) EnodeURL() (string, error) {
	return "", errors.New("beacon node does not have an discv4 Enode URL, use ENR or multi-address instead")
}

type ValidatorClient struct {
	*hivesim.Client
}

