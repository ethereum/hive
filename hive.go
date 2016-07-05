package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

const (
	clientImagePrefix    = "hive/client/"
	validatorImagePrefix = "hive/validator/"
)

var (
	dockerEndpoint = flag.String("docker-endpoint", "unix:///var/run/docker.sock", "Unix socket to th local Docker daemon")
	validateClient = flag.String("validate", "[^:]+:develop", "Regexp selecting the client(s) to validate")
	overrideClient = flag.String("override", "", "Client binary to override the default one with")
	noImageCache   = flag.Bool("nocache", false, "Disabled image caching, rebuilding all modified docker images")
)

func main() {
	flag.Parse()

	// Connect to the local docker daemon and make sure it works
	daemon, err := docker.NewClient(*dockerEndpoint)
	if err != nil {
		log15.Crit("failed to connect to docker deamon", "error", err)
		return
	}
	env, err := daemon.Version()
	if err != nil {
		log15.Crit("failed to retrieve docker version", "error", err)
		return
	}
	log15.Info("docker daemon online", "version", env.Get("Version"))

	// If validation is requested, run only the standalone tests
	if *validateClient != "" {
		if err := validateClients(daemon, *validateClient, *overrideClient, *noImageCache); err != nil {
			log15.Crit("failed to build client images", "error", err)
			return
		}
	}
}

// validateClients runs all the standalone validation tests against a number of
// clients matching the given pattern.
func validateClients(daemon *docker.Client, pattern string, binary string, nocache bool) error {
	// Build all the clients matching the validation pattern
	log15.Info("building clients for validation", "pattern", pattern)
	clients, err := buildClients(daemon, pattern, nocache)
	if err != nil {
		return err
	}
	// Build all the validators known to the test harness
	log15.Info("building validators for testing")
	validators, err := buildValidators(daemon, nocache)
	if err != nil {
		return err
	}
	// Iterate over all client and validator combos and cross-execute them
	for client, clientImage := range clients {
		for validator, validatorImage := range validators {
			logger := log15.New("client", client, "validator", validator)
			validate(daemon, clientImage, validatorImage, logger)
		}
	}
	return nil
}

func validate(daemon *docker.Client, client, validator string, logger log15.Logger) error {
	logger.Info("running client validation")

	// Create the client container and make sure it's cleaned up afterwards
	logger.Debug("creating client container")
	cc, err := daemon.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{Image: client},
	})
	if err != nil {
		logger.Error("failed to create client", "error", err)
		return err
	}
	clogger := logger.New("id", cc.ID[:8])
	clogger.Debug("created client container")
	defer func() {
		clogger.Debug("deleting client container")
		daemon.RemoveContainer(docker.RemoveContainerOptions{ID: cc.ID, Force: true})
	}()

	// Create the validator container and make sure it's cleaned up afterwards
	logger.Debug("creating validator container")
	vc, err := daemon.CreateContainer(docker.CreateContainerOptions{
		Config:     &docker.Config{Image: validator},
		HostConfig: &docker.HostConfig{Links: []string{cc.ID + ":client"}},
	})
	if err != nil {
		logger.Error("failed to create validator", "error", err)
		return err
	}
	vlogger := logger.New("id", vc.ID[:8])
	vlogger.Debug("created validator container")
	defer func() {
		vlogger.Debug("deleting validator container")
		daemon.RemoveContainer(docker.RemoveContainerOptions{ID: vc.ID, Force: true})
	}()

	// Retrieve the genesis block and initial chain from the validator
	logger.Debug("copying genesis.json into client")
	if err = copyBetweenContainers(daemon, cc.ID, vc.ID, "/genesis.json"); err != nil {
		logger.Error("failed to copy genesis", "error", err)
		return err
	}
	logger.Debug("copying blocks into client")
	if err = copyBetweenContainers(daemon, cc.ID, vc.ID, "/blocks"); err != nil {
		logger.Error("failed to copy blocks", "error", err)
		return err
	}
	// Start the client and wait for it to finish
	clogger.Debug("running client container")
	cwaiter, err := runContainer(daemon, cc.ID, clogger)
	if err != nil {
		clogger.Error("failed to run client", "error", err)
	}
	defer cwaiter.Close()

	// Wait for the HTTP/RPC socket to open or the container to fail
	for {
		// If the container died, bail out
		c, err := daemon.InspectContainer(cc.ID)
		if err != nil {
			clogger.Error("failed to inspect client", "error", err)
		}
		if !c.State.Running {
			break
		}
		// Container seems to be alive, check whether the RPC is accepting connections
		if conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", c.NetworkSettings.IPAddress, 8545)); err == nil {
			clogger.Debug("client container online")
			conn.Close()
			break
		}
		time.Sleep(time.Second)
	}
	// Start the tester container and wait until it finishes
	vlogger.Debug("running validator container")
	vwaiter, err := runContainer(daemon, vc.ID, vlogger)
	if err != nil {
		vlogger.Error("failed to run client", "error", err)
	}
	vwaiter.Wait()

	return nil
}

// copyBetweenContainers copies a file from one docker container to another one.
func copyBetweenContainers(daemon *docker.Client, dest, src string, path string) error {
	// Download a tarball of the file from the source container
	tarball := new(bytes.Buffer)
	if err := daemon.DownloadFromContainer(src, docker.DownloadFromContainerOptions{
		Path:         path,
		OutputStream: tarball,
	}); err != nil {
		return err
	}
	// Upload the tarball into the destination container
	if err := daemon.UploadToContainer(dest, docker.UploadToContainerOptions{
		InputStream: tarball,
		Path:        "/",
	}); err != nil {
		return err
	}
	return nil
}

// runContainer attaches to the output streams of an existing container, then
// starts executing the container and returns the CloseWaiter to allow the caller
// to wait for termination.
func runContainer(daemon *docker.Client, id string, logger log15.Logger) (docker.CloseWaiter, error) {
	// Attach to the container and stream stderr and stdout
	outR, outW := io.Pipe()
	errR, errW := io.Pipe()

	go io.Copy(os.Stdout, outR)
	go io.Copy(os.Stderr, errR)

	logger.Debug("attaching to container")
	waiter, err := daemon.AttachToContainerNonBlocking(docker.AttachToContainerOptions{
		Container:    id,
		OutputStream: outW,
		ErrorStream:  errW,
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
	if err := daemon.StartContainer(id, nil); err != nil {
		logger.Error("failed to start container", "error", err)
		return nil, err
	}
	return waiter, nil
}
