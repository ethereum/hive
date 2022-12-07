package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"regexp"
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
		errorOnFailingTests   = flag.Bool("exit.fail", true, "Exit with error code 1 if any test fails")

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
	if *simPattern != "" && *simDevMode {
		log15.Warn("--sim is ignored when using --dev mode")
		simList = nil
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
	builder, cb, err := libdocker.Connect(*dockerEndpoint, dockerConfig)
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
	env := libhive.SimEnv{
		LogDir:             *testResultsRoot,
		SimLogLevel:        *simLogLevel,
		SimTestPattern:     *simTestPattern,
		SimParallelism:     *simParallelism,
		SimDurationLimit:   *simTimeLimit,
		ClientStartTimeout: *clientTimeout,
	}
	runner := libhive.NewRunner(inv, builder, cb)
	clientList := splitAndTrim(*clients, ",")

	if err := runner.Build(ctx, clientList, simList); err != nil {
		fatal(err)
	}

	if *simDevMode {
		runner.RunDevMode(ctx, env, *simDevModeAPIEndpoint)
		return
	}

	var failCount int
	for _, sim := range simList {
		result, err := runner.Run(ctx, sim, env)
		if err != nil {
			fatal(err)
		}
		failCount += result.TestsFailed
		log15.Info(fmt.Sprintf("simulation %s finished", sim), "suites", result.Suites, "tests", result.Tests, "failed", result.TestsFailed)
	}

	if *errorOnFailingTests && failCount > 0 {
		switch failCount {
		case 0:
		case 1:
			fatal(errors.New("1 test failed"))
		default:
			fatal(fmt.Errorf("%d tests failed", failCount))
		}
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
