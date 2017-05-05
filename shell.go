package main

import (
	"os"
	"os/signal"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

// mainInShell is the entry point into hive in the case where an outer docker
// container shell is requested. It assembles a new docker image from the entire
// project and just calls a hive instance within it.
//
// The end goal of this mechanism is preventing any leakage of junk (be that file
// system, docker images and/or containers, network traffic) into the host system.
func mainInShell(daemon *docker.Client, overrides []string, cacher *buildCacher) error {
	// Build the image for the outer shell container and the container itself
	log15.Info("creating outer shell container")

	image, err := buildShell(daemon, cacher)
	if err != nil {
		return err
	}
	// Create the shell container and make sure it's deleted afterwards
	shell, err := createShellContainer(daemon, image, overrides)
	if err != nil {
		log15.Error("failed to create shell container", "error", err)
		return err
	}
	log15.Debug("created shell container")
	defer func() {
		log15.Debug("deleting shell container")
		if err := daemon.RemoveContainer(docker.RemoveContainerOptions{ID: shell.ID, Force: true}); err != nil {
			log15.Error("failed to delete shell container", "error", err)
		}
	}()
	// Start up a hive instance within the shell
	log15.Info("starting outer shell container")

	waiter, err := runContainer(daemon, shell.ID, log15.Root(), "", true)
	if err != nil {
		log15.Error("failed to execute hive shell", "error", err)
		return err
	}
	// Register an interrupt handler to cleanly tear the shell down
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	go func() {
		<-interrupt
		log15.Error("shell interrupted, stopping")
		err := daemon.StopContainer(shell.ID, 0)
		if err != nil {
			log15.Error("failed to stop hive shell", "error", err)
		}
	}()
	// Wait for container termination and return
	waiter.Wait()
	return nil
}
