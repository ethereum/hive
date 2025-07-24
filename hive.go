package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ethereum/hive/internal/libdocker"
	"github.com/ethereum/hive/internal/libhive"
	"github.com/lmittmann/tint"
	docker "github.com/fsouza/go-dockerclient"
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
		testResultsRoot = flag.String("results-root", "workspace/logs", "Target `directory` for results files and logs.")
		loglevelFlag    = flag.Int("loglevel", 3, "Log `level` for system events. Supports values 0-5.")
		dockerAuth      = flag.Bool("docker.auth", false, `Enable docker authentication from system config files. The following files are checked in the order listed:
If the environment variable DOCKER_CONFIG is set to a non-empty string:
- $DOCKER_CONFIG/plaintext-passwords.json
- $DOCKER_CONFIG/config.json
Otherwise, it looks for files in the $HOME directory:
- $HOME/.docker/plaintext-passwords.json
- $HOME/.docker/config.json
- $HOME/.dockercfg`)
		dockerEndpoint        = flag.String("docker.endpoint", "", "Endpoint of the local Docker daemon.")
		dockerNoCache         = flag.String("docker.nocache", "", "Regular `expression` selecting the docker images to forcibly rebuild.")
		dockerPull            = flag.Bool("docker.pull", false, "Refresh base images when building images.")
		dockerOutput          = flag.Bool("docker.output", false, "Relay all docker output to stderr.")
		dockerBuildOutput     = flag.Bool("docker.buildoutput", false, "Relay only docker build output to stderr.")
		simPattern            = flag.String("sim", "", "Regular `expression` selecting the simulators to run.")
		simTestPattern        = flag.String("sim.limit", "", "Regular `expression` selecting tests/suites (interpreted by simulators).")
		simParallelism        = flag.Int("sim.parallelism", 1, "Max `number` of parallel clients/containers (interpreted by simulators).")
		simRandomSeed         = flag.Int("sim.randomseed", 0, "Randomness seed number (interpreted by simulators).")
		simTestLimit          = flag.Int("sim.testlimit", 0, "[DEPRECATED] Max `number` of tests to execute per client (interpreted by simulators).")
		simTimeLimit          = flag.Duration("sim.timelimit", 0, "Simulation `timeout`. Hive aborts the simulator if it exceeds this time.")
		simLogLevel           = flag.Int("sim.loglevel", 3, "Selects log `level` of client instances. Supports values 0-5.")
		simDevMode            = flag.Bool("dev", false, "Only starts the simulator API endpoint (listening at 127.0.0.1:3000 by default) without starting any simulators.")
		simDevModeAPIEndpoint = flag.String("dev.addr", "127.0.0.1:3000", "Endpoint that the simulator API listens on")
		useCredHelper         = flag.Bool("docker.cred-helper", false, "(DEPRECATED) Use --docker.auth instead.")

		// Cleanup flags
		cleanupContainers = flag.Bool("cleanup", false, "Clean up Hive containers instead of running simulations")
		cleanupDryRun     = flag.Bool("cleanup.dry-run", false, "Show what containers would be cleaned up without actually removing them")
		cleanupInstance   = flag.String("cleanup.instance", "", "Clean up containers from specific Hive instance ID only")
		cleanupType       = flag.String("cleanup.type", "", "Clean up specific container type only (client, simulator, proxy)")
		cleanupOlderThan  = flag.Duration("cleanup.older-than", 0, "Clean up containers older than specified duration (e.g., 1h, 24h)")
		listContainers    = flag.Bool("list", false, "List Hive containers instead of running simulations")

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
	simBuildArgs := make(buildArgs)
	flag.Var(&simBuildArgs, "sim.buildarg", "Argument to pass to the docker engine when building the simulator image, in the form of ARGNAME=VALUE.")

	// Parse the flags and configure the logger.
	flag.Parse()
	terminal := os.Getenv("TERM")
	tintHandler := tint.NewHandler(os.Stderr, &tint.Options{
		Level:   convertLogLevel(*loglevelFlag),
		NoColor: terminal == "" || terminal == "dumb",
	})
	slog.SetDefault(slog.New(tintHandler))
	// See: https://github.com/ethereum/hive/issues/1200.
	if err := os.Setenv("GODEBUG", "multipartmaxparts=20000"); err != nil {
		fatal(err)
	}
	if *simTestLimit > 0 {
		slog.Warn("Option --sim.testlimit is deprecated and will have no effect.")
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
		slog.Warn("--sim is ignored when using --dev mode")
		simList = nil
	}

	// Create the docker backends.
	dockerConfig := &libdocker.Config{
		Inventory:         inv,
		PullEnabled:       *dockerPull,
		UseAuthentication: *dockerAuth || *useCredHelper,
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
	} else if *dockerBuildOutput {
		dockerConfig.BuildOutput = os.Stderr
	}
	builder, cb, err := libdocker.Connect(*dockerEndpoint, dockerConfig)
	if err != nil {
		fatal(err)
	}

	// Handle cleanup/list operations if requested.
	if *cleanupContainers || *listContainers {
		dockerClient := cb.GetDockerClient()
		if dockerClient == nil {
			fatal("Docker client not available for cleanup operations")
		}
		
		client, ok := dockerClient.(*docker.Client)
		if !ok {
			fatal("Invalid Docker client type")
		}
		
		if *listContainers {
			err := libhive.ListHiveContainers(context.Background(), client, *cleanupInstance)
			if err != nil {
				fatal("Failed to list containers:", err)
			}
			return
		}

		if *cleanupContainers {
			cleanupOpts := libhive.CleanupOptions{
				InstanceID:    *cleanupInstance,
				OlderThan:     *cleanupOlderThan,
				DryRun:        *cleanupDryRun,
				ContainerType: *cleanupType,
			}
			err := libhive.CleanupHiveContainers(context.Background(), client, cleanupOpts)
			if err != nil {
				fatal("Failed to cleanup containers:", err)
			}
			return
		}
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
	hiveInfo := libhive.HiveInfo{
		Command:        os.Args,
		ClientFile:     clientList,
		ClientFilePath: *clientsFile,
	}

	// Build clients and simulators.
	if err := runner.Build(ctx, clientList, simList, simBuildArgs); err != nil {
		fatal(err)
	}
	if *simDevMode {
		runner.RunDevMode(ctx, env, *simDevModeAPIEndpoint, hiveInfo)
		return
	}

	// Run simulators.
	var failCount int
	for _, sim := range simList {
		result, err := runner.Run(ctx, sim, env, hiveInfo)
		if err != nil {
			fatal(err)
		}
		failCount += result.TestsFailed
		slog.Info(fmt.Sprintf("simulation %s finished", sim), "suites", result.Suites, "tests", result.Tests, "failed", result.TestsFailed)
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

// convertLogLevel maps log level from range 0-5 to the range used by package slog.
// Input levels are ordered in increasing amounts of messages, i.e. level zero is silent
// and level 5 means everything is printed.
func convertLogLevel(level int) slog.Level {
	switch level {
	case 0:
		return 99
	case 1:
		return slog.LevelError
	case 2:
		return slog.LevelWarn
	case 3:
		return slog.LevelInfo
	default:
		return slog.LevelDebug
	}
}
