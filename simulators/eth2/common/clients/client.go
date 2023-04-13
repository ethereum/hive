package clients

import (
	"fmt"
	"net"
	"strconv"

	"github.com/ethereum/hive/hivesim"
)

type Client interface {
	IsRunning() bool
	GetIP() net.IP
	ClientType() string
}

type EnodeClient interface {
	Client
	GetEnodeURL() (string, error)
}

type ManagedClient interface {
	Client
	AddStartOption(...interface{})
	Start() error
	Shutdown() error
}

var _ ManagedClient = &HiveManagedClient{}

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
	opts, err := h.OptionsGenerator()
	if err != nil {
		return fmt.Errorf("unable to get start options: %v", err)
	}

	if opts == nil {
		opts = make([]hivesim.StartOption, 0)
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

var _ Client = &ExternalClient{}

type ExternalClient struct {
	Type     string
	IP       net.IP
	Port     int
	EnodeURL string
}

func ExternalClientFromURL(url string, typ string) (*ExternalClient, error) {
	ip, portStr, err := net.SplitHostPort(url)
	if err != nil {
		return nil, err
	}
	port, err := strconv.ParseInt(portStr, 10, 64)
	if err != nil {
		return nil, err
	}
	return &ExternalClient{
		Type: typ,
		IP:   net.ParseIP(ip),
		Port: int(port),
	}, nil
}

func (m *ExternalClient) IsRunning() bool {
	// We can try pinging a certain port for status
	return true
}

func (m *ExternalClient) GetIP() net.IP {
	return m.IP
}

func (m *ExternalClient) GetPort() int {
	return m.Port
}

func (m *ExternalClient) ClientType() string {
	return m.Type
}

func (m *ExternalClient) GetEnodeURL() (string, error) {
	return m.EnodeURL, nil
}
