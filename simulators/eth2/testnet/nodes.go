package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/testnet/setup"
	"github.com/protolambda/eth2api"
	"github.com/protolambda/eth2api/client/nodeapi"
)

const (
	PortUserRPC      = 8545
	PortEngineRPC    = 8551
	PortBeaconTCP    = 9000
	PortBeaconUDP    = 9000
	PortBeaconAPI    = 4000
	PortBeaconGRPC   = 4001
	PortMetrics      = 8080
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

func (en *Eth1Node) MustGetEnode() string {
	addr, err := en.EnodeURL()
	if err != nil {
		panic(err)
	}
	return addr
}

type BeaconNode struct {
	*hivesim.Client
	API *eth2api.Eth2HttpClient
}

func NewBeaconNode(cl *hivesim.Client) *BeaconNode {
	return &BeaconNode{
		Client: cl,
		API: &eth2api.Eth2HttpClient{
			Addr:  fmt.Sprintf("http://%s:%d", cl.IP, PortBeaconAPI),
			Cli:   &http.Client{},
			Codec: eth2api.JSONCodec{},
		},
	}
}

func (bn *BeaconNode) ENR() (string, error) {
	ctx, _ := context.WithTimeout(context.Background(), time.Second*10)
	var out eth2api.NetworkIdentity
	if err := nodeapi.Identity(ctx, bn.API, &out); err != nil {
		return "", err
	}
	fmt.Printf("p2p addrs: %v\n", out.P2PAddresses)
	fmt.Printf("peer id: %s\n", out.PeerID)
	return out.ENR, nil
}

func (bn *BeaconNode) EnodeURL() (string, error) {
	return "", errors.New("beacon node does not have an discv4 Enode URL, use ENR or multi-address instead")
}

type ValidatorClient struct {
	*hivesim.Client
	keys []*setup.KeyDetails
}

func (v *ValidatorClient) ContainsKey(pk [48]byte) bool {
	for _, k := range v.keys {
		if k.ValidatorPubkey == pk {
			return true
		}
	}
	return false
}
