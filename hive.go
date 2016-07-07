package main

import (
	"flag"
	"os"
	"strings"
	"syscall"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

var (
	dockerEndpoint = flag.String("docker-endpoint", "unix:///var/run/docker.sock", "Unix socket to th local Docker daemon")
	noImageCache   = flag.Bool("nocache", false, "Disabled image caching, rebuilding all modified docker images")

	smokePattern     = flag.String("smoke", "", "Regexp selecting the client(s) to smoke-test")
	validatePattern  = flag.String("validate", ":master", "Regexp selecting the client(s) to validate")
	validatorPattern = flag.String("validators", ".", "Regexp selecting the validation tests to run")
	simulatePattern  = flag.String("simulate", "", "Regexp selecting the client(s) to simulate")
	simulatorPattern = flag.String("simulators", ".", "Regexp selecting the simulation tests to run")
	overrideFiles    = flag.String("override", "", "Comma separated files to override in client containers")

	loglevelFlag = flag.Int("loglevel", 3, "Log level to use for displaying system events")
)

func main() {
	// Parse the flags and configure the logger
	flag.Parse()
	if *loglevelFlag < 6 {
		log15.Root().SetHandler(log15.MultiHandler(
			log15.LvlFilterHandler(log15.Lvl(*loglevelFlag), log15.StdoutHandler),
			log15.LvlFilterHandler(log15.LvlDebug, log15.StreamHandler(os.Stderr, log15.LogfmtFormat())),
		))
		log, _ := os.OpenFile("log.txt", os.O_WRONLY|os.O_CREATE|os.O_SYNC|os.O_TRUNC, 0644)
		syscall.Dup2(int(log.Fd()), 2)
	} else {
		log15.Root().SetHandler(log15.LvlFilterHandler(log15.LvlDebug, log15.StderrHandler))
	}
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
	// Smoke tests are exclusive with all other flags
	if *smokePattern != "" {
		if err := validateClients(daemon, *smokePattern, "smoke/", overrides, true); err != nil {
			log15.Crit("failed to smoke-validate client images", "error", err)
			return
		}
		if err := simulateClients(daemon, *smokePattern, "smoke/", overrides, true); err != nil {
			log15.Crit("failed to smoke-simulate client images", "error", err)
			return
		}
		return
	}
	// Otherwise run all requested validation and simulation tests
	if *validatePattern != "" {
		if err := validateClients(daemon, *validatePattern, *validatorPattern, overrides, *noImageCache); err != nil {
			log15.Crit("failed to validate clients", "error", err)
			return
		}
	}
	if *simulatePattern != "" {
		if err := simulateClients(daemon, *simulatePattern, *simulatorPattern, overrides, *noImageCache); err != nil {
			log15.Crit("failed to simulate clients", "error", err)
			return
		}
	}
}
