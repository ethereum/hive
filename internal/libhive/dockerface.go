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
	StartContainer(ctx context.Context, image string, opt ContainerOptions) (*ContainerInfo, error)
	WaitContainer(ctx context.Context, containerID string) (exitCode int, err error)
	StopContainer(containerID string) error

	// RunEnodeSh runs the /enode.sh script in the given container and returns its output.
	RunEnodeSh(containerID string) (string, error)

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
	LogDir        string // if set, put log file in this directory
	LogFilePrefix string // if set, the log file name will have this prefix
	CheckLive     bool   // if this is set, the backend waits for TCP port 8545 to open.
	Env           map[string]string
	Files         map[string]*multipart.FileHeader
}

// This is returned by StartContainer.
type ContainerInfo struct {
	ID      string // docker container ID
	IP      string // IP address
	MAC     string // MAC address. TODO: remove
	LogFile string
}

// Builder can build docker images of clients and simulators.
type Builder interface {
	BuildClientImage(ctx context.Context, name string) (string, error)
	BuildSimulatorImage(ctx context.Context, name string) (string, error)

	// ReadFile returns the content of a file in the given image.
	ReadFile(image, path string) ([]byte, error)
}
