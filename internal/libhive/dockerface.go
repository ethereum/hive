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

	// RunProgram runs a command in the given container and returns its outputs and exit code.
	RunProgram(ctx context.Context, containerID string, opt ExecOptions) (*ExecInfo, error)

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

// ExecOptions contains the command and settings for executing a command in a running container
type ExecOptions struct {
	// Boolean value, runs the exec process with extended privileges.
	Privileged bool
	// A string value specifying the user, and optionally, group to run the exec process inside the container. Format is one of: "user", "user:group", "uid", or "uid:gid".
	User string
	// Command to run specified as a string or an array of strings.
	Cmd []string
}

// ExecInfo is returned by RunProgram
type ExecInfo struct {
	// The std-out output of the program execution
	StdOut string
	// The std-err output of the program execution
	StdErr string
	// The exit code of the execution
	ExitCode int
}

// ContainerOptions contains the launch parameters for docker containers.
type ContainerOptions struct {
	// These options apply when creating the container.
	Env map[string]string
	// Files can be plain files (default) or TAR archives for more customization.
	//
	// Using the MIME header "X-HIVE-FILETYPE", customize the meaning of the data of a multi-part file upload.
	// - "DEFAULT" (or omit) for a regular file with open to world (0777),
	//   and placed in the root of the system, with its HTTP multi-part filename.
	// - "TAR" for detailed customization of permissions, meta-data and easy preparation of many files.
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
