package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ethereum/hive/internal/libdocker"
	"github.com/ethereum/hive/internal/libhive"
	"gopkg.in/inconshreveable/log15.v2"
)

type buildArgs map[string]string

func (args *buildArgs) String() string {
	var kv []string
	for k, v := range *args {
		kv = append(kv, k+"="+v)
	}
	sort.Strings(kv)
	return strings.Join(kv, ",")
}

// Set implements flag.Value.
func (args *buildArgs) Set(value string) error {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return errors.New("invalid build argument format, expected ARG=VALUE")
	}
	(*args)[parts[0]] = parts[1]
	return nil
}

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
		simRandomSeed         = flag.Int("sim.randomseed", 0, "Randomness seed number (interpreted by simulators).")
		simTestLimit          = flag.Int("sim.testlimit", 0, "[DEPRECATED] Max `number` of tests to execute per client (interpreted by simulators).")
		simTimeLimit          = flag.Duration("sim.timelimit", 0, "Simulation `timeout`. Hive aborts the simulator if it exceeds this time.")
		simLogLevel           = flag.Int("sim.loglevel", 3, "Selects log `level` of client instances. Supports values 0-5.")
		simDevMode            = flag.Bool("dev", false, "Only starts the simulator API endpoint (listening at 127.0.0.1:3000 by default) without starting any simulators.")
		simDevModeAPIEndpoint = flag.String("dev.addr", "127.0.0.1:3000", "Endpoint that the simulator API listens on")
		useCredHelper         = flag.Bool("docker.cred-helper", false, "configure docker authentication using locally-configured credential helper")

		clientsFile = flag.String("client-file", "", `YAML `+"`file`"+` containing client configurations.`)

		clients = flag.String("client", "go-ethereum", "Comma separated `list` of clients to use. Client names in the list may be given as\n"+
			"just the client name, or a client_branch specifier. If a branch name is supplied,\n"+
			"the client image will use the given git branch or docker tag. Multiple instances of\n"+
			"a single client type may be requested with different branches.\n"+
			"Example: \"besu_latest,besu_20.10.2\"\n")

		clientTimeout = flag.Duration("client.checktimelimit", 3*time.Minute, "The `timeout` of waiting for clients to open up the RPC port.\n"+
			"If a very long chain is imported, this timeout may need to be quite large.\n"+
			"A lower value means that hive won't wait as long in case the node crashes and\n"+
			"never opens the RPC port.")
	)

	// Add the sim.buildarg flag multiple times to allow multiple build arguments.
	var simBuildArgs buildArgs
	flag.Var(&simBuildArgs, "sim.buildarg", "Argument to pass to the docker engine when building the simulator image, in the form of ARGNAME=VALUE.")

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
		Inventory:           inv,
		PullEnabled:         *dockerPull,
		UseCredentialHelper: *useCredHelper,
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
		SimRandomSeed:      *simRandomSeed,
		SimDurationLimit:   *simTimeLimit,
		ClientStartTimeout: *clientTimeout,
	}
	runner := libhive.NewRunner(inv, builder, cb)

	// Parse the client list.
	// It can be supplied as a comma-separated list, or as a YAML file.
	var clientList []libhive.ClientDesignator
	if *clientsFile == "" {
		clientList, err = libhive.ParseClientList(&inv, *clients)
		if err != nil {
			fatal("-client:", err)
		}
	} else {
		clientList, err = parseClientsFile(&inv, *clientsFile)
		if err != nil {
			fatal("-client-file:", err)
		}
		// If YAML file is used, the list can be filtered by the -client flag.
		if flagIsSet("client") {
			filter := strings.Split(*clients, ",")
			clientList = libhive.FilterClients(clientList, filter)
		}
	}

	// Build clients and simulators.
	if err := runner.Build(ctx, clientList, simList, simBuildArgs); err != nil {
		fatal(err)
	}
	if *simDevMode {
		runner.RunDevMode(ctx, env, *simDevModeAPIEndpoint)
		return
	}

	// Run simulators.
	var failCount int
	for _, sim := range simList {
		result, err := runner.Run(ctx, sim, env)
		if err != nil {
			fatal(err)
		}
		failCount += result.TestsFailed
		log15.Info(fmt.Sprintf("simulation %s finished", sim), "suites", result.Suites, "tests", result.Tests, "failed", result.TestsFailed)
	}

	switch failCount {
	case 0:
	case 1:
		fatal(errors.New("1 test failed"))
	default:
		fatal(fmt.Errorf("%d tests failed", failCount))
	}
}

func fatal(args ...interface{}) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(1)
}

func parseClientsFile(inv *libhive.Inventory, file string) ([]libhive.ClientDesignator, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return libhive.ParseClientListYAML(inv, f)
}

func flagIsSet(name string) bool {
	var found bool
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
