package libdocker

import (
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"

	"github.com/ethereum/hive/internal/libhive"
	docker "github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

// Config is the configuration of the docker backend.
type Config struct {
	Inventory libhive.Inventory

	Logger log15.Logger

	// When building containers, any client or simulator image build matching the pattern
	// will avoid the docker cache.
	NoCachePattern *regexp.Regexp

	// This forces pulling of base images when building clients and simulators.
	PullEnabled bool

	// These two are log destinations for output from docker.
	ContainerOutput io.Writer
	BuildOutput     io.Writer
}

func Connect(dockerEndpoint string, cfg *Config) (*Builder, *ContainerBackend, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = log15.Root()
	}

	client, err := docker.NewClient(dockerEndpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("can't connect to docker: %v", err)
	}
	env, err := client.Version()
	if err != nil {
		return nil, nil, fmt.Errorf("can't get docker version: %v", err)
	}
	logger.Debug("docker daemon online", "version", env.Get("Version"))
	builder := NewBuilder(client, cfg)
	backend := NewContainerBackend(client, cfg)
	return builder, backend, nil
}

// LookupBridgeIP attempts to locate the IPv4 address of the local docker0 bridge
// network adapter.
func LookupBridgeIP(logger log15.Logger) (net.IP, error) {
	// Find the local IPv4 address of the docker0 bridge adapter
	interfaes, err := net.Interfaces()
	if err != nil {
		logger.Error("failed to list network interfaces", "error", err)
		return nil, err
	}
	// Iterate over all the interfaces and find the docker0 bridge
	for _, iface := range interfaes {
		if iface.Name == "docker0" || strings.Contains(iface.Name, "vEthernet") {
			// Retrieve all the addresses assigned to the bridge adapter
			addrs, err := iface.Addrs()
			if err != nil {
				logger.Error("failed to list docker bridge addresses", "error", err)
				return nil, err
			}
			// Find a suitable IPv4 address and return it
			for _, addr := range addrs {
				ip, _, err := net.ParseCIDR(addr.String())
				if err != nil {
					logger.Error("failed to list parse address", "address", addr, "error", err)
					return nil, err
				}
				if ipv4 := ip.To4(); ipv4 != nil {
					return ipv4, nil
				}
			}
		}
	}
	// Crap, no IPv4 found, bounce
	return nil, errors.New("not found")
}
