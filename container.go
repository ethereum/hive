// This file contains the utility methods for managing docker containers.

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

// hiveEnvvarPrefix is the prefix of the environment variables names that should
// be moved from test images to client container to fine tune their setup.
const hiveEnvvarPrefix = "HIVE_"

// createClientContainer creates a docker container from a client image and moves
// any hive environment variables from the tester image into the new client.
//
// A batch of environment variables may be specified to override from originating
// from the tester image. This is useful in particular during simulations where
// the tester itself can fine tune parameters for individual nodes.
func createClientContainer(daemon *docker.Client, client string, tester string, override map[string][]string) (*docker.Container, error) {
	// Gather all the hive environment variables from the tester
	ti, err := daemon.InspectImage(tester)
	if err != nil {
		return nil, err
	}
	vars := []string{}
	for _, envvar := range ti.Config.Env {
		if strings.HasPrefix(envvar, hiveEnvvarPrefix) && override[envvar] == nil {
			vars = append(vars, envvar)
		}
	}
	// Inject any explicit overrides
	for key, vals := range override {
		vars = append(vars, key+"="+vals[0])
	}
	// Create the client container with tester envvars injected
	return daemon.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: client,
			Env:   vars,
		},
		HostConfig: &docker.HostConfig{
			Binds: []string{fmt.Sprintf("%s/.ethash:/root/.ethash", os.Getenv("HOME"))},
		},
	})
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
