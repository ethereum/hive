package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

// validateClients runs a batch of validation tests matched by validatorPattern
// against all clients matching clientPattern.
func validateClients(daemon *docker.Client, clientPattern, validatorPattern string, nocache bool) error {
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
	fmt.Printf("Validation results:\n%s\n", string(out))

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
		vlogger.Error("failed to run validator", "error", err)
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
