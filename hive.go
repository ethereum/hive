package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
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
		testResultsRoot = flag.String("results-root", "workspace/logs", "Target `directory` for results files and logs.")
		loglevelFlag    = flag.Int("loglevel", 3, "Log `level` to use for displaying system events.")
		dockerEndpoint  = flag.String("docker.endpoint", "unix:///var/run/docker.sock", "Endpoint of the local Docker daemon.")
		dockerNoCache   = flag.String("docker.nocache", "", "Regular `expression` selecting the docker images to forcibly rebuild.")
		dockerPull      = flag.Bool("docker.pull", false, "Refresh base images when building containers.")
		dockerOutput    = flag.Bool("docker.output", false, "Relay all docker output to stderr.")
		simPattern      = flag.String("sim", "", "Regular `expression` selecting the simulations to run.")
		simParallelism  = flag.Int("sim.parallelism", 1, "Max `number` of parallel clients/containers (interpreted by simulator).")
		simTestLimit    = flag.Int("sim.testlimit", 0, "Max `number` of tests to execute per client (interpreted by simulators).")
		simTimeLimit    = flag.Duration("sim.timelimit", 0, "Simulation `timeout`. Hive aborts the simulator if it exceeds this time.")
		simLogLevel     = flag.Int("sim.loglevel", 3, "The base log `level` for simulator client instances. Supports values 0-5.")

		clients = flag.String("client", "go-ethereum", "Comma separated `list` of clients to use. Client names in the list may be given as\n"+
			"just the client name, or a client_branch specifier. If a branch name is\n"+
			"supplied, a client image of the branch will be built. Multiple instances of a\n"+
			"single client type may be requested with different branches.\n"+
			"Example: --client besu_latest,besu_20.10.2")
		clientTimeout = flag.Duration("client.checktimelimit", 3*time.Minute, "The `timeout` of waiting for clients to open up the RPC port.\n"+
			"If a very long chain is imported, this timeout may need to be quite large. A lower value means that\n"+
			"hive won't wait as long in case the node crashes and never opens the RPC port.")
	)

	// Parse the flags and configure the logger.
	flag.Parse()
	log15.Root().SetHandler(log15.LvlFilterHandler(log15.Lvl(*loglevelFlag), log15.StreamHandler(os.Stderr, log15.TerminalFormat())))

	inv, err := libhive.LoadInventory(".")
	if err != nil {
		fatal(err)
	}

	// Get the list of simulations.
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
			SimParallelism:     *simParallelism,
			SimTestLimit:       *simTestLimit,
			ClientStartTimeout: *clientTimeout,
		},
		SimDurationLimit: *simTimeLimit,
	}
	clientList := splitAndTrim(*clients, ",")
	if err := runner.initClients(ctx, clientList); err != nil {
		fatal(err)
	}
	if len(simList) > 0 {
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
	r.env.Images = make(map[string]string)
	r.env.ClientVersions = make(map[string]string)

	if len(clientList) == 0 {
		return fmt.Errorf("client list is empty, cannot simulate")
	}
	log15.Info(fmt.Sprintf("building %d clients...", len(clientList)))
	for _, client := range clientList {
		if !r.inv.HasClient(client) {
			return fmt.Errorf("unknown client %q", client)
		}
		image, err := r.builder.BuildClientImage(ctx, client)
		if err != nil {
			return err
		}
		version, err := r.builder.ReadFile(image, "/version.json")
		if err != nil {
			log15.Warn("can't read version info of "+client, "image", image, "err", err)
		}
		r.env.Images[client] = image
		r.env.ClientVersions[client] = string(version)
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

// run runs one simulation.
func (r *simRunner) run(ctx context.Context, sim string) error {
	log15.Info(fmt.Sprintf("running simulation: %s", sim))

	tm := libhive.NewTestManager(r.env, r.container, -1)
	defer func() {
		if err := tm.Terminate(); err != nil {
			log15.Error("could not terminate test manager", "error", err)
		}
	}()

	// Start the simulation API.
	addr, server, err := startTestSuiteAPI(tm)
	if err != nil {
		log15.Error("failed to start simulator API", "error", err)
		return err
	}
	defer shutdownServer(server)

	// Start the simulator container.
	log15.Debug("starting simulator container")
	opts := libhive.ContainerOptions{
		LogDir:        r.env.LogDir,
		LogFilePrefix: fmt.Sprintf("%d-simulator-", time.Now().Unix()),
		Env: map[string]string{
			"HIVE_SIMULATOR":   "http://" + addr.String(),
			"HIVE_PARALLELISM": strconv.Itoa(r.env.SimParallelism),
			"HIVE_LOGLEVEL":    strconv.Itoa(r.env.SimLogLevel),
		},
	}
	if r.env.SimTestLimit != 0 {
		opts.Env["HIVE_SIMLIMIT"] = strconv.Itoa(r.env.SimTestLimit)
	}

	sc, err := r.container.StartContainer(ctx, r.simImages[sim], opts)
	if err != nil {
		return err
	}

	// TODO: There is a race here. The test manager needs to know about the
	// simulator container ID because the docker network APIs need it, but we
	// can't know the ID until the container is already running.
	tm.SetSimContainerID(sc.ID)

	slogger := log15.New("id", sc.ID[:8])
	slogger.Debug("started simulator container")
	defer func() {
		slogger.Debug("deleting simulator container")
		if err := r.container.StopContainer(sc.ID); err != nil {
			slogger.Error("can't delete simulator container", "err", err)
		}
	}()

	done := make(chan struct{})
	go func() {
		r.container.WaitContainer(ctx, sc.ID)
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
		return ctx.Err()
	}
	return nil
}

// startTestSuiteAPI starts an HTTP webserver listening for simulator commands
// on the docker bridge and executing them until it is torn down.
func startTestSuiteAPI(tm *libhive.TestManager) (net.Addr, *http.Server, error) {
	// Find the IP address of the host container
	bridge, err := libdocker.LookupBridgeIP(log15.Root())
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
		log15.Error("failed to listen on bridge adapter", "err", err)
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
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log15.Debug("simulation API server shutdown failed", "err", err)
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
