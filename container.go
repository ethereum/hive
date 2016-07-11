// This file contains the utility methods for managing docker containers.

package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
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
func createClientContainer(daemon *docker.Client, client string, tester string, overrideFiles []string, overrideEnvs map[string]string) (*docker.Container, error) {
	// Gather all the hive environment variables from the tester
	ti, err := daemon.InspectImage(tester)
	if err != nil {
		return nil, err
	}
	vars := []string{}
	for _, envvar := range ti.Config.Env {
		if strings.HasPrefix(envvar, hiveEnvvarPrefix) && overrideEnvs[envvar] == "" {
			vars = append(vars, envvar)
		}
	}
	// Inject any explicit envvar overrides
	for key, val := range overrideEnvs {
		if strings.HasPrefix(key, hiveEnvvarPrefix) {
			vars = append(vars, key+"="+val)
		}
	}
	// Create the client container with tester envvars injected
	c, err := daemon.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: client,
			Env:   vars,
		},
		HostConfig: &docker.HostConfig{
			Binds: []string{fmt.Sprintf("%s/.ethash:/root/.ethash", os.Getenv("HOME"))},
		},
	})
	if err != nil {
		return nil, err
	}
	// Inject any explicit file overrides into the container
	if err := uploadToContainer(daemon, c.ID, overrideFiles); err != nil {
		daemon.RemoveContainer(docker.RemoveContainerOptions{ID: c.ID, Force: true})
		return nil, err
	}
	return c, nil
}

// uploadToContainer injects a batch of files into the target container.
func uploadToContainer(daemon *docker.Client, id string, files []string) error {
	// Short circuit if there are no files to upload
	if len(files) == 0 {
		return nil
	}
	// Create a tarball archive with all the data files
	tarball := new(bytes.Buffer)
	tw := tar.NewWriter(tarball)

	for _, path := range files {
		// Fetch the next file to inject into the container
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		data, err := ioutil.ReadAll(file)
		if err != nil {
			return err
		}
		info, err := file.Stat()
		if err != nil {
			return err
		}
		// Insert the file into the tarball archive
		header := &tar.Header{
			Name: filepath.Base(file.Name()),
			Mode: int64(info.Mode()),
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
	return daemon.UploadToContainer(id, docker.UploadToContainerOptions{
		InputStream: tarball,
		Path:        "/",
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

	copy := func(dst io.Writer, src io.Reader) {
		scanner := bufio.NewScanner(src)
		for scanner.Scan() {
			dst.Write([]byte(fmt.Sprintf("[%s] %s\n", id[:8], scanner.Text())))
		}
	}
	go copy(os.Stderr, outR)
	go copy(os.Stderr, errR)

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
