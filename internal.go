package main

import (
	"errors"
	"os"
	"os/signal"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

// makeGenesisDAG runs the ethash DAG generator to ensure that the genesis epochs
// DAG is created prior to it being needed by simulations.
func makeGenesisDAG(daemon *docker.Client, cacher *buildCacher) error {
	// Build the image for the DAG generator
	log15.Info("creating ethash container")

	image, err := buildEthash(daemon, cacher)
	if err != nil {
		return err
	}
	// Create the ethash container container and make sure it's deleted afterwards
	ethash, err := createEthashContainer(daemon, image)
	if err != nil {
		log15.Error("failed to create ethash container", "error", err)
		return err
	}
	log15.Debug("created ethash container")
	defer func() {
		log15.Debug("deleting ethash container")
		err := daemon.RemoveContainer(docker.RemoveContainerOptions{ID: ethash.ID, Force: true})
		if err != nil {
			log15.Error("failed to delete ethash container ", "error", err)
		}
	}()
	// Start generating the genesis ethash DAG
	log15.Info("generating genesis DAG")

	waiter, err := runContainer(daemon, ethash.ID, log15.Root(), "", true)
	if err != nil {
		log15.Error("failed to execute ethash", "error", err)
		return err
	}
	// Register an interrupt handler to cleanly tear ethash down
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	errc := make(chan error, 1)
	go func() {
		<-interrupt
		errc <- errors.New("interrupted")
		err := daemon.StopContainer(ethash.ID, 0)
		if err != nil {
			log15.Error("failed to stop ethash", "error", err)
		}
	}()
	// Wait for container termination and return
	waiter.Wait()

	select {
	case err := <-errc:
		return err
	default:
		return nil
	}
}
