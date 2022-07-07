package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/hive/internal/libdocker"
	"github.com/ethereum/hive/internal/libhive"
	"gopkg.in/inconshreveable/log15.v2"
)

func main() {
	var (
		testResultsRoot       = flag.String("results-root", "workspace/logs", "Target `directory` for results files and logs.")
		loglevelFlag          = flag.Int("loglevel", 3, "Log `level` for system events. Supports values 0-5.")
		dockerEndpoint        = flag.String("docker.endpoint", "", "Endpoint of the local Docker daemon.")
		dockerNoCache         = flag.String("docker.nocache", "", "Regular `expression` selecting the docker images to forcibly rebuild.")
		dockerPull            = flag.Bool("docker.pull", false, "Refresh base images when building images.")
		dockerOutput          = flag.Bool("docker.output", false, "Relay all docker output to stderr.")
		simPattern            = flag.String("sim", "", "Regular `expression` selecting the simulators to run.")
		simTestPattern        = flag.String("sim.limit", "", "Regular `expression` selecting tests/suites (interpreted by simulators).")
		simParallelism        = flag.Int("sim.parallelism", 1, "Max `number` of parallel clients/containers (interpreted by simulators).")
		simTestLimit          = flag.Int("sim.testlimit", 0, "[DEPRECATED] Max `number` of tests to execute per client (interpreted by simulators).")
		simTimeLimit          = flag.Duration("sim.timelimit", 0, "Simulation `timeout`. Hive aborts the simulator if it exceeds this time.")
		simLogLevel           = flag.Int("sim.loglevel", 3, "Selects log `level` of client instances. Supports values 0-5.")
		simDevMode            = flag.Bool("dev", false, "Only starts the simulator API endpoint (listening at 127.0.0.1:3000 by default) without starting any simulators.")
		simDevModeAPIEndpoint = flag.String("dev.addr", "127.0.0.1:3000", "Endpoint that the simulator API listens on")

		clients = flag.String("client", "go-ethereum", "Comma separated `list` of clients to use. Client names in the list may be given as\n"+
			"just the client name, or a client_branch specifier. If a branch name is supplied,\n"+
			"the client image will use the given git branch or docker tag. Multiple instances of\n"+
			"a single client type may be requested with different branches.\n"+
			"Example: \"besu_latest,besu_20.10.2\"")
		clientTimeout = flag.Duration("client.checktimelimit", 3*time.Minute, "The `timeout` of waiting for clients to open up the RPC port.\n"+
			"If a very long chain is imported, this timeout may need to be quite large.\n"+
			"A lower value means that hive won't wait as long in case the node crashes and\n"+
			"never opens the RPC port.")
	)

	// Parse the flags and configure the logger.
	flag.Parse()
	log15.Root().SetHandler(log15.LvlFilterHandler(log15.Lvl(*loglevelFlag), log15.StreamHandler(os.Stderr, log15.TerminalFormat())))

	if *simTestLimit > 0 {
		log15.Warn("Option --sim.testlimit is deprecated and will have no effect.")
	}

	// Get the list of simulators.
	inv, err := libhive.LoadInventory(".")
	if err != nil {
		fatal(err)
	}
	simList, err := inv.MatchSimulators(*simPattern)
	if err != nil {
		fatal("bad --sim regular expression:", err)
	}
	if *simPattern != "" && len(simList) == 0 {
		fatal("no simulators for pattern", *simPattern)
	}

	// Create the docker backends.
	dockerConfig := &libdocker.Config{
		Inventory:   inv,
		PullEnabled: *dockerPull,
	}
	if *dockerNoCache != "" {
		re, err := regexp.Compile(*dockerNoCache)
		if err != nil {
			fatal("bad --docker-nocache regular expression:", err)
		}
		dockerConfig.NoCachePattern = re
	}
	if *dockerOutput {
		dockerConfig.ContainerOutput = os.Stderr
		dockerConfig.BuildOutput = os.Stderr
	}
	builder, containerBackend, err := libdocker.Connect(*dockerEndpoint, dockerConfig)
	if err != nil {
		fatal(err)
	}

	// Set up the context for CLI interrupts.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-sig
		cancel()
	}()

	// Run.
	runner := simRunner{
		inv:       inv,
		builder:   builder,
		container: containerBackend,
		env: libhive.SimEnv{
			LogDir:             *testResultsRoot,
			SimLogLevel:        *simLogLevel,
			SimTestPattern:     *simTestPattern,
			SimParallelism:     *simParallelism,
			ClientStartTimeout: *clientTimeout,
		},
		SimDurationLimit: *simTimeLimit,
	}
	clientList := splitAndTrim(*clients, ",")
	if err := runner.initClients(ctx, clientList); err != nil {
		fatal(err)
	}
	if err := libdocker.BuildProxy(ctx, builder); err != nil {
		fatal(err)
	}

	if *simDevMode {
		log15.Info("running in simulator development mode")
		runner.runSimulatorAPIDevMode(ctx, *simDevModeAPIEndpoint)
	} else if len(simList) > 0 {
		if err := runner.initSimulators(ctx, simList); err != nil {
			fatal(err)
		}
		if err := runner.runSimulations(ctx, simList); err != nil {
			fatal(err)
		}
	}
}

type simRunner struct {
	inv       libhive.Inventory
	container libhive.ContainerBackend
	builder   libhive.Builder
	env       libhive.SimEnv

	// This holds the image names of all built simulators.
	simImages map[string]string

	// This is the time limit for a single simulation run.
	SimDurationLimit time.Duration
}

// initClients builds client images.
func (r *simRunner) initClients(ctx context.Context, clientList []string) error {
	r.env.Definitions = make(map[string]*libhive.ClientDefinition)

	if len(clientList) == 0 {
		return errors.New("client list is empty, cannot simulate")
	}

	var anyBuilt bool
	log15.Info(fmt.Sprintf("building %d clients...", len(clientList)))
	for _, client := range clientList {
		if !r.inv.HasClient(client) {
			return fmt.Errorf("unknown client %q", client)
		}
		meta, err := r.builder.ReadClientMetadata(client)
		if err != nil {
			return err
		}
		image, err := r.builder.BuildClientImage(ctx, client)
		if err != nil {
			continue
		}
		anyBuilt = true
		version, err := r.builder.ReadFile(image, "/version.txt")
		if err != nil {
			log15.Warn("can't read version info of "+client, "image", image, "err", err)
		}
		r.env.Definitions[client] = &libhive.ClientDefinition{
			Name:    client,
			Version: strings.TrimSpace(string(version)),
			Image:   image,
			Meta:    *meta,
		}
	}
	if !anyBuilt {
		return errors.New("all clients failed to build")
	}
	return nil
}

// initSimulators builds simulator images.
func (r *simRunner) initSimulators(ctx context.Context, simList []string) error {
	r.simImages = make(map[string]string)

	log15.Info(fmt.Sprintf("building %d simulators...", len(simList)))
	for _, sim := range simList {
		image, err := r.builder.BuildSimulatorImage(ctx, sim)
		if err != nil {
			return err
		}
		r.simImages[sim] = image
	}
	return nil
}

func (r *simRunner) runSimulations(ctx context.Context, simList []string) error {
	log15.Info("creating output directory", "folder", r.env.LogDir)
	if err := os.MkdirAll(r.env.LogDir, 0755); err != nil {
		log15.Crit("failed to create logs folder", "err", err)
		return err
	}

	for _, sim := range simList {
		if err := r.run(ctx, sim); err != nil {
			return err
		}
	}
	return nil
}

func (r *simRunner) runSimulatorAPIDevMode(ctx context.Context, endpoint string) error {
	tm := libhive.NewTestManager(r.env, r.container, -1)
	defer func() {
		if err := tm.Terminate(); err != nil {
			log15.Error("could not terminate test manager", "error", err)
		}
	}()

	log15.Debug("starting simulator API proxy")
	proxy, err := r.container.ServeAPI(ctx, tm.API())
	if err != nil {
		log15.Error("can't start proxy", "err", err)
		return err
	}
	defer shutdownServer(proxy)

	log15.Debug("starting local API server")
	listener, err := net.Listen("tcp", endpoint)
	if err != nil {
		log15.Error("can't start TCP server", "err", err)
		return err
	}
	httpsrv := &http.Server{Handler: tm.API()}
	defer httpsrv.Close()
	go func() { httpsrv.Serve(listener) }()
	log15.Info(fmt.Sprintf("simulator API listening at %v", listener.Addr()))

	// wait for interrupt
	<-ctx.Done()
	return nil
}

// run runs one simulation.
func (r *simRunner) run(ctx context.Context, sim string) error {
	log15.Info(fmt.Sprintf("running simulation: %s", sim))

	// Start the simulation API.
	tm := libhive.NewTestManager(r.env, r.container, -1)
	defer func() {
		if err := tm.Terminate(); err != nil {
			log15.Error("could not terminate test manager", "error", err)
		}
	}()

	log15.Debug("starting simulator API server")
	server, err := r.container.ServeAPI(ctx, tm.API())
	if err != nil {
		log15.Error("can't start API server", "err", err)
		return err
	}
	defer shutdownServer(server)

	// Create the simulator container.
	opts := libhive.ContainerOptions{
		Env: map[string]string{
			"HIVE_SIMULATOR":    "http://" + server.Addr().String(),
			"HIVE_PARALLELISM":  strconv.Itoa(r.env.SimParallelism),
			"HIVE_LOGLEVEL":     strconv.Itoa(r.env.SimLogLevel),
			"HIVE_TEST_PATTERN": r.env.SimTestPattern,
		},
	}
	containerID, err := r.container.CreateContainer(ctx, r.simImages[sim], opts)
	if err != nil {
		return err
	}

	// Set the log file, and notify TestManager about the container.
	logbasename := fmt.Sprintf("%d-simulator-%s.log", time.Now().Unix(), containerID)
	opts.LogFile = filepath.Join(r.env.LogDir, logbasename)
	tm.SetSimContainerInfo(containerID, logbasename)

	log15.Debug("starting simulator container")
	sc, err := r.container.StartContainer(ctx, containerID, opts)
	if err != nil {
		return err
	}
	slogger := log15.New("sim", sim, "container", sc.ID[:8])
	slogger.Debug("started simulator container")
	defer func() {
		slogger.Debug("deleting simulator container")
		r.container.DeleteContainer(sc.ID)
	}()

	// Wait for simulator exit.
	done := make(chan struct{})
	go func() {
		sc.Wait()
		close(done)
	}()

	// if we have a simulation time limit, apply it.
	var timeout <-chan time.Time
	if r.SimDurationLimit != 0 {
		tt := time.NewTimer(r.SimDurationLimit)
		defer tt.Stop()
		timeout = tt.C
	}

	// Wait for simulation to end.
	select {
	case <-done:
	case <-timeout:
		slogger.Info("simulation timed out")
	case <-ctx.Done():
		slogger.Info("interrupted, shutting down")
		return errors.New("simulation interrupted")
	}
	return nil
}

// shutdownServer gracefully terminates the HTTP server.
func shutdownServer(server libhive.APIServer) {
	log15.Debug("terminating simulator API server")
	if err := server.Close(); err != nil {
		log15.Debug("API server shutdown failed", "err", err)
	}
}

func fatal(args ...interface{}) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(1)
}

func splitAndTrim(input, sep string) []string {
	list := strings.Split(input, sep)
	for i := range list {
		list[i] = strings.TrimSpace(list[i])
	}
	return list
}
