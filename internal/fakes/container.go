package fakes

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/ethereum/hive/internal/libhive"
)

// BackendHooks can be used to override the behavior of the fake backend.
type BackendHooks struct {
	CreateContainer  func(image string, opt libhive.ContainerOptions) (string, error)
	StartContainer   func(image, containerID string, opt libhive.ContainerOptions) (*libhive.ContainerInfo, error)
	DeleteContainer  func(containerID string) error
	PauseContainer   func(containerID string) error
	UnpauseContainer func(containerID string) error
	RunProgram       func(containerID string, cmd []string) (*libhive.ExecInfo, error)

	NetworkNameToID     func(string) (string, error)
	CreateNetwork       func(string) (string, error)
	RemoveNetwork       func(networkID string) error
	ContainerIP         func(containerID, networkID string) (net.IP, error)
	ConnectContainer    func(containerID, networkID string) error
	DisconnectContainer func(containerID, networkID string) error
}

var _ = libhive.ContainerBackend(&fakeBackend{})

// fakeBackend implements Backend without docker.
type fakeBackend struct {
	hooks         BackendHooks
	clientCounter uint64
	netCounter    uint64

	mutex sync.Mutex
	cimg  map[string]string // tracks created containers and their image names
}

type apiServer struct {
	s    *http.Server
	addr net.Addr
}

func (s apiServer) Close() error {
	return s.s.Close()
}

func (s apiServer) Addr() net.Addr {
	return s.addr
}

// NewBackend creates a new fake container backend.
func NewContainerBackend(hooks *BackendHooks) libhive.ContainerBackend {
	b := &fakeBackend{cimg: make(map[string]string)}
	if hooks != nil {
		b.hooks = *hooks
	}
	return b
}

func (b *fakeBackend) Build(context.Context, libhive.Builder) error {
	return nil
}

func (b *fakeBackend) ServeAPI(ctx context.Context, h http.Handler) (libhive.APIServer, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	srv := &http.Server{Handler: h}
	go srv.Serve(l)
	return apiServer{srv, l.Addr()}, nil
}

func (b *fakeBackend) CreateContainer(ctx context.Context, image string, opt libhive.ContainerOptions) (string, error) {
	var id string
	var err error
	if b.hooks.CreateContainer != nil {
		id, err = b.hooks.CreateContainer(image, opt)
		if err != nil {
			return "", err
		}
	} else {
		id = fmt.Sprintf("%0.8x", atomic.AddUint64(&b.clientCounter, 1))
	}

	b.mutex.Lock()
	if _, ok := b.cimg[id]; ok {
		b.mutex.Unlock()
		return id, fmt.Errorf("duplicate container ID %q", id)
	}
	b.cimg[id] = image
	b.mutex.Unlock()
	return id, nil
}

func (b *fakeBackend) StartContainer(ctx context.Context, containerID string, opt libhive.ContainerOptions) (*libhive.ContainerInfo, error) {
	// Get image name.
	b.mutex.Lock()
	image, ok := b.cimg[containerID]
	if !ok {
		b.mutex.Unlock()
		return nil, fmt.Errorf("container %s does not exist", containerID)
	}
	b.mutex.Unlock()

	// Call the hook.
	var info libhive.ContainerInfo
	if b.hooks.StartContainer != nil {
		info2, err := b.hooks.StartContainer(image, containerID, opt)
		if err != nil {
			return nil, err
		}
		info = *info2
		info.ID = containerID
		if info.Wait == nil {
			info.Wait = func() {}
		}
	}

	info.ID = containerID
	if info.IP == "" {
		ip := net.IP{192, 0, 2, byte(b.clientCounter)}
		info.IP = ip.String()
	}
	if info.MAC == "" {
		info.MAC = "00:80:41:ae:fd:7e"
	}
	info.Wait = func() {}
	return &info, nil
}

func (b *fakeBackend) DeleteContainer(containerID string) error {
	var err error
	if b.hooks.DeleteContainer != nil {
		err = b.hooks.DeleteContainer(containerID)
	}

	b.mutex.Lock()
	delete(b.cimg, containerID)
	b.mutex.Unlock()
	return err
}

func (b *fakeBackend) PauseContainer(containerID string) error {
	if b.hooks.PauseContainer != nil {
		return b.hooks.PauseContainer(containerID)
	}
	return nil
}

func (b *fakeBackend) UnpauseContainer(containerID string) error {
	if b.hooks.UnpauseContainer != nil {
		return b.hooks.UnpauseContainer(containerID)
	}
	return nil
}

func (b *fakeBackend) RunProgram(ctx context.Context, containerID string, cmd []string) (*libhive.ExecInfo, error) {
	if b.hooks.RunProgram != nil {
		return b.hooks.RunProgram(containerID, cmd)
	}
	return &libhive.ExecInfo{Stdout: "std output", Stderr: "std err", ExitCode: 0}, nil
}

func (b *fakeBackend) NetworkNameToID(name string) (string, error) {
	if b.hooks.NetworkNameToID != nil {
		return b.hooks.NetworkNameToID(name)
	}
	return "", errors.New("network not found")
}

func (b *fakeBackend) CreateNetwork(name string) (string, error) {
	if b.hooks.CreateNetwork != nil {
		return b.hooks.CreateNetwork(name)
	}
	id := fmt.Sprintf("%0.8x", atomic.AddUint64(&b.netCounter, 1))
	return id, nil
}

func (b *fakeBackend) RemoveNetwork(networkID string) error {
	if b.hooks.RemoveNetwork != nil {
		return b.hooks.RemoveNetwork(networkID)
	}
	return nil
}

func (b *fakeBackend) ContainerIP(containerID, networkID string) (net.IP, error) {
	if b.hooks.ContainerIP != nil {
		return b.hooks.ContainerIP(containerID, networkID)
	}
	return net.IP{203, 0, 113, 2}, nil
}

func (b *fakeBackend) ConnectContainer(containerID, networkID string) error {
	if b.hooks.ConnectContainer != nil {
		return b.hooks.ConnectContainer(containerID, networkID)
	}
	return nil
}

func (b *fakeBackend) DisconnectContainer(containerID, networkID string) error {
	if b.hooks.DisconnectContainer != nil {
		return b.hooks.DisconnectContainer(containerID, networkID)
	}
	return nil
}
