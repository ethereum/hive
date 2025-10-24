package main

import (
	"fmt"
	"net"

	"github.com/ethereum/hive/hivesim"
	"github.com/marioevz/eth-clients/clients"
)

var _ clients.ManagedClient = &HiveManagedClient{}

type HiveOptionsGenerator func() ([]hivesim.StartOption, error)

type HiveManagedClient struct {
	T                    *hivesim.T
	OptionsGenerator     HiveOptionsGenerator
	HiveClientDefinition *hivesim.ClientDefinition

	hiveClient        *hivesim.Client
	extraStartOptions []hivesim.StartOption
}

func (h *HiveManagedClient) IsRunning() bool {
	return h.hiveClient != nil
}

func (h *HiveManagedClient) Start() error {
	h.T.Logf("Starting client %s", h.ClientType())
	var (
		opts = make([]hivesim.StartOption, 0)
		err  error
	)
	if h.OptionsGenerator != nil {
		opts, err = h.OptionsGenerator()
		if err != nil {
			return fmt.Errorf("unable to get start options: %v", err)
		}
	}

	if h.extraStartOptions != nil {
		opts = append(opts, h.extraStartOptions...)
	}

	h.hiveClient = h.T.StartClient(h.HiveClientDefinition.Name, opts...)
	if h.hiveClient == nil {
		return fmt.Errorf("unable to launch client")
	}
	h.T.Logf(
		"Started client %s, container %s",
		h.ClientType(),
		h.hiveClient.Container,
	)
	return nil
}

func (h *HiveManagedClient) AddStartOption(opts ...interface{}) {
	if h.extraStartOptions == nil {
		h.extraStartOptions = make([]hivesim.StartOption, 0)
	}
	for _, o := range opts {
		if o, ok := o.(hivesim.StartOption); ok {
			h.extraStartOptions = append(h.extraStartOptions, o)
		}
	}
}

func (h *HiveManagedClient) GetIP() net.IP {
	if h.hiveClient == nil {
		return net.IP{}
	}
	return h.hiveClient.IP
}

func (h *HiveManagedClient) Shutdown() error {
	if err := h.T.Sim.StopClient(h.T.SuiteID, h.T.TestID, h.hiveClient.Container); err != nil {
		return err
	}
	h.hiveClient = nil
	return nil
}

func (h *HiveManagedClient) GetEnodeURL() (string, error) {
	return h.hiveClient.EnodeURL()
}

func (h *HiveManagedClient) ClientType() string {
	return h.HiveClientDefinition.Name
}
