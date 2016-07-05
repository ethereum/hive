package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

const (
	clientImagePrefix    = "hive/client/"
	validatorImagePrefix = "hive/validator/"
)

var (
	dockerEndpoint   = flag.String("docker-endpoint", "unix:///var/run/docker.sock", "Unix socket to th local Docker daemon")
	smokePattern     = flag.String("smoke", "", "Regexp selecting the client(s) to smoke-test")
	validatePattern  = flag.String("validate", "[^:]+:master", "Regexp selecting the client(s) to validate")
	validatorPattern = flag.String("validators", ".", "Regexp selecting the validation tests to run")
	overrideClient   = flag.String("override", "", "Client binary to override the default one with")
	noImageCache     = flag.Bool("nocache", false, "Disabled image caching, rebuilding all modified docker images")
	loglevelFlag     = flag.Int("loglevel", 3, "Log level to use for displaying system events")
)

func main() {
	// Parse the flags and configure the logger
	flag.Parse()
	log15.Root().SetHandler(log15.MultiHandler(
		log15.LvlFilterHandler(log15.Lvl(*loglevelFlag), log15.StdoutHandler),
		log15.LvlFilterHandler(log15.LvlDebug, log15.StreamHandler(os.Stderr, log15.LogfmtFormat())),
	))
	log, _ := os.OpenFile("log.txt", os.O_WRONLY|os.O_CREATE|os.O_SYNC|os.O_TRUNC, 0644)
	syscall.Dup2(int(log.Fd()), 2)

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

	switch {
	case *smokePattern != "":
		// If smoke testing is requested, run only part of the validators
		if err := validateClients(daemon, *smokePattern, "smoke/.", *overrideClient, true); err != nil {
			log15.Crit("failed to smoke-test client images", "error", err)
			return
		}

	case *validatePattern != "":
		// If validation is requested, run only the standalone tests
		if err := validateClients(daemon, *validatePattern, *validatorPattern, *overrideClient, *noImageCache); err != nil {
			log15.Crit("failed to validate client images", "error", err)
			return
		}
	}
}

// validateClients runs all the standalone validation tests against a number of
// clients matching the given pattern.
func validateClients(daemon *docker.Client, clientPattern, validatorPattern string, binary string, nocache bool) error {
	// Build all the clients matching the validation pattern
	log15.Info("building clients for validation", "pattern", clientPattern)
	clients, err := buildClients(daemon, clientPattern, nocache)
	if err != nil {
		return err
	}
	// Build all the validators known to the test harness
	log15.Info("building validators for testing", "pattern", validatorPattern)
	validators, err := buildValidators(daemon, validatorPattern, nocache)
	if err != nil {
		return err
	}
	// Iterate over all client and validator combos and cross-execute them
	results := make(map[string]map[string][]string)

	for client, clientImage := range clients {
		results[client] = make(map[string][]string)

		for validator, validatorImage := range validators {
			logger := log15.New("client", client, "validator", validator)
			start := time.Now()

			if pass, err := validate(daemon, clientImage, validatorImage, logger); pass {
				logger.Info("validation passed", "time", time.Since(start))
				results[client]["pass"] = append(results[client]["pass"], validator)
			} else {
				logger.Error("validation failed", "time", time.Since(start))
				fail := validator
				if err != nil {
					fail += ": " + err.Error()
				}
				results[client]["fail"] = append(results[client]["fail"], fail)
			}
		}
	}
	// Print the validation logs
	out, _ := json.MarshalIndent(results, "", "  ")
	fmt.Println(string(out))

	return nil
}

func validate(daemon *docker.Client, client, validator string, logger log15.Logger) (bool, error) {
	logger.Info("running client validation")

	// Create the client container and make sure it's cleaned up afterwards
	logger.Debug("creating client container")
	cc, err := daemon.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{Image: client},
	})
	if err != nil {
		logger.Error("failed to create client", "error", err)
		return false, err
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
		return false, err
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
		return false, err
	}
	logger.Debug("copying chain into client")
	if err = copyBetweenContainers(daemon, cc.ID, vc.ID, "/chain.rlp"); err != nil {
		logger.Error("failed to copy chain", "error", err)
		return false, err
	}
	logger.Debug("copying blocks into client")
	if err = copyBetweenContainers(daemon, cc.ID, vc.ID, "/blocks"); err != nil {
		logger.Error("failed to copy blocks", "error", err)
		return false, err
	}
	// Start the client and wait for it to finish
	clogger.Debug("running client container")
	cwaiter, err := runContainer(daemon, cc.ID, clogger)
	if err != nil {
		clogger.Error("failed to run client", "error", err)
		return false, err
	}
	defer cwaiter.Close()

	// Wait for the HTTP/RPC socket to open or the container to fail
	start := time.Now()
	for {
		// If the container died, bail out
		c, err := daemon.InspectContainer(cc.ID)
		if err != nil {
			clogger.Error("failed to inspect client", "error", err)
			return false, err
		}
		if !c.State.Running {
			clogger.Error("client container terminated")
			return false, errors.New("terminated unexpectedly")
		}
		// Container seems to be alive, check whether the RPC is accepting connections
		if conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", c.NetworkSettings.IPAddress, 8545)); err == nil {
			clogger.Debug("client container online", "time", time.Since(start))
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
		return false, err
	}
	vwaiter.Wait()

	// Retrieve the exist status to report pass of fail
	v, err := daemon.InspectContainer(vc.ID)
	if err != nil {
		vlogger.Error("failed to inspect validator", "error", err)
		return false, err
	}
	return v.State.ExitCode == 0, nil
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

	go io.Copy(os.Stderr, outR)
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
