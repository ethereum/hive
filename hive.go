package main

import (
	"flag"
	"os"
	"syscall"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

var (
	dockerEndpoint   = flag.String("docker-endpoint", "unix:///var/run/docker.sock", "Unix socket to th local Docker daemon")
	smokePattern     = flag.String("smoke", "", "Regexp selecting the client(s) to smoke-test")
	validatePattern  = flag.String("validate", "[^:]+:master", "Regexp selecting the client(s) to validate")
	validatorPattern = flag.String("validators", ".", "Regexp selecting the validation tests to run")
	overrideClient   = flag.String("override", "", "Client binary to override the default one with")
	noImageCache     = flag.Bool("nocache", false, "Disabled image caching, rebuilding all modified docker images")
	loglevelFlag     = flag.Int("loglevel", 3, "Log level to use for displaying system events")
)

func main() {
	// Parse the flags and configure the logger
	flag.Parse()
	log15.Root().SetHandler(log15.MultiHandler(
		log15.LvlFilterHandler(log15.Lvl(*loglevelFlag), log15.StdoutHandler),
		log15.LvlFilterHandler(log15.LvlDebug, log15.StreamHandler(os.Stderr, log15.LogfmtFormat())),
	))
	log, _ := os.OpenFile("log.txt", os.O_WRONLY|os.O_CREATE|os.O_SYNC|os.O_TRUNC, 0644)
	syscall.Dup2(int(log.Fd()), 2)

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

	switch {
	case *smokePattern != "":
		// If smoke testing is requested, run only part of the validators
		if err := validateClients(daemon, *smokePattern, "smoke/.", *overrideClient, true); err != nil {
			log15.Crit("failed to smoke-test client images", "error", err)
			return
		}

	case *validatePattern != "":
		// If validation is requested, run only the standalone tests
		if err := validateClients(daemon, *validatePattern, *validatorPattern, *overrideClient, *noImageCache); err != nil {
			log15.Crit("failed to validate client images", "error", err)
			return
		}
	}
}
