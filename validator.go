package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

// validationResult represents the results of a validation run, containing
// various metadata.
type validationResult struct {
	Start   time.Time `json:"start"`           // Time instance when the validation ended
	End     time.Time `json:"end"`             // Time instance when the validation ended
	Success bool      `json:"success"`         // Whether the entire validation succeeded
	Error   error     `json:"error,omitempty"` // Potential hive failure during validation
}

// validateClients runs a batch of validation tests matched by validatorPattern
// against all clients matching clientPattern.
func validateClients(daemon *docker.Client, clientPattern, validatorPattern string, overrides []string) (map[string]map[string]*validationResult, error) {
	// Build all the clients matching the validation pattern
	log15.Info("building clients for validation", "pattern", clientPattern)
	clients, err := buildClients(daemon, clientPattern)
	if err != nil {
		return nil, err
	}
	// Build all the validators known to the test harness
	log15.Info("building validators for testing", "pattern", validatorPattern)
	validators, err := buildValidators(daemon, validatorPattern)
	if err != nil {
		return nil, err
	}
	// Iterate over all client and validator combos and cross-execute them
	results := make(map[string]map[string]*validationResult)

	for client, clientImage := range clients {
		results[client] = make(map[string]*validationResult)

		for validator, validatorImage := range validators {
			logger := log15.New("client", client, "validator", validator)

			logdir := filepath.Join(hiveLogsFolder, "validations", fmt.Sprintf("%s[%s]", strings.Replace(validator, "/", ":", -1), client))
			os.RemoveAll(logdir)

			result := validate(daemon, clientImage, validatorImage, overrides, logger, logdir)
			if result.Success {
				logger.Info("validation passed", "time", result.End.Sub(result.Start))
			} else {
				logger.Error("validation failed", "time", result.End.Sub(result.Start))
			}
			results[client][validator] = result
		}
	}
	return results, nil
}

func validate(daemon *docker.Client, client, validator string, overrides []string, logger log15.Logger, logdir string) *validationResult {
	logger.Info("running client validation")
	result := &validationResult{
		Start: time.Now(),
	}
	defer func() { result.End = time.Now() }()

	// Create the client container and make sure it's cleaned up afterwards
	logger.Debug("creating client container")
	cc, err := createClientContainer(daemon, client, validator, nil, overrides, nil)
	if err != nil {
		logger.Error("failed to create client", "error", err)
		result.Error = err
		return result
	}
	clogger := logger.New("id", cc.ID[:8])
	clogger.Debug("created client container")
	defer func() {
		clogger.Debug("deleting client container")
		daemon.RemoveContainer(docker.RemoveContainerOptions{ID: cc.ID, Force: true})
	}()

	// Start the client container and retrieve its IP address for the validator
	clogger.Debug("running client container")
	cwaiter, err := runContainer(daemon, cc.ID, clogger, filepath.Join(logdir, "client.log"), false)
	if err != nil {
		clogger.Error("failed to run client", "error", err)
		result.Error = err
		return result
	}
	defer cwaiter.Close()

	lcc, err := daemon.InspectContainer(cc.ID)
	if err != nil {
		clogger.Error("failed to retrieve client IP", "error", err)
		result.Error = err
		return result
	}
	cip := lcc.NetworkSettings.IPAddress

	// Wait for the HTTP/RPC socket to open or the container to fail
	start := time.Now()
	for {
		// If the container died, bail out
		c, err := daemon.InspectContainer(cc.ID)
		if err != nil {
			clogger.Error("failed to inspect client", "error", err)
			result.Error = err
			return result
		}
		if !c.State.Running {
			clogger.Error("client container terminated")
			result.Error = errors.New("terminated unexpectedly")
			return result
		}
		// Container seems to be alive, check whether the RPC is accepting connections
		if conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", c.NetworkSettings.IPAddress, 8545)); err == nil {
			clogger.Debug("client container online", "time", time.Since(start))
			conn.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	// Create the validator container and make sure it's cleaned up afterwards
	logger.Debug("creating validator container")
	vc, err := daemon.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: validator,
			Env:   []string{"HIVE_CLIENT_IP=" + cip},
		},
	})
	if err != nil {
		logger.Error("failed to create validator", "error", err)
		result.Error = err
		return result
	}
	vlogger := logger.New("id", vc.ID[:8])
	vlogger.Debug("created validator container")
	defer func() {
		vlogger.Debug("deleting validator container")
		daemon.RemoveContainer(docker.RemoveContainerOptions{ID: vc.ID, Force: true})
	}()

	// Start the tester container and wait until it finishes
	vlogger.Debug("running validator container")
	vwaiter, err := runContainer(daemon, vc.ID, vlogger, filepath.Join(logdir, "validator.log"), false)
	if err != nil {
		vlogger.Error("failed to run validator", "error", err)
		result.Error = err
		return result
	}
	vwaiter.Wait()

	// Retrieve the exist status to report pass of fail
	v, err := daemon.InspectContainer(vc.ID)
	if err != nil {
		vlogger.Error("failed to inspect validator", "error", err)
		result.Error = err
		return result
	}
	result.Success = v.State.ExitCode == 0
	return result
}
