package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/hive/simulators/common"
	"github.com/gorilla/pat"

	docker "github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

var (
	simListenerAddress string
	testManager        *common.TestManager
	server             *http.Server
)

// runSimulations runs each 'simulation' container, which are hosts for executing one or more test-suites
func runSimulations(simulatorPattern string, overrides []string, cacher *buildCacher) error {

	// Clean up
	defer terminateAndUpdate()

	// Build all the simulators known to the test harness
	log15.Info("building simulators for testing", "pattern", simulatorPattern)
	simulators, err := buildSimulators(simulatorPattern, cacher)
	if err != nil {
		return err
	}

	// Create a testcase manager
	testManager = common.NewTestManager(*testResultsRoot, *hiveMaxTestsFlag, killNodeHandler)

	// Start the simulator HTTP API
	err = startTestSuiteAPI()
	if err != nil {
		log15.Error("failed to start simulator API", "error", err)
		return err
	}

	for simulator, simulatorImage := range simulators {
		logger := log15.New("simulator", simulator)

		// TODO -  logdir:
		// A simulator can run multiple test suites so we don't want logfiles directly related to a test suite run in the UI.
		// Also the simulator can fail before a testsuite output is even produced.
		// What the simulator reports is not really interesting from a testing perspective. These messages are
		// about failures or warnings in even executing the testsuites.
		// Possible solution: list executions on a separate page, link from testsuites to executions where possible,
		// and have the execution details including duration, hive logs, and sim logs displayed.
		// logdir will be the execution folder. ie executiondir.
		logdir := *testResultsRoot
		err = simulate(*simLimiterFlag, simulatorImage, simulator, overrides, logger, logdir)
		if err != nil {
			return err
		}

	}
	return nil
}

// simulate runs a simulator container, which is a host for one or more testsuite
// runners. These communicate with this hive testsuite provider and host via
// a client API to run testsuites and their testcases.
func simulate(simDuration int, simulator string, simulatorLabel string, overrides []string, logger log15.Logger, logdir string) error {
	logger.Info(fmt.Sprintf("running client simulation: %s", simulatorLabel))

	// The simulator creates the testŕesult files, aswell as updates the index file. However, it needs to also
	// be aware of the location of it's own logfile, which should also be placed into the index.
	// We generate a unique name here, and pass it to the simulator via ENV vars
	var logName string
	{
		b := make([]byte, 16)
		rand.Read(b)
		logName = fmt.Sprintf("%d-%x-simulator.log", time.Now().Unix(), b)
	}

	// Start the simulator controller container
	logger.Debug("creating simulator container")
	hostConfig := &docker.HostConfig{Privileged: true, CapAdd: []string{"SYS_PTRACE"}, SecurityOpt: []string{"seccomp=unconfined"}}
	sc, err := dockerClient.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: simulator,
			Env: []string{
				fmt.Sprintf("HIVE_SIMULATOR=http://%v", simListenerAddress),
				fmt.Sprintf("HIVE_DEBUG=%v", strconv.FormatBool(*hiveDebug)),
				fmt.Sprintf("HIVE_PARALLELISM=%d", *simulatorParallelism),
				fmt.Sprintf("HIVE_SIMLIMIT=%d", *simulatorTestLimit),
				fmt.Sprintf("HIVE_SIMLOG=%v", logName),
			},
		},
		HostConfig: hostConfig,
	})

	if err != nil {
		logger.Error("failed to create simulator", "error", err)
		return err
	}

	slogger := logger.New("id", sc.ID[:8])
	slogger.Debug("created simulator container")
	defer func() {
		slogger.Debug("deleting simulator container")
		if err := dockerClient.RemoveContainer(docker.RemoveContainerOptions{ID: sc.ID, Force: true}); err != nil {
			slogger.Error("failed to delete simulator container", "error", err)
		}
	}()

	// Start the tester container and wait until it finishes
	slogger.Debug("running simulator container")

	waiter, err := runContainer(sc.ID, slogger, filepath.Join(logdir, logName), false, *loglevelFlag)
	if err != nil {
		slogger.Error("failed to run simulator", "error", err)
		return err
	}

	// if we have a simulation time limiter, then timeout the simulation using the usual go pattern
	if simDuration != -1 {
		//make a timeout channel
		timeoutchan := make(chan error, 1)
		//wait for the simulator in a go routine, then push to the channel
		go func() {
			e := waiter.Wait()
			timeoutchan <- e
		}()
		//work out what the timeout is as a duration
		simTimeoutDuration := time.Duration(simDuration) * time.Second
		//wait for either the waiter or the timeout
		select {
		case err := <-timeoutchan:
			return err
		case <-time.After(simTimeoutDuration):
			err := waiter.Close()
			return err
		}
	} else {
		waiter.Wait()
	}

	return nil

}

// startTestSuiteAPI starts an HTTP webserver listening for simulator commands
// on the docker bridge and executing them until it is torn down.
func startTestSuiteAPI() error {
	// Find the IP address of the host container
	log15.Debug("looking up docker bridge IP")
	bridge, err := lookupBridgeIP(log15.Root())
	if err != nil {
		log15.Error("failed to lookup bridge IP", "error", err)
		return err
	}

	log15.Debug("docker bridge IP found", "ip", bridge)

	// Serve connections until the listener is terminated
	log15.Debug("starting simulator API server")

	var mux *pat.Router = pat.New()
	mux.Get("/testsuite/{suite}/test/{test}/node/{node}", nodeInfoGet)
	mux.Post("/testsuite/{suite}/test/{test}/node", nodeStart)
	mux.Post("/testsuite/{suite}/test/{test}/pseudo", pseudoStart)
	mux.Delete("/testsuite/{suite}/test/{test}/node/{node}", nodeKill)
	mux.Post("/testsuite/{suite}/test/{test}", testDelete) //post because the delete http verb does not always support a message body
	mux.Post("/testsuite/{suite}/test", testStart)
	mux.Delete("/testsuite/{suite}", suiteEnd)
	mux.Post("/testsuite", suiteStart)
	mux.Get("/clients", clientTypesGet)
	// Start the API webserver for simulators to coordinate with
	addr, _ := net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:0", bridge))
	listener, err := net.ListenTCP("tcp4", addr)
	if err != nil {
		log15.Error("failed to listen on bridge adapter", "error", err)
		return err
	}
	simListenerAddress = listener.Addr().String()
	log15.Debug("listening for simulator commands", "ip", bridge, "port", listener.Addr().(*net.TCPAddr).Port)
	server = &http.Server{Handler: mux}

	go server.Serve(listener)

	return nil
}

func checkSuiteRequest(request *http.Request, w http.ResponseWriter) (common.TestSuiteID, error) {
	testSuiteString := request.URL.Query().Get(":suite")
	testSuite, err := strconv.Atoi(testSuiteString)
	if err != nil {
		http.Error(w, "invalid test suite", http.StatusBadRequest)
		return 0, fmt.Errorf("invalid test suite: %v", testSuiteString)
	}
	testSuiteID := common.TestSuiteID(testSuite)
	if _, running := testManager.IsTestSuiteRunning(testSuiteID); !running {
		http.Error(w, "test suite not running", http.StatusBadRequest)
		return 0, fmt.Errorf("test suite not running: %d", testSuite)
	}
	return testSuiteID, nil
}

func checkTestRequest(request *http.Request, w http.ResponseWriter) (common.TestID, bool) {
	testString := request.URL.Query().Get(":test")
	testCase, err := strconv.Atoi(testString)
	if err != nil {
		log15.Error("invalid test case", "identifier", testString)
		http.Error(w, "invalid test case", http.StatusBadRequest)
		return 0, false
	}
	testCaseID := common.TestID(testCase)
	if _, running := testManager.IsTestRunning(testCaseID); !running {
		log15.Error("test case not running", "testId", testCaseID)
		http.Error(w, "test case not running", http.StatusBadRequest)
		return 0, false
	}
	return testCaseID, true
}

// nodeInfoGet tries to execute the mandatory enode.sh , which returns the enode id
func nodeInfoGet(w http.ResponseWriter, request *http.Request) {
	testSuite, err := checkSuiteRequest(request, w)
	if err != nil {
		log15.Error("nodeInfoGet failed", "error", err)
		return
	}
	testCase, ok := checkTestRequest(request, w)
	if !ok {
		log15.Info("Server - node info get, test request failed")
		return
	}
	node := request.URL.Query().Get(":node")
	log15.Info("Server - node info get")

	nodeInfo, err := testManager.GetNodeInfo(common.TestSuiteID(testSuite), common.TestID(testCase), node)
	if err != nil {
		log15.Error("nodeInfoGet unable to get node", "node", node, "error", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	exec, err := dockerClient.CreateExec(docker.CreateExecOptions{
		AttachStdout: true,
		AttachStderr: false,
		Tty:          false,
		Cmd:          []string{"/enode.sh"},
		Container:    nodeInfo.ID, //id is the container id
	})
	if err != nil {
		log15.Error("nodeInfoGet unable to create enode exec", "node", node, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = dockerClient.StartExec(exec.ID, docker.StartExecOptions{
		Detach:       false,
		OutputStream: w,
	})
	if err != nil {
		log15.Error("nodeInfoGet unable to start enode exec", "node", node, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

//start a new node as part of a test
func nodeStart(w http.ResponseWriter, request *http.Request) {
	if _, err := checkSuiteRequest(request, w); err != nil {
		log15.Error("nodeStart failed", "error", err)
		return
	}
	testCase, ok := checkTestRequest(request, w)
	if !ok {
		return
	}
	if err := request.ParseMultipartForm((1 << 10) * 4); err != nil {
		log15.Error("nodeStart: Could not parse node request", "error", err)
		http.Error(w, "Could not parse node request", http.StatusBadRequest)
		return
	}
	files := make(map[string]*multipart.FileHeader)
	for key, fheaders := range request.MultipartForm.File {
		if len(fheaders) > 0 {
			files[key] = fheaders[0]
		}
	}
	envs := make(map[string]string)
	for key, vals := range request.MultipartForm.Value {
		envs[key] = vals[0]
	}
	//TODO logdir
	logdir := *testResultsRoot
	nodeInfo, nodeID, ok := newNode(w, envs, files, allClients, request, true, true, logdir)
	testManager.RegisterNode(testCase, nodeID, nodeInfo)
	log15.Debug("nodeStart", "nodeID", nodeID, "ok", ok)
}

//start a pseudo client and register it as part of a test
func pseudoStart(w http.ResponseWriter, request *http.Request) {
	if _, err := checkSuiteRequest(request, w); err != nil {
		log15.Error("pseudoStart failed", "error", err)
		return
	}
	testCase, ok := checkTestRequest(request, w)
	if !ok {
		return
	}
	log15.Info("Server - pseudo start request")
	// parse any envvar overrides from simulators
	request.ParseForm()
	envs := make(map[string]string)
	for key, vals := range request.Form {
		envs[key] = vals[0]
	}
	//TODO logdir
	logdir := *testResultsRoot
	nodeInfo, nodeID, ok := newNode(w, envs, nil, allPseudos, request, false, false, logdir)
	if ok {
		testManager.RegisterPseudo(testCase, nodeID, nodeInfo)
	}
}

func killNodeHandler(testSuite common.TestSuiteID, test common.TestID, node string) error {
	//attempt to get the client or pseudo
	nodeInfo, err := testManager.GetNodeInfo(testSuite, test, node)
	if err != nil {
		log15.Error(fmt.Sprintf("unable to get node: %s", err.Error()))
		return err
	}
	clientName := nodeInfo.Name
	containerID := nodeInfo.ID //using the ID as 'container id'
	log15.Debug("deleting client container", "name", clientName, "id", containerID)
	err = dockerClient.RemoveContainer(docker.RemoveContainerOptions{ID: containerID, Force: true})
	return err
}

func nodeKill(w http.ResponseWriter, request *http.Request) {

	testSuite, err := checkSuiteRequest(request, w)
	if err != nil {
		log15.Debug("nodeKill failed", "error", err)
		return
	}
	testCase, ok := checkTestRequest(request, w)
	if !ok {
		log15.Error("nodeKill failed")
		return
	}
	node := request.URL.Query().Get(":node")
	err = killNodeHandler(testSuite, testCase, node)
	if err != nil {
		log15.Error("nodeKill unable to delete node", "node", node, "error", err)
		msg := fmt.Sprintf("unable to delete node: %s", err.Error())
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	log15.Debug("nodeKill Ok", "node", node)
}

func testDelete(w http.ResponseWriter, request *http.Request) {
	testSuite, err := checkSuiteRequest(request, w)
	if err != nil {
		log15.Error("testDelete: test end request failed", "error", err)
		return
	}
	testCase, ok := checkTestRequest(request, w)
	if !ok {
		log15.Error("testDelete: test end request failed, checkTestRequest failed")
		return
	}

	// Regardless of earlier errors, we need to at least try to end the test, to
	// clean up resources
	var summaryResult common.TestResult
	var clientResults map[string]*common.TestResult
	var responseWritten = false // Have we already committed a response?
	defer func() {
		//nb! endtest invokes the kill node handler indirectly
		err := testManager.EndTest(testSuite, testCase, &summaryResult, clientResults)
		if err == nil {
			return
		}
		log15.Error("testDelete: request failed, unable to end testcase", "error", err)
		if !responseWritten {
			http.Error(w, fmt.Sprintf("unable to end test case: %s", err.Error()), http.StatusInternalServerError)
		}
	}()

	dict := parseForm(request)
	summaryResultData, ok := dict["summaryresult"]
	if !ok {
		log15.Error("testDelete: request failed, missing summary result")
		msg := fmt.Sprintf("missing summary result")
		responseWritten = true
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	// Summary result is required
	if err = json.Unmarshal([]byte(summaryResultData), &summaryResult); err != nil {
		log15.Error("testDelete: request failed, unmarshalling failed", "error", err)
		msg := fmt.Sprintf("summary result could not be unmarshalled")
		responseWritten = true
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	//clientResults are optional
	if clientResultsData, ok := dict["clientresults"]; ok {
		if err := json.Unmarshal([]byte(clientResultsData), &clientResults); err != nil {
			log15.Error("testDelete: client result unmarhalling failed", "error", err)
		}
	}
}

func parseForm(r *http.Request) map[string]string {
	r.ParseForm()
	dict := make(map[string]string)
	for key, vals := range r.Form {
		dict[key] = vals[0]
	}
	return dict
}

func testStart(w http.ResponseWriter, request *http.Request) {
	testSuite, err := checkSuiteRequest(request, w)
	if err != nil {
		log15.Error("testStart fail", "error", err)
		return
	}
	dict := parseForm(request)
	testID, err := testManager.StartTest(testSuite, dict["name"], dict["description"])
	if err != nil {
		log15.Error("testStart unable to start test case", "name", dict["name"], "error", err)
		msg := fmt.Sprintf("unable to start test case: %s", err.Error())
		http.Error(w, msg, http.StatusInternalServerError)
	}
	log15.Debug("testStart ok", "testId", testID, "name", dict["name"])
	fmt.Fprintf(w, "%s", testID)
}

func suiteStart(w http.ResponseWriter, request *http.Request) {
	log15.Info("Server - suites start request")
	dict := parseForm(request)
	suiteID, err := testManager.StartTestSuite(dict["name"], dict["description"], dict["simlog"])
	if err != nil {
		msg := fmt.Sprintf("unable to start test case: %s", err.Error())
		log15.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
	}
	fmt.Fprintf(w, "%s", suiteID)
}

func suiteEnd(w http.ResponseWriter, request *http.Request) {
	testSuite, err := checkSuiteRequest(request, w)
	if err != nil {
		log15.Error("suiteEnd fail", "error", err)
		return
	}
	err = testManager.EndTestSuite(testSuite)
	if err != nil {
		log15.Error("suiteEnd unable to end suite", "error", err)
		http.Error(w, fmt.Sprintf("unable to end test suite: %s", err.Error()), http.StatusInternalServerError)
	}
	log15.Debug("suiteEnd ok", "testSuite", testSuite)
}

func clientTypesGet(w http.ResponseWriter, request *http.Request) {
	log15.Info("Server - client types request")
	w.Header().Set("Content-Type", "application/json")
	clients := make([]string, 0, len(allClients))
	for client := range allClients {
		clients = append(clients, client)
	}
	json.NewEncoder(w).Encode(clients)
}

func newNode(w http.ResponseWriter, envs map[string]string, files map[string]*multipart.FileHeader, clients map[string]string, r *http.Request, checkliveness bool, useTimeout bool, logdir string) (*common.TestClientInfo, string, bool) {

	//the simulation controller needs to tell us now what client to run for the test
	clientName, in := envs["CLIENT"]
	if !in {
		log15.Error("Missing client type", "error", nil)
		http.Error(w, "Missing client type", http.StatusBadRequest)
		return nil, "", false
	}

	testClientInfo := &common.TestClientInfo{
		ID:              "",
		Name:            clientName,
		VersionInfo:     "",
		InstantiatedAt:  time.Now(),
		LogFile:         "",
		WasInstantiated: false,
	}

	//default the loglevel to the simulator log level setting (different from the system log level setting)
	logLevel := *simloglevelFlag
	logLevelString, in := envs["HIVE_LOGLEVEL"]
	if !in {
		envs["HIVE_LOGLEVEL"] = strconv.Itoa(logLevel)
	} else {
		var err error
		if logLevel, err = strconv.Atoi(logLevelString); err != nil {
			log15.Error("Simulator client HIVE_LOGLEVEL is not an integer", "error", nil)
			http.Error(w, "HIVE_LOGLEVEL not an integer", http.StatusBadRequest)
			return testClientInfo, "", false
		}
	}
	//the simulation host may prevent or be unaware of the simulation controller's requested client
	imageName, in := clients[clientName]
	if !in {
		log15.Error("Unknown or forbidden client type", "clientName", clientName)
		http.Error(w, "Unknown or forbidden client type", http.StatusBadRequest)
		return testClientInfo, "", false
	}
	//create and start the requested client container
	log15.Debug("starting new client", "imagename", imageName, "clientName", clientName)
	container, err := createClientContainer(imageName, envs, files)
	if err != nil {
		log15.Error("failed to create client", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return testClientInfo, "", false
	}
	containerID := container.ID[:8]
	containerIP := ""
	containerMAC := ""
	//and now initialise it with supplied files
	//start a new client logger
	logger := log15.New("id", containerID)
	logfileRelative := filepath.Join(strings.Replace(clientName, string(filepath.Separator), "_", -1), fmt.Sprintf("client-%s.log", containerID))
	logfile := filepath.Join(logdir, logfileRelative)
	// Update the test-suite with the info we have
	testClientInfo.ID = containerID
	testClientInfo.LogFile = logfileRelative
	testClientInfo.WasInstantiated = true
	testClientInfo.InstantiatedAt = time.Now()
	//run the new client
	waiter, err := runContainer(container.ID, logger, logfile, false, logLevel)
	if err != nil {
		logger.Error("failed to start client", "error", err)
		// Clean up the underlying container too
		if removeErr := dockerClient.RemoveContainer(docker.RemoveContainerOptions{ID: containerID, Force: true}); removeErr != nil {
			logger.Error("failed to remove container", "error", removeErr)
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return testClientInfo, containerID, false
	}
	go func() {
		// Ensure the goroutine started by runContainer exits, so that
		// its resources (e.g. the logfile it creates) can be garbage
		// collected.
		err := waiter.Wait()
		if err == nil {
			logger.Debug("client container finished cleanly")
		} else {
			logger.Error("client container finished with error", "error", err)
		}
	}()

	// Wait for the HTTP/RPC socket to open or the container to fail
	start := time.Now()
	checkTime := 100 * time.Millisecond
	for {
		// Update the container state
		container, err = dockerClient.InspectContainer(container.ID)
		if err != nil {
			logger.Error("failed to inspect client", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return testClientInfo, containerID, false
		}
		if !container.State.Running {
			logger.Error("client container terminated")
			http.Error(w, "terminated unexpectedly", http.StatusInternalServerError)
			return testClientInfo, containerID, false
		}

		containerIP = container.NetworkSettings.IPAddress
		containerMAC = container.NetworkSettings.MacAddress
		if checkliveness {
			logger.Debug("Checking container online....", "checktime", checkTime, "state", container.State.String())
			// Container seems to be alive, check whether the RPC is accepting connections
			if conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", containerIP, 8545)); err == nil {
				logger.Debug("client container online", "time", time.Since(start))
				conn.Close()
				break
			}
		} else {
			break
		}

		time.Sleep(checkTime)
		checkTime = checkTime * 2
		if checkTime > 2*time.Second {
			checkTime = time.Second
		}

		if time.Since(container.Created) > timeoutCheckDuration {
			log15.Debug("deleting client container", "name", clientName, "id", containerID)
			err = dockerClient.RemoveContainer(docker.RemoveContainerOptions{ID: containerID, Force: true})
			if err == nil {
				logger.Error("client container terminated due to unresponsive RPC ")
				http.Error(w, "client container terminated due to unresponsive RPC", http.StatusInternalServerError)
			} else {
				logger.Error("failed to terminate client container due to unresponsive RPC")
				http.Error(w, "failed to terminate client container due to unresponsive RPC", http.StatusInternalServerError)
			}
			return testClientInfo, containerID, false

		}
	}
	//  Container online and responsive, return its ID, IP and MAC for later reference
	fmt.Fprintf(w, "%s@%s@%s", containerID, containerIP, containerMAC)
	return testClientInfo, containerID, true
}

// terminates http server, any running tests, and updates the results
// database
func terminateAndUpdate() {
	log15.Debug("terminating simulator server")

	//NB! Kill this first to make sure no further testsuites are started
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if server != nil {
		if err := server.Shutdown(ctx); err != nil {
			log15.Error(fmt.Sprintf("Could not gracefully shutdown the server: %s", err.Error()))
		}
	}
	// Cleanup any tests that might be still running and make sure
	// the test results are updated with the fact that the
	// host terminated for those that were prematurely ended.
	// NB!: this indirectly kills client containers. There is
	// no need for client container timeouts.
	if testManager == nil {
		return
	}
	if err := testManager.Terminate(); err != nil {
		log15.Error("Could not terminate tests: %s\n", err.Error())
	}

}
