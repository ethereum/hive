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
	"sync"
	"time"

	"github.com/ethereum/hive/hiveproxy"
	"github.com/ethereum/hive/internal/libhive"
	docker "github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

type ContainerBackend struct {
	client *docker.Client
	config *Config
	logger log15.Logger

	proxy *hiveproxy.Proxy
}

func NewContainerBackend(c *docker.Client, cfg *Config) *ContainerBackend {
	b := &ContainerBackend{client: c, config: cfg, logger: cfg.Logger}
	if b.logger == nil {
		b.logger = log15.Root()
	}
	return b
}

// RunProgram runs a /hive-bin script in a container.
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
	createOpts := docker.CreateContainerOptions{
		Context: ctx,
		Config: &docker.Config{
			Image: imageName,
			Env:   vars,
		},
	}

	if opt.Input != nil {
		// Pre-announce that stdin will be attached. The stdin attachment
		// will fail silently if this is not set.
		createOpts.Config.AttachStdin = true
		createOpts.Config.StdinOnce = true
		createOpts.Config.OpenStdin = true
	}
	if opt.Output != nil {
		// Pre-announce that stdout will be attached. Not sure if this does anything,
		// but it's probably best to give Docker the info as early as possible.
		createOpts.Config.AttachStdout = true
	}

	c, err := b.client.CreateContainer(createOpts)
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
	if opt.CheckLive != 0 && b.proxy == nil {
		panic("attempt to start container with CheckLive, but proxy is not running")
	}

	info := &libhive.ContainerInfo{ID: containerID[:8], LogFile: opt.LogFile}
	logger := b.logger.New("container", info.ID)

	// Run the container.
	var startTime = time.Now()
	waiter, err := b.runContainer(ctx, logger, containerID, opt)
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
		logger.Debug("container exited", "err", err)
		err = waiter.Close()
		logger.Debug("container files closed", "err", err)
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

	// Set up the port check if requested.
	hasStarted := make(chan struct{})
	if opt.CheckLive != 0 {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		addr := &net.TCPAddr{IP: net.ParseIP(info.IP), Port: int(opt.CheckLive)}
		go func() {
			err := b.proxy.CheckLive(ctx, addr)
			if err == nil {
				close(hasStarted)
			}
		}()
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

// DeleteContainer removes the given container. If the container is running, it is stopped.
func (b *ContainerBackend) DeleteContainer(containerID string) error {
	b.logger.Debug("removing container", "container", containerID[:8])
	err := b.client.RemoveContainer(docker.RemoveContainerOptions{ID: containerID, Force: true})
	if err != nil {
		b.logger.Error("can't remove container", "container", containerID[:8], "err", err)
	}
	return err
}

// PauseContainer pauses the given container.
func (b *ContainerBackend) PauseContainer(containerID string) error {
	b.logger.Debug("pausing container", "container", containerID[:8])
	err := b.client.PauseContainer(containerID)
	if err != nil {
		b.logger.Error("can't pause container", "container", containerID[:8], "err", err)
	}
	return err
}

// UnpauseContainer unpauses the given container.
func (b *ContainerBackend) UnpauseContainer(containerID string) error {
	b.logger.Debug("unpausing container", "container", containerID[:8])
	err := b.client.UnpauseContainer(containerID)
	if err != nil {
		b.logger.Error("can't unpause container", "container", containerID[:8], "err", err)
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
func (b *ContainerBackend) runContainer(ctx context.Context, logger log15.Logger, id string, opts libhive.ContainerOptions) (docker.CloseWaiter, error) {
	var (
		outStream io.Writer
		errStream io.Writer
		closer    = newFileCloser(logger)
	)

	switch {
	case opts.Output != nil && opts.LogFile != "":
		return nil, fmt.Errorf("can't use LogFile and Output options at the same time")

	case opts.Output != nil:
		outStream = opts.Output
		closer.addFile(opts.Output)

		// If console logging is requested, dump stderr there.
		if b.config.ContainerOutput != nil {
			prefixer := newLinePrefixWriter(b.config.ContainerOutput, fmt.Sprintf("[%s] ", id[:8]))
			closer.addFile(prefixer)
			errStream = prefixer

			// outStream = io.MultiWriter(outStream, prefixer)
		}

	case opts.LogFile != "":
		// Redirect container output to logfile.
		if err := os.MkdirAll(filepath.Dir(opts.LogFile), 0755); err != nil {
			return nil, err
		}
		log, err := os.OpenFile(opts.LogFile, os.O_WRONLY|os.O_CREATE|os.O_SYNC|os.O_TRUNC, 0644)
		if err != nil {
			return nil, err
		}
		closer.addFile(log)
		outStream = log

		// If console logging was requested, tee the output and tag it with the container id.
		if b.config.ContainerOutput != nil {
			prefixer := newLinePrefixWriter(b.config.ContainerOutput, fmt.Sprintf("[%s] ", id[:8]))
			closer.addFile(prefixer)
			outStream = io.MultiWriter(log, prefixer)
		}
		// In LogFile mode, stderr is redirected to stdout.
		errStream = outStream
	}

	// Configure the streams and attach.
	attach := docker.AttachToContainerOptions{Container: id}
	if outStream != nil {
		attach.Stream = true
		attach.Stdout = true
		attach.OutputStream = outStream
	}
	if errStream != nil {
		attach.ErrorStream = errStream
		attach.Stderr = true
	}
	if opts.Input != nil {
		attach.InputStream = opts.Input
		attach.Stdin = true
		closer.addFile(opts.Input)
	}

	logger.Debug("attaching to container", "stdin", attach.Stdin, "stdout", attach.Stdout, "stderr", attach.Stderr)
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
