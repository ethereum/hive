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

// hiveLogsFolder is the directory in which to place runtime logs from each of
// the docker containers.
var hiveLogsFolder = filepath.Join("workspace", "logs")

// createShellContainer creates a docker container from the hive shell's image.
func createShellContainer(daemon *docker.Client, image string, overrides []string) (*docker.Container, error) {
	// Configure any workspace requirements for the container
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(pwd, "workspace", "docker"), os.ModePerm); err != nil {
		return nil, err
	}
	// Create the list of bind points to make host files available internally
	binds := make([]string, 0, len(overrides)+2)
	for _, override := range overrides {
		if path, err := filepath.Abs(override); err == nil {
			binds = append(binds, fmt.Sprintf("%s:%s:ro", path, path)) // Mount to the same place, read only
		}
	}
	binds = append(binds, []string{
		fmt.Sprintf("%s/.ethash:/root/.ethash", os.Getenv("HOME")),                                // Reuse any already existing ethash DAGs
		fmt.Sprintf("%s/workspace/docker:/var/lib/docker", pwd),                                   // Surface any docker-in-docker data caches
		fmt.Sprintf("%s/workspace/logs:/gopath/src/github.com/karalabe/hive/workspace/logs", pwd), // Surface all the log files from the shell
	}...)

	// Create and return the actual docker container
	return daemon.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: image,
			Env:   []string{fmt.Sprintf("UID=%d", os.Getuid())}, // Forward the user ID for the workspace permissions
			Cmd:   os.Args[1:],
		},
		HostConfig: &docker.HostConfig{
			Privileged: true, // Docker in docker requires privileged mode
			Binds:      binds,
		},
	})
}

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
func runContainer(daemon *docker.Client, id string, logger log15.Logger, logfile string, shell bool) (docker.CloseWaiter, error) {
	// If we're the outer shell, log straight to stderr, nothing fancy
	stream := io.Writer(os.Stderr)
	if !shell {
		// For non shell containers, create and open the log file for the output
		if err := os.MkdirAll(filepath.Dir(logfile), os.ModePerm); err != nil {
			return nil, err
		}
		log, err := os.OpenFile(logfile, os.O_WRONLY|os.O_CREATE|os.O_SYNC|os.O_TRUNC, os.ModePerm)
		if err != nil {
			return nil, err
		}
		stream = io.Writer(log)

		// If console logging was requested, tee the output and tag it with the container id
		if *loglevelFlag > 5 {
			// Hook into the containers output stream and tee it out
			hookedR, hookedW := io.Pipe()
			stream = io.MultiWriter(log, hookedW)

			// Tag all log messages with the container ID if not the outer shell
			copy := func(dst io.Writer, src io.Reader) (int64, error) {
				scanner := bufio.NewScanner(src)
				for scanner.Scan() {
					dst.Write([]byte(fmt.Sprintf("[%s] %s\n", id[:8], scanner.Text())))
				}
				return 0, nil
			}
			go copy(os.Stderr, hookedR)
		}
	}
	logger.Debug("attaching to container")
	waiter, err := daemon.AttachToContainerNonBlocking(docker.AttachToContainerOptions{
		Container:    id,
		OutputStream: stream,
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
	if err := daemon.StartContainer(id, nil); err != nil {
		logger.Error("failed to start container", "error", err)
		return nil, err
	}
	return waiter, nil
}
