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

	"github.com/ethereum/hive/internal/hivedocker"
	"github.com/ethereum/hive/internal/libhive"
	"gopkg.in/inconshreveable/log15.v2"
)

func main() {
	var (
		dockerEndpoint = flag.String("docker-endpoint", "unix:///var/run/docker.sock", "Endpoint to the local Docker daemon")
		noCachePattern = flag.String("docker-nocache", "", "Regexp selecting the docker images to forcibly rebuild")
		pullEnabled    = flag.Bool("docker-pull", false, "Refresh base images when building containers")

		testResultsRoot = flag.String("results-root", "workspace/logs", "Target folder for results output and historical results aggregation")

		clientListFlag     = flag.String("client", "go-ethereum_latest", "Comma separated list of permitted clients for the test type, where client is formatted clientname_branch eg: go-ethereum_latest and the client name is a subfolder of the clients directory")
		checkTimeLimitFlag = flag.Duration("client.checktimelimit", 3*time.Minute, "The timeout to wait for a newly "+
			"instantiated client to open up the RPC port. If a very long chain is imported, this timeout may need to be quite large. "+
			"A lower value means that hive won't wait as long for in case node crashes and never opens the RPC port.")

		simulatorPattern     = flag.String("sim", "", "Regexp selecting the simulation tests to run")
		simulatorParallelism = flag.Int("sim.parallelism", 1, "Max number of parallel clients/containers to run tests against")
		simulatorTestLimit   = flag.Int("sim.testlimit", -1, "Max number of tests to execute per client (interpreted by simulators)")
		simLimiterFlag       = flag.Int("sim.timelimit", -1, "Run all simulators with a time limit in seconds, -1 being unlimited")
		simloglevelFlag      = flag.Int("sim.loglevel", 3, "The base log level for simulator client instances. "+
			"This number from 0-6 is interpreted differently depending on the client type.")

		loglevelFlag = flag.Int("loglevel", 3, "Log level to use for displaying system events")

		// Deprecated flags:
		_ = flag.Bool("debug", false, "A flag indicating debug mode, to allow docker containers to launch headless delve instances and so on")
		_ = flag.String("override", "", "Comma separated regexp:files to override in client containers")
		_ = flag.Bool("docker-noshell", false, "This flag has been deprecated and remains for script backwards compatibility. It will be removed in the future.")
		_ = flag.String("docker-hostalias", "unix:///var/run/docker.sock", "Endpoint to the host Docket daemon from within a validator")
		_ = flag.Bool("sim.rootcontext", false, "Indicates if the simulation should build "+
			"the dockerfile with root (simulator) or local context. Needed for access to sibling folders like simulators/common")
		_ = flag.Int("hivemaxtestcount", -1, "Limit the number of tests the simulator is permitted to generate in a testsuite for the Hive provider. "+
			"Used for smoke testing consensus tests themselves.")
	)

	// Parse the flags and configure the logger.
	flag.Parse()
	log15.Root().SetHandler(log15.LvlFilterHandler(log15.Lvl(*loglevelFlag), log15.StreamHandler(os.Stderr, log15.TerminalFormat())))

	inv, err := libhive.LoadInventory(".")
	if err != nil {
		fatal(err)
	}

	// Get the list of simulations.
	simList, err := inv.MatchSimulators(*simulatorPattern)
	if err != nil {
		fatal("bad --sim regular expression:", err)
	}

	// Create the docker backends.
	dockerConfig := &hivedocker.Config{
		Inventory:   inv,
		PullEnabled: *pullEnabled,
	}
	if *noCachePattern != "" {
		re, err := regexp.Compile(*noCachePattern)
		if err != nil {
			fatal("bad --docker-nocache regular expression:", err)
		}
		dockerConfig.NoCachePattern = re
	}
	if *loglevelFlag > 3 {
		dockerConfig.ContainerOutput = os.Stderr
		dockerConfig.BuildOutput = os.Stderr
	}
	builder, containerBackend, err := hivedocker.Connect(*dockerEndpoint, dockerConfig)
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
			SimLogLevel:        *simloglevelFlag,
			SimParallelism:     *simulatorParallelism,
			SimTestLimit:       *simulatorTestLimit,
			ClientStartTimeout: *checkTimeLimitFlag,
		},
		SimDurationLimit: time.Duration(*simLimiterFlag) * time.Second,
	}
	clientList := splitAndTrim(*clientListFlag, ",")
	if err := runner.initClients(ctx, clientList); err != nil {
		fatal(err)
	}
	if len(simList) > 0 {
		if err := runner.runSimulations(ctx, simList); err != nil {
			os.Exit(1)
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

	log15.Info("building simulators...")
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
	defer shutdownServer(ctx, server)

	// Start the simulator container.
	log15.Debug("starting simulator container")
	opts := libhive.ContainerOptions{
		LogDir:        r.env.LogDir,
		LogFilePrefix: fmt.Sprintf("%d-simulator-", time.Now().Unix()),
		Env: map[string]string{
			"HIVE_SIMULATOR":   "http://" + addr.String(),
			"HIVE_PARALLELISM": strconv.Itoa(r.env.SimParallelism),
			"HIVE_LOGLEVEL":    strconv.Itoa(r.env.SimLogLevel),
			"HIVE_SIMLIMIT":    strconv.Itoa(r.env.SimTestLimit),
		},
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
		return ctx.Err()
	}
	return nil
}

// startTestSuiteAPI starts an HTTP webserver listening for simulator commands
// on the docker bridge and executing them until it is torn down.
func startTestSuiteAPI(tm *libhive.TestManager) (net.Addr, *http.Server, error) {
	// Find the IP address of the host container
	bridge, err := hivedocker.LookupBridgeIP(log15.Root())
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
func shutdownServer(ctx context.Context, server *http.Server) {
	log15.Debug("terminating simulator server")

	//NB! Kill this first to make sure no further testsuites are started
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log15.Error("simulation API server shutdown failed", "err", err)
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
