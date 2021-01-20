package libhive

import (
	"context"
	"fmt"
	"mime/multipart"
	"net"
)

// ContainerBackend captures the docker interactions of the simulation API.
type ContainerBackend interface {
	// These methods work with containers.
	CreateContainer(ctx context.Context, image string, opt ContainerOptions) (string, error)
	StartContainer(ctx context.Context, containerID string, opt ContainerOptions) (*ContainerInfo, error)
	DeleteContainer(containerID string) error

	// RunEnodeSh runs the /enode.sh script in the given container and returns its output.
	RunEnodeSh(ctx context.Context, containerID string) (string, error)

	// These methods configure docker networks.
	NetworkNameToID(name string) (string, error)
	CreateNetwork(name string) (string, error)
	RemoveNetwork(id string) error
	ContainerIP(containerID, networkID string) (net.IP, error)
	ConnectContainer(containerID, networkID string) error
	DisconnectContainer(containerID, networkID string) error
}

// This error is returned by NetworkNameToID if a docker network is not present.
var ErrNetworkNotFound = fmt.Errorf("network not found")

// ContainerOptions contains the launch parameters for docker containers.
type ContainerOptions struct {
	// These options apply when creating the container.
	Env   map[string]string
	Files map[string]*multipart.FileHeader

	// These options apply when starting the container.
	CheckLive bool   // requests check for TCP port 8545
	LogFile   string // if set, container output is written to this file
}

// ContainerInfo is returned by StartContainer.
type ContainerInfo struct {
	ID      string // docker container ID
	IP      string // IP address
	MAC     string // MAC address. TODO: remove
	LogFile string

	// The wait function returns when the container is stopped.
	// This must be called for all containers that were started
	// to avoid resource leaks.
	Wait func()
}

// Builder can build docker images of clients and simulators.
type Builder interface {
	BuildClientImage(ctx context.Context, name string) (string, error)
	BuildSimulatorImage(ctx context.Context, name string) (string, error)

	// ReadFile returns the content of a file in the given image.
	ReadFile(image, path string) ([]byte, error)
}
