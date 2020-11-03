package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ethereum/hive/internal/libhive"
	docker "github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

// runSimulations runs each 'simulation' container, which are hosts for executing one or more test-suites
func runSimulations(simulatorPattern string, overrides []string, cacher *buildCacher, errorReport *HiveErrorReport) error {
	// Build all the simulators known to the test harness
	log15.Info("building simulators for testing", "pattern", simulatorPattern)
	simulators, err := buildSimulators(simulatorPattern, cacher, errorReport)
	if err != nil {
		return err
	}

	for simulator, simulatorImage := range simulators {
		logger := log15.New("simulator", simulator)

		// TODO -  logdir:
		// A simulator can run multiple test suites so we don't want logfiles directly related to a test suite run in the UI.
		// Also the simulator can fail before a testsuite output is even produced.
		// What the simulator reports is not really interesting from a testing perspective. These messages are
		// about failures or warnings in even executing the testsuites.
		// Possible solution: list executions on a separate page, link from testsuites to executions where possible,
		// and have the execution details including duration, hive logs, and sim logs displayed.
		// logdir will be the execution folder. ie executiondir.
		logdir := *testResultsRoot
		err = simulate(*simLimiterFlag, simulatorImage, simulator, logger, logdir)
		if err != nil {
			return err
		}

	}
	return nil
}

// simulate runs a simulator container, which is a host for one or more testsuite
// runners. These communicate with this hive testsuite provider and host via
// a client API to run testsuites and their testcases.
func simulate(simDuration int, simulator string, simulatorLabel string, logger log15.Logger, logdir string) error {
	logger.Info(fmt.Sprintf("running client simulation: %s", simulatorLabel))

	// The simulator creates the testŕesult files, aswell as updates the index file. However, it needs to also
	// be aware of the location of it's own logfile, which should also be placed into the index.
	// We generate a unique name here, and pass it to the simulator via ENV vars
	var logName string
	{
		b := make([]byte, 16)
		rand.Read(b)
		logName = fmt.Sprintf("%d-%x-simulator.log", time.Now().Unix(), b)
	}

	// Create the test manager.
	env := libhive.SimEnv{
		Images:               allClients,
		LogDir:               logdir,
		SimLogLevel:          *simloglevelFlag,
		PrintContainerOutput: *simloglevelFlag > 3,
	}
	backend := libhive.NewDockerBackend(env, dockerClient)
	tm := libhive.NewTestManager(env, backend, *hiveMaxTestsFlag)
	defer func() {
		if err := tm.Terminate(); err != nil {
			log15.Error("could not terminate test manager", "error", err)
		}
	}()

	// Start the simulator HTTP API
	addr, server, err := startTestSuiteAPI(tm)
	if err != nil {
		log15.Error("failed to start simulator API", "error", err)
		return err
	}
	defer shutdownServer(server)

	// Start the simulator controller container
	logger.Debug("creating simulator container")
	hostConfig := &docker.HostConfig{Privileged: true, CapAdd: []string{"SYS_PTRACE"}, SecurityOpt: []string{"seccomp=unconfined"}}
	sc, err := dockerClient.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: simulator,
			Env: []string{
				fmt.Sprintf("HIVE_SIMULATOR=http://%v", addr),
				fmt.Sprintf("HIVE_DEBUG=%v", strconv.FormatBool(*hiveDebug)),
				fmt.Sprintf("HIVE_PARALLELISM=%d", *simulatorParallelism),
				fmt.Sprintf("HIVE_SIMLIMIT=%d", *simulatorTestLimit),
				fmt.Sprintf("HIVE_SIMLOG=%v", logName),
			},
		},
		HostConfig: hostConfig,
	})
	tm.SetSimContainerID(sc.ID)
	slogger := logger.New("id", sc.ID[:8])
	slogger.Debug("created simulator container")
	defer func() {
		slogger.Debug("deleting simulator container")
		if err := dockerClient.RemoveContainer(docker.RemoveContainerOptions{ID: sc.ID, Force: true}); err != nil {
			slogger.Error("failed to delete simulator container", "error", err)
		}
	}()

	// Start the tester container and wait until it finishes
	slogger.Debug("running simulator container")

	waiter, err := runContainer(sc.ID, slogger, filepath.Join(logdir, logName), false, *loglevelFlag)
	if err != nil {
		slogger.Error("failed to run simulator", "error", err)
		return err
	}

	// if we have a simulation time limiter, then timeout the simulation using the usual go pattern
	if simDuration != -1 {
		//make a timeout channel
		timeoutchan := make(chan error, 1)
		//wait for the simulator in a go routine, then push to the channel
		go func() {
			e := waiter.Wait()
			timeoutchan <- e
		}()
		//work out what the timeout is as a duration
		simTimeoutDuration := time.Duration(simDuration) * time.Second
		//wait for either the waiter or the timeout
		select {
		case err := <-timeoutchan:
			return err
		case <-time.After(simTimeoutDuration):
			err := waiter.Close()
			return err
		}
	} else {
		waiter.Wait()
	}

	return nil

}

// startTestSuiteAPI starts an HTTP webserver listening for simulator commands
// on the docker bridge and executing them until it is torn down.
func startTestSuiteAPI(tm *libhive.TestManager) (net.Addr, *http.Server, error) {
	// Find the IP address of the host container
	bridge, err := lookupBridgeIP(log15.Root())
	if err != nil {
		log15.Error("failed to lookup bridge IP", "error", err)
		return nil, nil, err
	}
	log15.Debug("docker bridge IP found", "ip", bridge)

	// Serve connections until the listener is terminated
	log15.Debug("starting simulator API server")

	// Start the API webserver for simulators to coordinate with
	addr, _ := net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:0", bridge))
	listener, err := net.ListenTCP("tcp4", addr)
	if err != nil {
		log15.Error("failed to listen on bridge adapter", "error", err)
		return nil, nil, err
	}
	laddr := listener.Addr()
	log15.Debug("listening for simulator commands", "addr", laddr)
	server := &http.Server{Handler: tm.API()}

	go server.Serve(listener)
	return laddr, server, nil
}

// shutdownServer gracefully terminates the HTTP server.
func shutdownServer(server *http.Server) {
	log15.Debug("terminating simulator server")

	//NB! Kill this first to make sure no further testsuites are started
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log15.Error("simulation API server shutdown failed", "error", err)
	}
}
