package main

import (
	"github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

// mainInShell is the entry point into hive in the case where an outer docker
// container shell is requested. It assembles a new docker image from the entire
// project and just calls a hive instance within it.
//
// The end goal of this mechanism is preventing any leakage of junk (be that file
// system, docker images and/or containers, network traffic) into the host system.
func mainInShell(daemon *docker.Client, overrides []string) error {
	// Build the image for the outer shell container and the container itself
	log15.Info("creating outer shell container")

	image, err := buildShell(daemon)
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
		daemon.RemoveContainer(docker.RemoveContainerOptions{ID: shell.ID, Force: true})
	}()
	// Start up a hive instance within the shell
	log15.Info("starting outer shell container")

	waiter, err := runContainer(daemon, shell.ID, log15.Root(), true)
	if err != nil {
		log15.Error("failed to execute hive shell", "error", err)
		return err
	}
	waiter.Wait()

	return nil
}
