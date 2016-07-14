package main

import (
	"flag"
	"os"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

var (
	dockerEndpoint   = flag.String("docker-endpoint", "unix:///var/run/docker.sock", "Unix socket to the local Docker daemon")
	noShellContainer = flag.Bool("docker-noshell", false, "Disable outer docker shell, running directly on the host")

	clientPattern = flag.String("client", ":master", "Regexp selecting the client(s) to run against")
	overrideFiles = flag.String("override", "", "Comma separated files to override in client containers")
	smokeFlag     = flag.Bool("smoke", false, "Whether to only smoke test or run full test suite")

	validatorPattern = flag.String("test", ".", "Regexp selecting the validation tests to run")
	simulatorPattern = flag.String("sim", "", "Regexp selecting the simulation tests to run")

	loglevelFlag = flag.Int("loglevel", 3, "Log level to use for displaying system events")
)

func main() {
	// Parse the flags and configure the logger
	flag.Parse()
	log15.Root().SetHandler(log15.LvlFilterHandler(log15.Lvl(*loglevelFlag), log15.StreamHandler(os.Stderr, log15.TerminalFormat())))

	// Connect to the local docker daemon and make sure it works
	daemon, err := docker.NewClient(*dockerEndpoint)
	if err != nil {
		log15.Crit("failed to connect to docker deamon", "error", err)
		return
	}
	env, err := daemon.Version()
	if err != nil {
		log15.Crit("failed to retrieve docker version", "error", err)
		return
	}
	log15.Info("docker daemon online", "version", env.Get("Version"))

	// Gather any client files needing overriding
	overrides := []string{}
	if *overrideFiles != "" {
		overrides = strings.Split(*overrideFiles, ",")
	}
	// Depending on the flags, either run hive in place or in an outer container shell
	var fail error
	if *noShellContainer {
		fail = mainInHost(daemon, overrides)
	} else {
		fail = mainInShell(daemon, overrides)
	}
	if fail != nil {
		os.Exit(-1)
	}
}

// mainInHost runs the actual hive validation, simulation and benchmarking on the
// host machine itself. This is usually the path executed within an outer shell
// container, but can be also requested directly.
func mainInHost(daemon *docker.Client, overrides []string) error {
	// Smoke tests are exclusive with all other flags
	if *smokeFlag {
		if err := validateClients(daemon, *clientPattern, "smoke/", overrides); err != nil {
			log15.Crit("failed to smoke-validate client images", "error", err)
			return err
		}
		if err := simulateClients(daemon, *clientPattern, "smoke/", overrides); err != nil {
			log15.Crit("failed to smoke-simulate client images", "error", err)
			return err
		}
		return nil
	}
	// Otherwise run all requested validation and simulation tests
	if *validatorPattern != "" {
		if err := validateClients(daemon, *clientPattern, *validatorPattern, overrides); err != nil {
			log15.Crit("failed to validate clients", "error", err)
			return err
		}
	}
	if *simulatorPattern != "" {
		if err := simulateClients(daemon, *clientPattern, *simulatorPattern, overrides); err != nil {
			log15.Crit("failed to simulate clients", "error", err)
			return err
		}
	}
	return nil
}
