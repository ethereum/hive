package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/ethereum/hive/chaintools"
	docker "github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

//errors
var (
	// errNoMatchingClients is returned when no matching clients are found for a given --client regexp value
	errNoMatchingClients = errors.New("no matching clients found")
)

//flags
var (
	dockerEndpoint = flag.String("docker-endpoint", "unix:///var/run/docker.sock", "Endpoint to the local Docker daemon")

	//TODO - this needs to be passed on to the shell container if it is being used
	dockerHostAlias = flag.String("docker-hostalias", "unix:///var/run/docker.sock", "Endpoint to the host Docket daemon from within a validator")

	testResultsRoot = flag.String("results-root", "workspace/logs", "Target folder for results output and historical results aggregation")

	noShellContainer = flag.Bool("docker-noshell", false, "Disable outer docker shell, running directly on the host")
	noCachePattern   = flag.String("docker-nocache", "", "Regexp selecting the docker images to forcibly rebuild")

	clientListFlag     = flag.String("client", "go-ethereum_latest", "Comma separated list of permitted clients for the test type, where client is formatted clientname_branch eg: go-ethereum_latest and the client name is a subfolder of the clients directory")
	checkTimeLimitFlag = flag.Duration("client.checktimelimit", 3*time.Minute, "The timeout to wait for a newly "+
		"instantiated client to open up the RPC port. If a very long chain is imported, this timeout may need to be quite large. "+
		"A lower value means that hive won't wait as long for in case node crashes and never opens the RPC port.")

	overrideFiles = flag.String("override", "", "Comma separated regexp:files to override in client containers")
	smokeFlag     = flag.Bool("smoke", false, "Whether to only smoke test or run full test suite")

	simulatorPattern     = flag.String("sim", "", "Regexp selecting the simulation tests to run")
	simulatorParallelism = flag.Int("sim.parallelism", 1, "Max number of parallel clients/containers to run tests against")
	simulatorTestLimit   = flag.Int("sim.testlimit", -1, "Max number of tests to execute per client (interpreted by simulators)")
	simRootContext       = flag.Bool("sim.rootcontext", false, "Indicates if the simulation should build "+
		"the dockerfile with root (simulator) or local context. Needed for access to sibling folders like simulators/common")
	simLimiterFlag  = flag.Int("sim.timelimit", -1, "Run all simulators with a time limit in seconds, -1 being unlimited")
	simloglevelFlag = flag.Int("sim.loglevel", 3, "The base log level for simulator client instances. "+
		"This number from 0-6 is interpreted differently depending on the client type.")

	hiveMaxTestsFlag = flag.Int("hivemaxtestcount", -1, "Limit the number of tests the simulator is permitted to generate in a testsuite for the Hive provider. "+
		"Used for smoke testing consensus tests themselves.")

	hiveDebug = flag.Bool("debug", false, "A flag indicating debug mode, to allow docker containers to launch headless delve instances and so on")

	chainGenerate   = flag.Bool("chainGenerate", false, "Tell Hive to generate a blockchain on the basis of a supplied genesis and terminate")
	chainLength     = flag.Uint("chainLength", 2, "The length of the chain to generate")
	chainConfig     = flag.String("chainConfig", "", "Reserved for future usage. Will allow Hive to generate test chains of different types")
	chainOutputPath = flag.String("chainOutputPath", ".", "Chain destination folder")
	chainGenesis    = flag.String("chainGenesis", "", "The path and filename to the source genesis.json")
	chainBlockTime  = flag.Uint("chainBlockTime", 30, "The desired block time in seconds")

	loglevelFlag = flag.Int("loglevel", 3, "Log level to use for displaying system events")
)

var (
	clientList           []string                           //the list of permitted clients specified by the user
	allClients           map[string]string                  //map of client names (name_branch format) to docker image names
	allPseudos           map[string]string                  //map of pseudo names to docker image names
	allClientVersions    map[string]map[string]string       //map of client names (name_branch format) to a general json struct (map[string]string) containing the version info
	dockerClient         *docker.Client                     //the web client to the docker api
	timeoutCheckDuration = time.Duration(120 * time.Second) //liveness check timeout
)

func main() {
	// Make sure hive can use multiple CPU cores when needed
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Parse the flags and configure the logger
	flag.Parse()
	timeoutCheckDuration = *checkTimeLimitFlag
	log15.Root().SetHandler(log15.LvlFilterHandler(log15.Lvl(*loglevelFlag), log15.StreamHandler(os.Stderr, log15.TerminalFormat())))
	if *chainGenerate {
		chaintools.ProduceTestChainFromGenesisFile(*chainGenesis, *chainOutputPath, *chainLength, *chainBlockTime)
		return
	}
	// Get the list of clients
	clientList = strings.Split(*clientListFlag, ",")
	for i := range clientList {
		clientList[i] = strings.TrimSpace(clientList[i])
	}
	// Connect to the local docker daemon and make sure it works
	var err error
	dockerClient, err = docker.NewClient(*dockerEndpoint)
	if err != nil {
		log15.Crit("failed to connect to docker deamon", "error", err)
		return
	}
	env, err := dockerClient.Version()
	if err != nil {
		log15.Crit("failed to retrieve docker version", "error", err)
		return
	}
	log15.Info("docker daemon online", "version", env.Get("Version"))
	//Gather any client files needing overriding and images not caching
	//TODO check this file override requirement here:
	overrides := []string{}
	if *overrideFiles != "" {
		overrides = strings.Split(*overrideFiles, ",")
	}
	cacher, err := newBuildCacher(*noCachePattern)
	if err != nil {
		log15.Crit("failed to parse nocache regexp", "error", err)
		return
	}
	// create hive error reporter
	errorReport := NewHiveErrorReport()
	//set up clients and get their versions
	if err := initClients(cacher, NewHiveErrorReport()); err != nil {
		log15.Crit("failed to initialize client(s), terminating test...")
		errorReport.WriteReport(fmt.Sprintf("%s/errorReport.json", *testResultsRoot))
		os.Exit(-1)
	}
	// Depending on the flags, either run hive in place or in an outer container shell
	var fail error
	if *noShellContainer {
		fail = mainInHost(overrides, cacher, errorReport)
	} else {
		fail = mainInShell(overrides, cacher, errorReport)
	}
	if err := errorReport.WriteReport(fmt.Sprintf("%s/errorReport.json", *testResultsRoot)); err != nil {
		log15.Crit("could not write error report", "error", err)
	}
	if fail != nil {
		os.Exit(-1)
	}
}

// mainInHost runs the actual hive testsuites on the
// host machine itself. This is usually the path executed within an outer shell
// container, but can be also requested directly.
func mainInHost(overrides []string, cacher *buildCacher, errorReport *HiveErrorReport) error {
	var err error

	// create or use the specified rootpath
	log15.Info("Creating output folder if necessary", "folder", *testResultsRoot)
	if err := os.MkdirAll(*testResultsRoot, os.ModePerm); err != nil {
		log15.Crit("failed to create logs folder", "error", err)
		return err
	}
	// Run all testsuites
	if *simulatorPattern != "" {
		//execute testsuites
		if err = runSimulations(*simulatorPattern, overrides, cacher, errorReport); err != nil {
			log15.Crit("failed to run simulations", "error", err)
			return err
		}
	}

	return nil

}

// initClients builds any docker images needed and maps
// client name_branchs
func initClients(cacher *buildCacher, errorReport *HiveErrorReport) error {
	var err error
	// Build all the clients that we need and make a map of
	// names (eg: geth_latest, in the format client_branch )
	// against image names in the docker image name format
	allClients, err = buildClients(clientList, cacher, errorReport)
	if err != nil {
		log15.Crit("failed to build client images", "error", err)
		return err
	}
	// Build all pseudo clients. pseudo-clients need to be available
	// to simulators. pseudo-clients play the role of special types
	// of actor in a network, such as network relay for example
	allPseudos, err = buildPseudoClients("pseudo", cacher, errorReport)
	if err != nil {
		log15.Crit("failed to build client images", "error", err)
		return err
	}
	// Retrieve the version information of all clients being tested
	if allClientVersions, err = fetchClientVersions(cacher); err != nil {
		log15.Crit("failed to retrieve client versions", "error", err)
		return err
	}
	return nil
}
