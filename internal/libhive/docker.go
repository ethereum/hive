package libhive

import (
	"archive/tar"
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"

	. "github.com/ethereum/hive/internal/hive"
)

var (
	ErrNetworkNotFound = fmt.Errorf("network not found")

	defaultStartTimeout = time.Duration(60 * time.Second)
)

// hiveEnvvarPrefix is the prefix of the environment variables names that should
// be moved from test images to client container to fine tune their setup.
const hiveEnvvarPrefix = "HIVE_"

type dockerBackend struct {
	hiveEnv SimEnv
	client  *docker.Client
}

func NewDockerBackend(env SimEnv, c *docker.Client) Backend {
	return &dockerBackend{hiveEnv: env, client: c}
}

func (b *dockerBackend) RunEnodeSh(containerID string) (string, error) {
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

func (b *dockerBackend) StartClient(name string, env map[string]string, files map[string]*multipart.FileHeader, checklive bool) (*ClientInfo, error) {
	info := &ClientInfo{
		Name:        name,
		VersionInfo: b.hiveEnv.ClientVersions[name],
	}

	// Default the loglevel to the simulator log level setting.
	logLevel := b.hiveEnv.SimLogLevel
	logLevelString, in := env["HIVE_LOGLEVEL"]
	if !in {
		env["HIVE_LOGLEVEL"] = strconv.Itoa(logLevel)
	} else {
		if _, err := strconv.Atoi(logLevelString); err != nil {
			log15.Warn(fmt.Sprintf("simulator client HIVE_LOGLEVEL %q is not an integer", logLevelString))
		}
	}

	// Resolve the docker image name.
	imageName, in := b.hiveEnv.Images[name]
	if !in {
		return info, fmt.Errorf("unknown client name %q", name)
	}

	// create and start the requested client container
	container, err := b.createClient(imageName, env, files)
	if err != nil {
		return info, fmt.Errorf("can't create client container: %v", err)
	}

	// start a new client logger
	info.ID = container.ID[:8]
	logger := log15.New("image", imageName, "container", info.ID)
	logger.Debug("client container created")

	var logfile string
	if b.hiveEnv.LogDir != "" {
		safeName := strings.Replace(name, string(filepath.Separator), "_", -1)
		info.LogFile = filepath.Join(safeName, fmt.Sprintf("client-%s.log", info.ID))
		logfile = filepath.Join(b.hiveEnv.LogDir, info.LogFile)
	}

	// Update the test-suite with the info we have.
	info.WasInstantiated = true
	info.InstantiatedAt = time.Now()

	//run the new client
	waiter, err := b.runContainer(logger, container.ID, logfile)
	if err != nil {
		logger.Error("failed to start client", "error", err)
		// Clean up the underlying container too
		if removeErr := b.client.RemoveContainer(docker.RemoveContainerOptions{ID: info.ID, Force: true}); removeErr != nil {
			logger.Error("failed to remove container", "error", removeErr)
		}
		return info, fmt.Errorf("can't run client container: %v", err)
	}

	go func() {
		// Ensure the goroutine started by runContainer exits, so that its resources (e.g.
		// the logfile it creates) can be garbage collected.
		err := waiter.Wait()
		if err == nil {
			logger.Debug("client container finished cleanly")
		} else {
			logger.Error("client container finished with error", "error", err)
		}
	}()

	// Wait for the HTTP/RPC socket to open or the container to fail
	var (
		containerIP  = ""
		containerMAC = ""
		start        = time.Now()
		checkTime    = 100 * time.Millisecond
		lastmsg      time.Time
	)
	for {
		// Update the container state
		container, err = b.client.InspectContainer(container.ID)
		if err != nil {
			logger.Error("failed to inspect client", "error", err)
			return info, fmt.Errorf("can't get container state: %v", err)
		}
		if !container.State.Running {
			logger.Error("client container terminated", "state", container.State.String())
			return info, errors.New("terminated unexpectedly")
		}

		containerIP = container.NetworkSettings.IPAddress
		containerMAC = container.NetworkSettings.MacAddress
		if checklive {
			if time.Since(lastmsg) >= time.Second {
				logger.Debug("checking container online....", "checktime", checkTime, "state", container.State.String())
				lastmsg = time.Now()
			}
			// Container seems to be alive, check whether the RPC is accepting connections
			if conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", containerIP, 8545)); err == nil {
				logger.Debug("client container online", "time", time.Since(start))
				conn.Close()
				break
			}
		} else {
			break
		}

		time.Sleep(checkTime)

		timeout := b.hiveEnv.ClientStartTimeout
		if timeout == 0 {
			timeout = defaultStartTimeout
		}
		if time.Since(container.Created) > timeout {
			log15.Debug("deleting client container", "name", name, "id", info.ID)
			err = b.client.RemoveContainer(docker.RemoveContainerOptions{ID: container.ID, Force: true})
			if err != nil {
				logger.Error("failed to terminate client container due to unresponsive RPC")
			}
			return info, fmt.Errorf("client terminated due to unresponsive RPC")
		}
	}

	// Container online and responsive.
	info.IP = containerIP
	info.MAC = containerMAC
	return info, nil
}

// StopContainer stops the given container.
func (b *dockerBackend) StopContainer(containerID string) error {
	return b.client.RemoveContainer(docker.RemoveContainerOptions{ID: containerID, Force: true})
}

// CreateNetwork creates a docker network.
func (b *dockerBackend) CreateNetwork(name string) (string, error) {
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
func (b *dockerBackend) NetworkNameToID(name string) (string, error) {
	networks, err := b.client.ListNetworks()
	if err != nil {
		return "", err
	}
	for _, net := range networks {
		if net.Name == name {
			return net.ID, nil
		}
	}
	return "", ErrNetworkNotFound
}

// RemoveNetwork deletes a docker network.
func (b *dockerBackend) RemoveNetwork(id string) error {
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
func (b *dockerBackend) ContainerIP(containerID, networkID string) (net.IP, error) {
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
func (b *dockerBackend) ConnectContainer(containerID, networkID string) error {
	return b.client.ConnectNetwork(networkID, docker.NetworkConnectionOptions{
		Container: containerID,
	})
}

// DisconnectContainer disconnects the given container from a network.
func (b *dockerBackend) DisconnectContainer(containerID, networkID string) error {
	return b.client.DisconnectNetwork(networkID, docker.NetworkConnectionOptions{
		Container: containerID,
	})
}

// createClientContainer creates a docker container from a client image.
//
// A batch of environment variables may be specified to override from originating from the
// tester image. This is useful in particular during simulations where the tester itself
// can fine tune parameters for individual nodes.
func (b *dockerBackend) createClient(client string, env map[string]string, files map[string]*multipart.FileHeader) (*docker.Container, error) {
	// Inject any explicit envvar overrides
	vars := []string{}
	for key, val := range env {
		if strings.HasPrefix(key, hiveEnvvarPrefix) {
			vars = append(vars, key+"="+val)
		}
	}
	// Create the client container with envvars injected
	c, err := b.client.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: client,
			Env:   vars,
		},
	})
	if err != nil {
		return nil, err
	}

	// now upload files.
	err = b.uploadFiles(c.ID, files)
	return c, err
}

// uploadToContainer injects a batch of files into the target container.
func (b *dockerBackend) uploadFiles(id string, files map[string]*multipart.FileHeader) error {
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
func (b *dockerBackend) runContainer(logger log15.Logger, id, logfile string) (docker.CloseWaiter, error) {
	stdout := io.Writer(os.Stdout)
	stream := io.Writer(os.Stderr)

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
		stream = io.Writer(log)
		fdsToClose = append(fdsToClose, log)

		// If console logging was requested, tee the output and tag it with the container id.
		if b.hiveEnv.PrintContainerOutput {
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
			go copy(os.Stderr, hookedR)
		}
		stdout = stream
	}

	// Attach the output stream.
	logger.Debug("attaching to container")
	waiter, err := b.client.AttachToContainerNonBlocking(docker.AttachToContainerOptions{
		Container:    id,
		OutputStream: stdout,
		ErrorStream:  stream,
		Stream:       true,
		Stdout:       true,
		Stderr:       true,
	})
	if err != nil {
		logger.Error("failed to attach to container", "error", err)
		return nil, err
	}

	// Start the requested container and wait until it terminates
	logger.Debug("starting container")

	hostConfig := &docker.HostConfig{}
	if err := b.client.StartContainer(id, hostConfig); err != nil {
		logger.Error("failed to start container", "error", err)
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
			w.logger.Error("failed to close fd", "error", err)
		}
	}
	return err
}
