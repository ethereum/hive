package libhive

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net"
	"net/http"
)

// ContainerBackend captures the docker interactions of the simulation API.
type ContainerBackend interface {
	// Build is a hook allowing ContainerBackend to build internal helper images.
	// This is called before anything else in the simulation run.
	Build(context.Context, Builder) error

	// This is for launching the simulation API server.
	ServeAPI(context.Context, http.Handler) (APIServer, error)

	// These methods work with containers.
	CreateContainer(ctx context.Context, image string, opt ContainerOptions) (string, error)
	StartContainer(ctx context.Context, containerID string, opt ContainerOptions) (*ContainerInfo, error)
	DeleteContainer(containerID string) error
	PauseContainer(containerID string) error
	UnpauseContainer(containerID string) error

	// RunProgram runs a command in the given container and returns its outputs and exit code.
	RunProgram(ctx context.Context, containerID string, cmdline []string) (*ExecInfo, error)

	// These methods configure docker networks.
	NetworkNameToID(name string) (string, error)
	CreateNetwork(name string) (string, error)
	RemoveNetwork(id string) error
	ContainerIP(containerID, networkID string) (net.IP, error)
	ConnectContainer(containerID, networkID string) error
	DisconnectContainer(containerID, networkID string) error
}

// APIServer is a handle for the HTTP API server.
type APIServer interface {
	Addr() net.Addr // returns the listening address of the HTTP server
	Close() error   // stops the server
}

// This error is returned by NetworkNameToID if a docker network is not present.
var ErrNetworkNotFound = fmt.Errorf("network not found")

// ContainerOptions contains the launch parameters for docker containers.
type ContainerOptions struct {
	Env   map[string]string
	Files map[string]*multipart.FileHeader

	// This requests checking for the given TCP port to be opened by the container.
	CheckLive uint16

	// Output: if LogFile is set, container stdin and stderr is redirected to the
	// given log file. If Output is set, stdout is redirected to the writer. These
	// options are mutually exclusive.
	LogFile string
	Output  io.WriteCloser

	// Input: if set, container stdin draws from the given reader.
	Input io.ReadCloser
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
	ReadClientMetadata(name string) (*ClientMetadata, error)
	BuildClientImage(ctx context.Context, name string) (string, error)
	BuildSimulatorImage(ctx context.Context, name string) (string, error)
	BuildImage(ctx context.Context, name string, fsys fs.FS) error

	// ReadFile returns the content of a file in the given image.
	ReadFile(ctx context.Context, image, path string) ([]byte, error)
}

// ClientMetadata is metadata to describe the client in more detail, configured with a YAML file in the client dir.
type ClientMetadata struct {
	Roles []string `yaml:"roles" json:"roles"`
}
