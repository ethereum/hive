package hivedocker

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/hive/internal/libhive"
	docker "github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

type ContainerBackend struct {
	client *docker.Client
	config *Config
	logger log15.Logger
}

func NewContainerBackend(c *docker.Client, cfg *Config) *ContainerBackend {
	b := &ContainerBackend{client: c, config: cfg, logger: cfg.Logger}
	if b.logger == nil {
		b.logger = log15.Root()
	}
	return b
}

// RunEnodeSh runs the enode.sh script in a container.
func (b *ContainerBackend) RunEnodeSh(containerID string) (string, error) {
	exec, err := b.client.CreateExec(docker.CreateExecOptions{
		AttachStdout: true,
		AttachStderr: false,
		Tty:          false,
		Cmd:          []string{"/enode.sh"},
		Container:    containerID, //id is the container id
	})
	if err != nil {
		return "", fmt.Errorf("can't create enode.sh exec in %s: %v", containerID, err)
	}
	outputBuf := new(bytes.Buffer)
	err = b.client.StartExec(exec.ID, docker.StartExecOptions{
		Detach:       false,
		OutputStream: outputBuf,
	})
	if err != nil {
		return "", fmt.Errorf("can't run enode.sh in %s: %v", containerID, err)
	}
	return outputBuf.String(), nil
}

// StartContainer starts a docker container.
func (b *ContainerBackend) StartContainer(ctx context.Context, imageName string, opt libhive.ContainerOptions) (*libhive.ContainerInfo, error) {
	container, err := b.createContainer(imageName, opt.Env, opt.Files)
	if err != nil {
		return nil, fmt.Errorf("can't create client container: %v", err)
	}
	info := &libhive.ContainerInfo{ID: container.ID[:8]}
	logger := b.logger.New("image", imageName, "container", info.ID)

	// Get the IP of container.
	logger.Debug("container created")

	// Set up log file.
	if opt.LogDir != "" {
		basename := opt.LogFilePrefix + container.ID + ".log"
		info.LogFile = filepath.Join(opt.LogDir, basename)
	}

	// Run the container.
	var startTime = time.Now()
	waiter, err := b.runContainer(logger, container.ID, info.LogFile)
	if err != nil {
		waiter.Close()
		logger.Error("container did not start", "err", err)
		if removeErr := b.client.RemoveContainer(docker.RemoveContainerOptions{ID: info.ID, Force: true}); removeErr != nil {
			logger.Error("can't remove container", "err", removeErr)
		}
		return info, fmt.Errorf("container did not start: %v", err)
	}

	// Get the IP. This can only be done after the container has started.
	inspect := docker.InspectContainerOptions{Context: ctx, ID: info.ID}
	container, err = b.client.InspectContainerWithOptions(inspect)
	if err != nil {
		waiter.Close()
		if removeErr := b.client.RemoveContainer(docker.RemoveContainerOptions{ID: info.ID, Force: true}); removeErr != nil {
			logger.Error("can't remove container", "err", removeErr)
		}
		return info, err
	}
	info.IP = container.NetworkSettings.IPAddress
	info.MAC = container.NetworkSettings.MacAddress

	var containerExit = make(chan struct{}, 1)
	go func() {
		// Ensure the goroutine started by runContainer exits, so that its resources (e.g.
		// the logfile it creates) can be garbage collected.
		// TODO: This is not great. If we get an interrupt, the log file might not be flushed correctly.
		err := waiter.Wait()
		waiter.Close()
		containerExit <- struct{}{}
		if err == nil {
			logger.Debug("container finished cleanly")
		} else {
			logger.Error("container finished with error", "err", err)
		}
	}()

	var hasStarted = make(chan struct{}, 1)
	if opt.CheckLive {
		// Port check requested.
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		addr := fmt.Sprintf("%s:%d", info.IP, 8545)
		go checkPort(ctx, logger, addr, hasStarted)
	} else {
		// No port check, just assume it has started by now.
		hasStarted <- struct{}{}
	}

	// Wait for events.
	var checkErr error
	select {
	case <-hasStarted:
		logger.Debug("container online", "time", time.Since(startTime))
	case <-containerExit:
		checkErr = errors.New("terminated unexpectedly")
	case <-ctx.Done():
		checkErr = errors.New("container did not start")
	}

	if checkErr != nil {
		logger.Debug("deleting container")
		err := b.client.RemoveContainer(docker.RemoveContainerOptions{ID: container.ID, Force: true})
		if err != nil {
			logger.Error("can't delete container")
		}
	}
	return info, checkErr
}

// checkPort waits for the given TCP address to accept a connection.
func checkPort(ctx context.Context, logger log15.Logger, addr string, notify chan<- struct{}) {
	var (
		lastMsg time.Time
		ticker  = time.NewTicker(100 * time.Millisecond)
	)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if time.Since(lastMsg) >= time.Second {
				logger.Debug("checking container online...")
				lastMsg = time.Now()
			}
			var dialer net.Dialer
			conn, err := dialer.DialContext(ctx, "tcp", addr)
			if err == nil {
				notify <- struct{}{}
				conn.Close()
				return
			}
		}
	}
}

// WaitContainer waits for a container to exit.
func (b *ContainerBackend) WaitContainer(ctx context.Context, containerID string) (int, error) {
	return b.client.WaitContainerWithContext(containerID, ctx)
}

// StopContainer stops the given container.
func (b *ContainerBackend) StopContainer(containerID string) error {
	return b.client.RemoveContainer(docker.RemoveContainerOptions{ID: containerID, Force: true})
}

// CreateNetwork creates a docker network.
func (b *ContainerBackend) CreateNetwork(name string) (string, error) {
	network, err := b.client.CreateNetwork(docker.CreateNetworkOptions{
		Name:           name,
		CheckDuplicate: true,
		Attachable:     true,
	})
	if err != nil {
		return "", err
	}
	return network.ID, nil
}

// NetworkNameToID finds the network ID of network by the given name.
func (b *ContainerBackend) NetworkNameToID(name string) (string, error) {
	networks, err := b.client.ListNetworks()
	if err != nil {
		return "", err
	}
	for _, net := range networks {
		if net.Name == name {
			return net.ID, nil
		}
	}
	return "", libhive.ErrNetworkNotFound
}

// RemoveNetwork deletes a docker network.
func (b *ContainerBackend) RemoveNetwork(id string) error {
	info, err := b.client.NetworkInfo(id)
	if err != nil {
		return err
	}
	for _, container := range info.Containers {
		err := b.DisconnectContainer(container.Name, id)
		if err != nil {
			return err
		}
	}
	return b.client.RemoveNetwork(id)
}

// ContainerIP finds the IP of a container in the given network.
func (b *ContainerBackend) ContainerIP(containerID, networkID string) (net.IP, error) {
	details, err := b.client.InspectContainerWithOptions(docker.InspectContainerOptions{
		ID: containerID,
	})
	if err != nil {
		return nil, err
	}
	// Range over all networks to which the container is connected and get network-specific IP.
	for _, network := range details.NetworkSettings.Networks {
		if network.NetworkID == networkID {
			return net.ParseIP(network.IPAddress), nil
		}
	}
	return nil, fmt.Errorf("network not found")
}

// ConnectContainer connects the given container to a network.
func (b *ContainerBackend) ConnectContainer(containerID, networkID string) error {
	return b.client.ConnectNetwork(networkID, docker.NetworkConnectionOptions{
		Container: containerID,
	})
}

// DisconnectContainer disconnects the given container from a network.
func (b *ContainerBackend) DisconnectContainer(containerID, networkID string) error {
	return b.client.DisconnectNetwork(networkID, docker.NetworkConnectionOptions{
		Container: containerID,
	})
}

// createContainer creates a docker container from an image.
func (b *ContainerBackend) createContainer(imageName string, env map[string]string, files map[string]*multipart.FileHeader) (*docker.Container, error) {
	vars := []string{}
	for key, val := range env {
		vars = append(vars, key+"="+val)
	}
	// Create the container with env.
	c, err := b.client.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: imageName,
			Env:   vars,
		},
	})
	if err != nil {
		return nil, err
	}

	// Now upload files.
	err = b.uploadFiles(c.ID, files)
	return c, err
}

// uploadFiles uploads the given files into a docker container.
func (b *ContainerBackend) uploadFiles(id string, files map[string]*multipart.FileHeader) error {
	// Short circuit if there are no files to upload
	if len(files) == 0 {
		return nil
	}
	// Create a tarball archive with all the data files
	tarball := new(bytes.Buffer)
	tw := tar.NewWriter(tarball)
	for fileName, fileHeader := range files {
		// Fetch the next file to inject into the container
		file, err := fileHeader.Open()
		if err != nil {
			return err
		}
		defer file.Close()

		data, err := ioutil.ReadAll(file)
		if err != nil {
			return err
		}
		// Insert the file into the tarball archive
		header := &tar.Header{
			Name: fileName, //filepath.Base(fileHeader.Filename),
			Mode: int64(0777),
			Size: int64(len(data)),
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if _, err := tw.Write(data); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}

	// Upload the tarball into the destination container
	return b.client.UploadToContainer(id, docker.UploadToContainerOptions{
		InputStream: tarball,
		Path:        "/",
	})
}

// runContainer attaches to the output streams of an existing container, then
// starts executing the container and returns the CloseWaiter to allow the caller
// to wait for termination.
func (b *ContainerBackend) runContainer(logger log15.Logger, id, logfile string) (docker.CloseWaiter, error) {
	var stream io.Writer

	// Redirect container output to logfile.
	var fdsToClose []io.Closer
	if logfile != "" {
		if err := os.MkdirAll(filepath.Dir(logfile), 0755); err != nil {
			return nil, err
		}
		log, err := os.OpenFile(logfile, os.O_WRONLY|os.O_CREATE|os.O_SYNC|os.O_TRUNC, 0644)
		if err != nil {
			return nil, err
		}
		stream = log
		fdsToClose = append(fdsToClose, log)

		// If console logging was requested, tee the output and tag it with the container id.
		if b.config.ContainerOutput != nil {
			// Hook into the containers output stream and tee it out.
			hookedR, hookedW, err := os.Pipe()
			if err != nil {
				return nil, err
			}
			stream = io.MultiWriter(log, hookedW)
			fdsToClose = append(fdsToClose, hookedW)
			// Tag all log messages with the container ID.
			copy := func(dst io.Writer, src io.Reader) (int64, error) {
				scanner := bufio.NewScanner(src)
				for scanner.Scan() {
					dst.Write([]byte(fmt.Sprintf("[%s] %s\n", id[:8], scanner.Text())))
				}
				return 0, nil
			}
			go copy(b.config.ContainerOutput, hookedR)
		}
	}

	// Attach the output stream.
	logger.Debug("attaching to container")
	attach := docker.AttachToContainerOptions{Container: id}
	if stream != nil {
		attach.OutputStream = stream
		attach.ErrorStream = stream
		attach.Stream = true
		attach.Stdout = true
		attach.Stderr = true
	}
	waiter, err := b.client.AttachToContainerNonBlocking(attach)
	if err != nil {
		logger.Error("failed to attach to container", "err", err)
		return nil, err
	}

	logger.Debug("starting container")
	hostConfig := &docker.HostConfig{}
	if err := b.client.StartContainer(id, hostConfig); err != nil {
		logger.Error("failed to start container", "err", err)
		return nil, err
	}
	return fdClosingWaiter{
		CloseWaiter: waiter,
		closers:     fdsToClose,
		logger:      logger,
	}, nil
}

// fdClosingWaiter wraps a docker.CloseWaiter and closes all io.Closer
// instances passed to it, after it is done waiting.
type fdClosingWaiter struct {
	docker.CloseWaiter
	closers []io.Closer
	logger  log15.Logger
}

func (w fdClosingWaiter) Wait() error {
	err := w.CloseWaiter.Wait()
	for _, closer := range w.closers {
		if err := closer.Close(); err != nil {
			w.logger.Error("failed to close fd", "err", err)
		}
	}
	return err
}
