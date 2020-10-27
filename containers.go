// This file contains the utility methods for managing docker containers.

package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

// hiveEnvvarPrefix is the prefix of the environment variables names that should
// be moved from test images to client container to fine tune their setup.
const hiveEnvvarPrefix = "HIVE_"

// hiveLogsFolder is the directory in which to place runtime logs from each of
// the docker containers.

// createShellContainer creates a docker container from the hive shell's image.
func createShellContainer(image string, overrides []string) (*docker.Container, error) {
	// Configure any workspace requirements for the container
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for _, dir := range []string{"docker", "logs"} {
		if err := os.MkdirAll(filepath.Join(pwd, "workspace", dir), os.ModePerm); err != nil {
			return nil, err
		}
	}
	// Create the list of bind points to make host files available internally
	binds := make([]string, 0, len(overrides)+2)
	for _, override := range overrides {
		file := override
		if strings.Contains(override, ":") {
			file = override[strings.LastIndex(override, ":")+1:]
		}
		if path, err := filepath.Abs(file); err == nil {
			binds = append(binds, fmt.Sprintf("%s:%s:ro", path, path)) // Mount to the same place, read only
		}
	}
	binds = append(binds, []string{
		fmt.Sprintf("%s/workspace/docker:/var/lib/docker", pwd),                                   // Surface any docker-in-docker data caches
		fmt.Sprintf("%s/workspace/logs:/gopath/src/github.com/ethereum/hive/workspace/logs", pwd), // Surface all the log files from the shell
	}...)

	uid := os.Getuid()
	if uid == -1 {
		uid = 0
	}

	// Create and return the actual docker container
	return dockerClient.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: image,
			Env:   []string{fmt.Sprintf("UID=%d", uid)}, // Forward the user ID for the workspace permissions
			Cmd:   os.Args[1:],
		},
		HostConfig: &docker.HostConfig{
			Privileged: true, // Docker in docker requires privileged mode
			Binds:      binds,
		},
	})
}

// createClientContainer creates a docker container from a client image
//
// A batch of environment variables may be specified to override from originating
// from the tester image. This is useful in particular during simulations where
// the tester itself can fine tune parameters for individual nodes.
func createClientContainer(client string, overrideEnvs map[string]string, files map[string]*multipart.FileHeader) (*docker.Container, error) {
	// Inject any explicit envvar overrides
	vars := []string{}
	for key, val := range overrideEnvs {
		if strings.HasPrefix(key, hiveEnvvarPrefix) {
			vars = append(vars, key+"="+val)
		}
	}
	// Create the client container with envvars injected
	c, err := dockerClient.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: client,
			Env:   vars,
		},
	})
	if err != nil {
		return nil, err
	}

	//now upload files
	uploadToContainer(c.ID, files)

	return c, nil
}

// uploadToContainer injects a batch of files into the target container.
func uploadToContainer(id string, files map[string]*multipart.FileHeader) error {
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
	return dockerClient.UploadToContainer(id, docker.UploadToContainerOptions{
		InputStream: tarball,
		Path:        "/",
	})
}

// runContainer attaches to the output streams of an existing container, then
// starts executing the container and returns the CloseWaiter to allow the caller
// to wait for termination.
func runContainer(id string, logger log15.Logger, logfile string, shell bool, logLevel int) (docker.CloseWaiter, error) {
	// If we're the outer shell, log straight to stderr, nothing fancy
	stdout := io.Writer(os.Stdout)
	stream := io.Writer(os.Stderr)
	var fdsToClose []io.Closer
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
		fdsToClose = append(fdsToClose, log)

		// If console logging was requested, tee the output and tag it with the container id
		if logLevel > 3 {
			// Hook into the containers output stream and tee it out
			hookedR, hookedW, err := os.Pipe()
			if err != nil {
				return nil, err
			}
			stream = io.MultiWriter(log, hookedW)
			fdsToClose = append(fdsToClose, hookedW)

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
		// Only the shell gets to keep its standard output
		stdout = stream
	}
	logger.Debug("attaching to container")
	waiter, err := dockerClient.AttachToContainerNonBlocking(docker.AttachToContainerOptions{
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

	hostConfig := &docker.HostConfig{Privileged: true, CapAdd: []string{"SYS_PTRACE"}, SecurityOpt: []string{"seccomp=unconfined"}}
	if err := dockerClient.StartContainer(id, hostConfig); err != nil {
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
