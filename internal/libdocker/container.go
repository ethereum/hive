package libdocker

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func (b *ContainerBackend) RunProgram(ctx context.Context, containerID string, cmd []string) (*libhive.ExecInfo, error) {
	exec, err := b.client.CreateExec(docker.CreateExecOptions{
		Context:      ctx,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		Cmd:          cmd,
		Container:    containerID,
	})
	if err != nil {
		return nil, fmt.Errorf("can't create exec %v: %v", cmd, err)
	}
	outputBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	err = b.client.StartExec(exec.ID, docker.StartExecOptions{
		Context:      ctx,
		Detach:       false,
		OutputStream: outputBuf,
		ErrorStream:  errBuf,
	})
	if err != nil {
		return nil, fmt.Errorf("can't run exec %v: %v", cmd, err)
	}
	insp, err := b.client.InspectExec(exec.ID)
	if err != nil {
		return nil, fmt.Errorf("can't check execution result of %v: %v", cmd, err)
	}

	return &libhive.ExecInfo{
		Stdout:   outputBuf.String(),
		Stderr:   errBuf.String(),
		ExitCode: insp.ExitCode,
	}, nil
}

// CreateContainer creates a docker container.
func (b *ContainerBackend) CreateContainer(ctx context.Context, imageName string, opt libhive.ContainerOptions) (string, error) {
	vars := []string{}
	for key, val := range opt.Env {
		vars = append(vars, key+"="+val)
	}

	var hostConfig *docker.HostConfig

	if strings.Contains(imageName, "erigon") {
		portBindings := map[docker.Port][]docker.PortBinding{
			"8545/tcp": {{HostIP: "", HostPort: "0"}},
		}

		hostConfig = &docker.HostConfig{
			PortBindings: portBindings,
		}
	}
	c, err := b.client.CreateContainer(docker.CreateContainerOptions{
		Context: ctx,
		Config: &docker.Config{
			Image: imageName,
			Env:   vars,
		},
		HostConfig: hostConfig,
	})
	if err != nil {
		return "", err
	}
	logger := b.logger.New("image", imageName, "container", c.ID[:8])

	// Now upload files.
	if err := b.uploadFiles(ctx, c.ID, opt.Files); err != nil {
		logger.Error("container file upload failed", "err", err)
		b.DeleteContainer(c.ID)
		return "", err
	}
	logger.Debug("container created")
	return c.ID, err
}

// StartContainer starts a docker container.
func (b *ContainerBackend) StartContainer(ctx context.Context, containerID string, opt libhive.ContainerOptions) (*libhive.ContainerInfo, error) {
	info := &libhive.ContainerInfo{ID: containerID[:8], LogFile: opt.LogFile}
	logger := b.logger.New("container", info.ID)

	// Run the container.
	var startTime = time.Now()
	waiter, err := b.runContainer(ctx, logger, containerID, info.LogFile)
	if err != nil {
		b.DeleteContainer(containerID)
		return nil, fmt.Errorf("container did not start: %v", err)
	}

	// This goroutine waits for the container to end and closes log
	// files when done.
	containerExit := make(chan struct{})
	go func() {
		defer close(containerExit)
		err := waiter.Wait()
		waiter.Close()
		logger.Debug("container exited", "err", err)
	}()
	// Set up the wait function.
	info.Wait = func() { <-containerExit }

	// Get the IP. This can only be done after the container has started.
	inspect := docker.InspectContainerOptions{Context: ctx, ID: containerID}
	container, err := b.client.InspectContainerWithOptions(inspect)
	if err != nil {
		waiter.Close()
		b.DeleteContainer(containerID)
		info.Wait()
		info.Wait = nil
		return info, err
	}
	info.IP = container.NetworkSettings.IPAddress
	info.MAC = container.NetworkSettings.MacAddress

	portToCheck := fmt.Sprintf("%d", opt.CheckLive)

	for port, bindings := range container.NetworkSettings.Ports {
		if strings.HasPrefix(string(port), portToCheck) && len(bindings) > 0 {
			fmt.Println("bindings", bindings)
			portToCheck = bindings[0].HostPort
		}
	}

	// Set up the port check if requested.
	hasStarted := make(chan struct{})
	if opt.CheckLive != 0 {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		addr := fmt.Sprintf("%s:%s", "localhost" /*info.IP*/, portToCheck)
		go checkPort(ctx, logger, addr, hasStarted)
	} else {
		close(hasStarted)
	}

	// Wait for events.
	var checkErr error
	select {
	case <-hasStarted:
		logger.Debug("container online", "time", time.Since(startTime))
	case <-containerExit:
		checkErr = errors.New("terminated unexpectedly")
	case <-ctx.Done():
		checkErr = errors.New("timed out waiting for container startup")
	}
	if checkErr != nil {
		b.DeleteContainer(containerID)
		info.Wait()
		info.Wait = nil
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
				fmt.Println("is online addr?", addr)
				lastMsg = time.Now()
			}
			var dialer net.Dialer
			conn, err := dialer.DialContext(ctx, "tcp", addr)
			if err == nil {
				conn.Close()
				close(notify)
				return
			}
		}
	}
}

// DeleteContainer removes the given container. If the container is running, it is stopped.
func (b *ContainerBackend) DeleteContainer(containerID string) error {
	b.logger.Debug("removing container", "container", containerID[:8])
	err := b.client.RemoveContainer(docker.RemoveContainerOptions{ID: containerID, Force: true})
	if err != nil {
		b.logger.Error("can't remove container", "container", containerID[:8], "err", err)
	}
	return err
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

// uploadFiles uploads the given files into a docker container.
func (b *ContainerBackend) uploadFiles(ctx context.Context, id string, files map[string]*multipart.FileHeader) error {
	if len(files) == 0 {
		return nil
	}

	// Stream tar archive with all files.
	var (
		pipeR, pipeW = io.Pipe()
		streamErrCh  = make(chan error, 1)
	)
	go func() (err error) {
		defer func() { streamErrCh <- err }()
		defer pipeW.Close()

		tw := tar.NewWriter(pipeW)
		for filePath, fileHeader := range files {
			// Write file header.
			header := &tar.Header{Name: filePath, Mode: 0777, Size: fileHeader.Size}
			if err := tw.WriteHeader(header); err != nil {
				return err
			}
			// Write the file data.
			file, err := fileHeader.Open()
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(tw, file)
			file.Close()
			if copyErr != nil {
				return copyErr
			}
		}
		return tw.Close()
	}()

	// Upload the tar stream into the destination container.
	err := b.client.UploadToContainer(id, docker.UploadToContainerOptions{
		Context:     ctx,
		InputStream: pipeR,
		Path:        "/",
	})

	// Wait for the stream goroutine.
	streamErr := <-streamErrCh
	if err == nil && streamErr != nil {
		return streamErr
	}

	return err
}

// runContainer attaches to the output streams of an existing container, then
// starts executing the container and returns the CloseWaiter to allow the caller
// to wait for termination.
func (b *ContainerBackend) runContainer(ctx context.Context, logger log15.Logger, id, logfile string) (docker.CloseWaiter, error) {
	var stream io.Writer

	// Redirect container output to logfile.
	closer := newFileCloser(logger)
	if logfile != "" {
		if err := os.MkdirAll(filepath.Dir(logfile), 0755); err != nil {
			return nil, err
		}
		log, err := os.OpenFile(logfile, os.O_WRONLY|os.O_CREATE|os.O_SYNC|os.O_TRUNC, 0644)
		if err != nil {
			return nil, err
		}
		closer.addFile(log)
		stream = log

		// If console logging was requested, tee the output and tag it with the container id.
		if b.config.ContainerOutput != nil {
			prefixer := newLinePrefixWriter(b.config.ContainerOutput, fmt.Sprintf("[%s] ", id[:8]))
			closer.addFile(prefixer)
			stream = io.MultiWriter(log, prefixer)
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
		closer.closeFiles()
		logger.Error("failed to attach to container", "err", err)
		return nil, err
	}
	closer.w = waiter

	logger.Debug("starting container")
	if err := b.client.StartContainerWithContext(id, nil, ctx); err != nil {
		closer.Close()
		logger.Error("failed to start container", "err", err)
		return nil, err
	}
	return closer, nil
}

// fileCloser wraps a docker.CloseWaiter and closes all io.Closer instances held in it,
// after it is done waiting.
type fileCloser struct {
	w         docker.CloseWaiter
	logger    log15.Logger
	closers   []io.Closer
	closeOnce sync.Once
}

func newFileCloser(logger log15.Logger) *fileCloser {
	return &fileCloser{logger: logger}
}

func (w *fileCloser) Wait() error {
	err := w.w.Wait()
	w.closeFiles()
	return err
}

func (w *fileCloser) Close() error {
	err := w.w.Close()
	w.closeFiles()
	return err
}

func (w *fileCloser) addFile(c io.Closer) {
	w.closers = append(w.closers, c)
}

func (w *fileCloser) closeFiles() {
	w.closeOnce.Do(func() {
		for _, closer := range w.closers {
			if err := closer.Close(); err != nil {
				w.logger.Error("failed to close fd", "err", err)
			}
		}
	})
}

// linePrefixWriter wraps a writer, prefixing written lines with a string.
type linePrefixWriter struct {
	w      io.Writer
	prefix string
	buf    []byte // holds current incomplete line
}

func newLinePrefixWriter(w io.Writer, prefix string) *linePrefixWriter {
	return &linePrefixWriter{
		w:      w,
		prefix: prefix,
		buf:    []byte(prefix),
	}
}

func (w *linePrefixWriter) Write(bytes []byte) (int, error) {
	var err error
	for _, b := range bytes {
		if b == '\n' {
			// flush current line
			w.buf = append(w.buf, '\n')
			_, err = w.w.Write(w.buf)
			// start new line in buffer
			w.buf = w.buf[:0]
			w.buf = append(w.buf, w.prefix...)
		} else {
			w.buf = append(w.buf, b)
		}
	}
	return len(bytes), err
}

// Close flushes the last line.
func (w *linePrefixWriter) Close() error {
	var err error
	if len(w.buf) > len(w.prefix) {
		w.buf = append(w.buf, '\n')
		_, err = w.w.Write(w.buf)
	}
	w.buf = nil
	return err
}
