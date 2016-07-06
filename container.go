// This file contains the utility methods for managing docker containers.

package main

import (
	"bytes"
	"io"
	"os"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

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
