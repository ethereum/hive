package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

var (
	dockerEndpoint = flag.String("docker-endpoint", "unix:///var/run/docker.sock", "Endpoint to the local Docker daemon")

	//TODO - this needs to be passed on to the shell container if it is being used
	dockerHostAlias = flag.String("docker-hostalias", "unix:///var/run/docker.sock", "Endpoint to the host Docket daemon from within a validator")

	testResultsRoot        = flag.String("results-root", "workspace/logs", "Target folder for results output and historical results aggregation")
	testResultsSummaryFile = flag.String("summary-file", "listings.json", "Test run summary file to which summaries are appended")

	noShellContainer = flag.Bool("docker-noshell", false, "Disable outer docker shell, running directly on the host")
	noCachePattern   = flag.String("docker-nocache", "", "Regexp selecting the docker images to forcibly rebuild")

	clientPattern = flag.String("client", "_master", "Regexp selecting the client(s) to run against")
	overrideFiles = flag.String("override", "", "Comma separated regexp:files to override in client containers")
	smokeFlag     = flag.Bool("smoke", false, "Whether to only smoke test or run full test suite")

	validatorPattern = flag.String("test", ".", "Regexp selecting the validation tests to run")
	simulatorPattern = flag.String("sim", "", "Regexp selecting the simulation tests to run")
	benchmarkPattern = flag.String("bench", "", "Regexp selecting the benchmarks to run")

	loglevelFlag = flag.Int("loglevel", 3, "Log level to use for displaying system events")

	dockerTimeout         = flag.Int("dockertimeout", 10, "Time to wait for container to finish before stopping it")
	dockerTimeoutDuration = time.Duration(*dockerTimeout) * time.Minute
	timeoutCheck          = flag.Int("timeoutcheck", 30, "Seconds to check for timeouts of containers")
	timeoutCheckDuration  = time.Duration(*timeoutCheck) * time.Second

	runPath = time.Now().Format("20060102150405")
)

func main() {
	// Make sure hive can use multiple CPU cores when needed
	runtime.GOMAXPROCS(runtime.NumCPU())

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

	// Gather any client files needing overriding and images not caching
	overrides := []string{}
	if *overrideFiles != "" {
		overrides = strings.Split(*overrideFiles, ",")
	}
	cacher, err := newBuildCacher(*noCachePattern)
	if err != nil {
		log15.Crit("failed to parse nocache regexp", "error", err)
		return
	}
	// Depending on the flags, either run hive in place or in an outer container shell
	var fail error
	if *noShellContainer {
		fail = mainInHost(daemon, overrides, cacher)
	} else {
		fail = mainInShell(daemon, overrides, cacher)
	}
	if fail != nil {
		os.Exit(-1)
	}
}

func makeTestOutputDirectory(testName string, testCategory string, clientTypes []string) (string, error) {

	testName = strings.Replace(testName, "\\", "_", -1)

	//<WORKSPACE/LOGS>/20191803261015/validator_devp2p/
	testRoot := filepath.Join(*testResultsRoot, runPath, testCategory+"_"+testName)

	for _, client := range clientTypes {
		outputDir := filepath.Join(testRoot, client)
		if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
			return "", err
		}
	}

	//write out the test metadata
	testInfo := struct {
		Category string   `json:"category,omitempty"`
		Name     string   `json:"name,omitempty"`
		Clients  []string `json:"clients,omitempty"`
	}{Category: testCategory, Name: testName, Clients: clientTypes}

	testInfoJSON, err := json.MarshalIndent(testInfo, "", "  ")
	if err != nil {
		log15.Crit("failed to report results", "error", err)
		return "", err
	}

	testInfoJSONFileName := filepath.Join(testRoot, "testInfo.json")

	testInfoJSONFile, err := os.OpenFile(testInfoJSONFileName, os.O_WRONLY|os.O_CREATE|os.O_SYNC|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return "", err
	}

	_, err = testInfoJSONFile.Write(testInfoJSON)
	if err != nil {
		return "", err
	}

	return testRoot, nil
}

type summaryData struct {
	NSuccesses  int `json:"n_successes"`  //Number of successes
	NFails      int `json:"n_fails"`      //Number of fails
	NSubresults int `json:"n_subresults"` //Number of subresults
}

type resultSet struct {
	Clients     map[string]map[string]string            `json:"clients,omitempty"`
	Validations map[string]map[string]*validationResult `json:"validations,omitempty"`
	Simulations map[string]map[string]*simulationResult `json:"simulations,omitempty"`
	Benchmarks  map[string]map[string]*benchmarkResult  `json:"benchmarks,omitempty"`
}

type resultSetSummary struct {
	resultSet
	FileName string `json:"filename"`
}

type summaryFile struct {
	Files []resultSetSummary
}

func summariseResults(results *resultSet, detailFile string) resultSetSummary {

	for _, v := range results.Validations {
		for _, v2 := range v {
			v2.NSubresults = 1
			if v2.Success {
				v2.NSuccesses = 1
				v2.NFails = 0
			} else {
				v2.NSuccesses = 1
				v2.NFails = 0
			}
		}
	}

	for _, b := range results.Benchmarks {
		for _, b2 := range b {
			b2.NSubresults = 1
			if b2.Success {
				b2.NSuccesses = 1
				b2.NFails = 0
			} else {
				b2.NSuccesses = 1
				b2.NFails = 0
			}
		}
	}

	//TODO Change this type - it expects a number for subresults
	for _, s := range results.Simulations {
		for _, s2 := range s {
			s2.NSuccesses = 0
			s2.NFails = 0
			for _, sub := range s2.Subresults {
				if sub.Success {
					s2.NSuccesses++
				} else {
					s2.NFails++
				}
			}
			s2.Subresults = nil //remove this from the summary
		}
	}

	res := resultSetSummary{}
	res.resultSet = *results
	res.FileName = detailFile
	return res

}

// mainInHost runs the actual hive validation, simulation and benchmarking on the
// host machine itself. This is usually the path executed within an outer shell
// container, but can be also requested directly.
func mainInHost(daemon *docker.Client, overrides []string, cacher *buildCacher) error {
	results := resultSet{}
	var err error

	// Retrieve the versions of all clients being tested
	if results.Clients, err = fetchClientVersions(daemon, *clientPattern, cacher); err != nil {
		log15.Crit("failed to retrieve client versions", "error", err)
		b, ok := err.(*buildError)
		if ok {
			results.Clients = make(map[string]map[string]string)
			results.Clients[b.Client()] = map[string]string{"error": b.Error()}
			out, errMarshal := json.MarshalIndent(results, "", "  ")
			if errMarshal != nil {
				log15.Crit("failed to report results. Docker Failed build.", "error", err)
				return err
			}
			fmt.Println(string(out))
		}
		return err
	}
	// Smoke tests are exclusive with all other flags
	if *smokeFlag {
		if results.Validations, err = validateClients(daemon, *clientPattern, "smoke", overrides, cacher); err != nil {
			log15.Crit("failed to smoke-validate client images", "error", err)
			return err
		}
		if results.Simulations, err = simulateClients(daemon, *clientPattern, "smoke", overrides, cacher); err != nil {
			log15.Crit("failed to smoke-simulate client images", "error", err)
			return err
		}
		if results.Benchmarks, err = benchmarkClients(daemon, *clientPattern, "smoke", overrides, cacher); err != nil {
			log15.Crit("failed to smoke-benchmark client images", "error", err)
			return err
		}
	} else {
		// Otherwise run all requested validation and simulation tests
		if *validatorPattern != "" {
			if results.Validations, err = validateClients(daemon, *clientPattern, *validatorPattern, overrides, cacher); err != nil {
				log15.Crit("failed to validate clients", "error", err)
				return err
			}
		}
		if *simulatorPattern != "" {
			if err = makeGenesisDAG(daemon, cacher); err != nil {
				log15.Crit("failed generate DAG for simulations", "error", err)
				return err
			}
			if results.Simulations, err = simulateClients(daemon, *clientPattern, *simulatorPattern, overrides, cacher); err != nil {
				log15.Crit("failed to simulate clients", "error", err)
				return err
			}
		}
		if *benchmarkPattern != "" {
			if results.Benchmarks, err = benchmarkClients(daemon, *clientPattern, *benchmarkPattern, overrides, cacher); err != nil {
				log15.Crit("failed to benchmark clients", "error", err)
				return err
			}
		}
	}
	// Flatten the results and print them in JSON form
	out, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		log15.Crit("failed to report results", "error", err)
		return err
	}
	fmt.Println(string(out))

	//send the output to a file as log.json in the run root
	logFileName := filepath.Join(*testResultsRoot, runPath, "log.json")
	logFile, err := os.OpenFile(logFileName, os.O_WRONLY|os.O_CREATE|os.O_SYNC|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	_, err = logFile.WriteString(string(out))
	if err != nil {
		return err
	}
	logFile.Close()

	//process the output into a summary and append it to the summary index
	resultSummary := summariseResults(&results, logFileName)

	summaryFileName := filepath.Join(*testResultsRoot, *testResultsSummaryFile)

	//read the existing summary data
	summaryFileData, err := ioutil.ReadFile(summaryFileName)
	if err != nil {
		log15.Crit("failed to report summarised results", "error", err)
		return err
	}

	//back it up
	ioutil.WriteFile(summaryFileName+".bak", summaryFileData, 0644)

	//deserialize from json
	var allSummaryInfo summaryFile
	err = json.Unmarshal(summaryFileData, &allSummaryInfo)
	if err != nil {
		log15.Crit("failed to read summarised results", "error", err)
		return err
	}

	//add the new summary
	allSummaryInfo.Files = append(allSummaryInfo.Files, resultSummary)

	//now serialize and write out
	newSummary, err := json.MarshalIndent(allSummaryInfo, "", "  ")
	if err != nil {
		log15.Crit("failed to report summarised results", "error", err)
		return err
	}

	err = ioutil.WriteFile(summaryFileName, newSummary, 0644)
	if err != nil {
		log15.Crit("failed to report summarised results", "error", err)
		return err
	}

	return nil
}
