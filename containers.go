// This file contains the utility methods for managing docker containers.

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"

	docker "github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

// runContainer attaches to the output streams of an existing container, then
// starts executing the container and returns the CloseWaiter to allow the caller
// to wait for termination.
func runContainer(id string, logger log15.Logger, logfile string, logLevel int) (docker.CloseWaiter, error) {
	stdout := io.Writer(os.Stdout)
	stream := io.Writer(os.Stderr)
	var fdsToClose []io.Closer

	// Create and open the log file for the output
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

		// Tag all log messages with the container ID
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
