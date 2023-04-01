package libhive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/hive/internal/simapi"
	"github.com/gorilla/mux"
	"gopkg.in/inconshreveable/log15.v2"
)

// hiveEnvvarPrefix is the prefix of the environment variables names that should
// be moved from test images to client container to fine tune their setup.
const hiveEnvvarPrefix = "HIVE_"

// This is the default timeout for starting clients.
const defaultStartTimeout = time.Duration(60 * time.Second)

// newSimulationAPI creates handlers for the simulation API.
func newSimulationAPI(b ContainerBackend, env SimEnv, tm *TestManager) http.Handler {
	api := &simAPI{backend: b, env: env, tm: tm}

	// API routes.
	router := mux.NewRouter()
	router.HandleFunc("/clients", api.getClientTypes).Methods("GET")
	router.HandleFunc("/testsuite/{suite}/test/{test}/node/{node}/exec", api.execInClient).Methods("POST")
	router.HandleFunc("/testsuite/{suite}/test/{test}/node/{node}", api.getNodeStatus).Methods("GET")
	router.HandleFunc("/testsuite/{suite}/test/{test}/node", api.startClient).Methods("POST")
	router.HandleFunc("/testsuite/{suite}/test/{test}/node/{node}", api.stopClient).Methods("DELETE")
	router.HandleFunc("/testsuite/{suite}/test/{test}/node/{node}/pause", api.pauseClient).Methods("POST")
	router.HandleFunc("/testsuite/{suite}/test/{test}/node/{node}/pause", api.unpauseClient).Methods("DELETE")
	router.HandleFunc("/testsuite/{suite}/test", api.startTest).Methods("POST")
	// post because the delete http verb does not always support a message body
	router.HandleFunc("/testsuite/{suite}/test/{test}", api.endTest).Methods("POST")
	router.HandleFunc("/testsuite", api.startSuite).Methods("POST")
	router.HandleFunc("/testsuite/{suite}", api.endSuite).Methods("DELETE")
	router.HandleFunc("/testsuite/{suite}/network/{network}", api.networkCreate).Methods("POST")
	router.HandleFunc("/testsuite/{suite}/network/{network}", api.networkRemove).Methods("DELETE")
	router.HandleFunc("/testsuite/{suite}/network/{network}/{node}", api.networkIPGet).Methods("GET")
	router.HandleFunc("/testsuite/{suite}/network/{network}/{node}", api.networkConnect).Methods("POST")
	router.HandleFunc("/testsuite/{suite}/network/{network}/{node}", api.networkDisconnect).Methods("DELETE")
	return router
}

type simAPI struct {
	backend ContainerBackend
	env     SimEnv
	tm      *TestManager
}

// getClientTypes returns all known client types.
func (api *simAPI) getClientTypes(w http.ResponseWriter, r *http.Request) {
	clients := make([]*ClientDefinition, 0, len(api.tm.clientDefs))
	for _, def := range api.tm.clientDefs {
		clients = append(clients, def)
	}
	sort.Slice(clients, func(i, j int) bool {
		return clients[i].Name < clients[j].Name
	})
	serveJSON(w, clients)
}

// startSuite starts a suite.
func (api *simAPI) startSuite(w http.ResponseWriter, r *http.Request) {
	var suite simapi.TestRequest
	if err := json.NewDecoder(r.Body).Decode(&suite); err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}
	if suite.Name == "" {
		serveError(w, errors.New("suite name is empty"), http.StatusBadRequest)
		return
	}

	suiteID, err := api.tm.StartTestSuite(suite.Name, suite.Description)
	if err != nil {
		log15.Error("API: StartTestSuite failed", "error", err)
		serveError(w, err, http.StatusInternalServerError)
	}
	log15.Info("API: suite started", "suite", suiteID, "name", suite.Name)
	serveJSON(w, suiteID)
}

// endSuite ends a suite.
func (api *simAPI) endSuite(w http.ResponseWriter, r *http.Request) {
	suiteID, err := api.requestSuite(r)
	if err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}
	if err := api.tm.EndTestSuite(suiteID); err != nil {
		log15.Error("API: EndTestSuite failed", "suite", suiteID, "error", err)
		serveError(w, err, http.StatusInternalServerError)
		return
	}
	log15.Info("API: suite ended", "suite", suiteID)
	serveOK(w)
}

// startTest signals the start of a test case.
func (api *simAPI) startTest(w http.ResponseWriter, r *http.Request) {
	suiteID, err := api.requestSuite(r)
	if err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}
	var test simapi.TestRequest
	if err := json.NewDecoder(r.Body).Decode(&test); err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}
	if test.Name == "" {
		serveError(w, errors.New("test name is empty"), http.StatusBadRequest)
		return
	}

	testID, err := api.tm.StartTest(suiteID, test.Name, test.Description)
	if err != nil {
		err := fmt.Errorf("can't start test case: %s", err.Error())
		serveError(w, err, http.StatusInternalServerError)
		return
	}
	log15.Info("API: test started", "suite", suiteID, "test", testID, "name", test.Name)
	serveJSON(w, testID)
}

// endTest signals the end of a test case. It also shuts down all clients
// associated with the test.
func (api *simAPI) endTest(w http.ResponseWriter, r *http.Request) {
	suiteID, testID, err := api.requestSuiteAndTest(r)
	if err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}

	var result TestResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		log15.Error("API: invalid result data in endTest", "suite", suiteID, "test", testID, "error", err)
		err := fmt.Errorf("can't unmarshal result: %v", err)
		serveError(w, err, http.StatusBadRequest)
		return
	}

	err = api.tm.EndTest(suiteID, testID, &result)
	if err != nil {
		log15.Error("API: EndTest failed", "suite", suiteID, "test", testID, "error", err)
		err := fmt.Errorf("can't end test case: %v", err)
		serveError(w, err, http.StatusInternalServerError)
		return
	}

	log15.Info("API: test ended", "suite", suiteID, "test", testID, "pass", result.Pass)
	serveOK(w)
}

// startClient starts a client container.
func (api *simAPI) startClient(w http.ResponseWriter, r *http.Request) {
	suiteID, testID, err := api.requestSuiteAndTest(r)
	if err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}

	// Client launch parameters are given as multipart/form-data.
	const maxMemory = 8 * 1024 * 1024
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		log15.Error("API: could not parse node request", "error", err)
		err := fmt.Errorf("could not parse node request")
		serveError(w, err, http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	if !r.Form.Has("config") {
		log15.Error("API: missing 'config' parameter in node request", "error", err)
		err := fmt.Errorf("missing 'config' parameter in node request")
		serveError(w, err, http.StatusBadRequest)
		return
	}
	var clientConfig simapi.NodeConfig
	if err := json.Unmarshal([]byte(r.Form.Get("config")), &clientConfig); err != nil {
		log15.Error("API: invalid 'config' parameter in node request", "error", err)
		err := fmt.Errorf("invalid 'config' parameter in node request")
		serveError(w, err, http.StatusBadRequest)
		return
	}

	// Get the client name.
	clientDef, err := api.checkClient(&clientConfig)
	if err != nil {
		log15.Error("API: " + err.Error())
		serveError(w, err, http.StatusBadRequest)
		return
	}
	// Get the network names, if any, for the container to be connected to at start.
	networks, err := api.checkClientNetworks(&clientConfig, suiteID)
	if err != nil {
		log15.Error("API: "+err.Error(), "client", clientDef.Name)
		serveError(w, err, http.StatusBadRequest)
		return
	}

	files := make(map[string]*multipart.FileHeader)
	for key, fheaders := range r.MultipartForm.File {
		if len(fheaders) > 0 {
			// Note: the PARAMETER NAME (not the 'filename') is used as the destination
			// file path in the container. This is because RFC 7578 says that directory
			// components should be ignored in the filename supplied by the form, and
			// package multipart strips the directory info away at parse time.
			files[key] = fheaders[0]
		}
	}

	// Sanitize environment.
	env := clientConfig.Environment
	for k := range env {
		if !strings.HasPrefix(k, hiveEnvvarPrefix) {
			delete(env, k)
		}
	}
	// Set default client loglevel to sim loglevel.
	if env == nil {
		env = make(map[string]string)
	}

	if env["HIVE_LOGLEVEL"] == "" {
		env["HIVE_LOGLEVEL"] = strconv.Itoa(api.env.SimLogLevel)
	}

	// Set up the timeout.
	timeout := api.env.ClientStartTimeout
	if timeout == 0 {
		timeout = defaultStartTimeout
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// Create the client container.
	options := ContainerOptions{Env: env, Files: files}
	containerID, err := api.backend.CreateContainer(ctx, clientDef.Image, options)
	if err != nil {
		log15.Error("API: client container create failed", "client", clientDef.Name, "error", err)
		err := fmt.Errorf("client container create failed (%v)", err)
		serveError(w, err, http.StatusInternalServerError)
		return
	}

	// Set the log file. We need the container ID for this,
	// so it can only be set after creating the container.
	logPath, logFilePath := api.clientLogFilePaths(clientDef.Name, containerID)
	options.LogFile = logFilePath

	// Connect to the networks if requested, so it is started already joined to each one.
	for _, network := range networks {
		if err := api.tm.ConnectContainer(suiteID, network, containerID); err != nil {
			log15.Error("API: failed to connect container", "network", network, "container", containerID, "error", err)
			serveError(w, err, http.StatusInternalServerError)
			return
		}
	}

	// by default: check the eth1 port
	options.CheckLive = 8545
	if portStr := env["HIVE_CHECK_LIVE_PORT"]; portStr != "" {
		v, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			log15.Error("API: could not parse check-live port", "error", err)
			serveError(w, err, http.StatusBadRequest)
			return
		}
		options.CheckLive = uint16(v)
	}

	// Start it!
	info, err := api.backend.StartContainer(ctx, containerID, options)
	if info != nil {
		clientInfo := &ClientInfo{
			ID:             info.ID,
			IP:             info.IP,
			Name:           clientDef.Name,
			InstantiatedAt: time.Now(),
			LogFile:        logPath,
			wait:           info.Wait,
		}

		// Add client version to the test suite.
		api.tm.testSuiteMutex.Lock()
		if suite, ok := api.tm.runningTestSuites[suiteID]; ok {
			suite.ClientVersions[clientDef.Name] = clientDef.Version
		}
		api.tm.testSuiteMutex.Unlock()

		// Register the node. This should always be done, even if starting the container
		// failed, to ensure that the failed client log is associated with the test.
		api.tm.RegisterNode(testID, info.ID, clientInfo)
	}
	if err != nil {
		log15.Error("API: could not start client", "client", clientDef.Name, "container", containerID[:8], "error", err)
		err := fmt.Errorf("client did not start: %v", err)
		serveError(w, err, http.StatusInternalServerError)
		return
	}

	// It's started.
	log15.Info("API: client "+clientDef.Name+" started", "suite", suiteID, "test", testID, "container", containerID[:8])
	serveJSON(w, &simapi.StartNodeResponse{ID: info.ID, IP: info.IP})
}

// clientLogFilePaths determines the log file path of a client container.
// Note that jsonPath gets written to the result JSON and always uses '/' as the separator.
// The filePath is passed to the docker backend and uses the platform separator.
func (api *simAPI) clientLogFilePaths(clientName, containerID string) (jsonPath string, file string) {
	// TODO: might be nice to put timestamp into the filename as well.
	safeDir := strings.Replace(clientName, string(filepath.Separator), "_", -1)
	jsonPath = path.Join(safeDir, fmt.Sprintf("client-%s.log", containerID))
	file = filepath.Join(api.env.LogDir, filepath.FromSlash(jsonPath))
	return jsonPath, file
}

func (api *simAPI) checkClient(req *simapi.NodeConfig) (*ClientDefinition, error) {
	if req.Client == "" {
		return nil, errors.New("missing client type in start request")
	}
	def, ok := api.tm.clientDefs[req.Client]
	if !ok {
		return nil, errors.New("unknown client type in start request")
	}
	return def, nil
}

// checkClientNetworks pre-checks the existence of initial networks for a client container.
func (api *simAPI) checkClientNetworks(req *simapi.NodeConfig, suiteID TestSuiteID) ([]string, error) {
	for _, network := range req.Networks {
		if !api.tm.NetworkExists(suiteID, network) {
			return nil, fmt.Errorf("invalid network name '%s' in client start request", network)
		}
	}
	return req.Networks, nil
}

// stopClient terminates a client container.
func (api *simAPI) stopClient(w http.ResponseWriter, r *http.Request) {
	_, testID, err := api.requestSuiteAndTest(r)
	if err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}
	node := mux.Vars(r)["node"]

	err = api.tm.StopNode(testID, node)
	switch {
	case err == ErrNoSuchNode:
		serveError(w, err, http.StatusNotFound)
	case err != nil:
		serveError(w, err, http.StatusInternalServerError)
	default:
		serveOK(w)
	}
}

// pauseClient pauses a client container.
func (api *simAPI) pauseClient(w http.ResponseWriter, r *http.Request) {
	_, testID, err := api.requestSuiteAndTest(r)
	if err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}
	node := mux.Vars(r)["node"]

	err = api.tm.PauseNode(testID, node)
	switch {
	case err == ErrNoSuchNode:
		serveError(w, err, http.StatusNotFound)
	case err != nil:
		serveError(w, err, http.StatusInternalServerError)
	default:
		serveOK(w)
	}
}

// unpauseClient unpauses a client container.
func (api *simAPI) unpauseClient(w http.ResponseWriter, r *http.Request) {
	_, testID, err := api.requestSuiteAndTest(r)
	if err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}
	node := mux.Vars(r)["node"]

	err = api.tm.UnpauseNode(testID, node)
	switch {
	case err == ErrNoSuchNode:
		serveError(w, err, http.StatusNotFound)
	case err != nil:
		serveError(w, err, http.StatusInternalServerError)
	default:
		serveOK(w)
	}
}

// getNodeStatus returns the status of a client container.
func (api *simAPI) getNodeStatus(w http.ResponseWriter, r *http.Request) {
	suiteID, testID, err := api.requestSuiteAndTest(r)
	if err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}

	node := mux.Vars(r)["node"]
	nodeInfo, err := api.tm.GetNodeInfo(suiteID, testID, node)
	if err != nil {
		log15.Error("API: can't find node", "node", node, "error", err)
		serveError(w, err, http.StatusNotFound)
		return
	}

	serveJSON(w, &simapi.NodeResponse{ID: nodeInfo.ID, Name: nodeInfo.Name})
}

func (api *simAPI) execInClient(w http.ResponseWriter, r *http.Request) {
	suiteID, testID, err := api.requestSuiteAndTest(r)
	if err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}

	node := mux.Vars(r)["node"]
	nodeInfo, err := api.tm.GetNodeInfo(suiteID, testID, node)
	if err != nil {
		log15.Error("API: can't find node", "node", node, "error", err)
		serveError(w, err, http.StatusNotFound)
		return
	}

	// Parse and validate the exec request.
	commandline, err := parseExecRequest(r.Body)
	if err != nil {
		log15.Error("API: invalid exec request", "node", node, "error", err)
		serveError(w, err, http.StatusBadRequest)
		return
	}
	info, err := api.backend.RunProgram(r.Context(), nodeInfo.ID, commandline)
	if err != nil {
		log15.Error("API: client script exec error", "node", node, "error", err)
		serveError(w, err, http.StatusInternalServerError)
		return
	}
	serveJSON(w, &info)
}

// parseExecRequest decodes and validates a client script exec request.
func parseExecRequest(r io.Reader) ([]string, error) {
	var request simapi.ExecRequest
	if err := json.NewDecoder(r).Decode(&request); err != nil {
		return nil, fmt.Errorf("invalid JSON: %v", err)
	}
	if len(request.Command) == 0 {
		return nil, errors.New("empty command")
	}
	script := request.Command[0]
	if strings.Contains(script, "/") {
		return nil, errors.New("script name must not contain directory separator")
	}
	request.Command[0] = "/hive-bin/" + script
	return request.Command, nil
}

// networkCreate creates a docker network.
func (api *simAPI) networkCreate(w http.ResponseWriter, r *http.Request) {
	suiteID, err := api.requestSuite(r)
	if err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}

	networkName := mux.Vars(r)["network"]
	err = api.tm.CreateNetwork(suiteID, networkName)
	if err != nil {
		log15.Error("API: failed to create network", "network", networkName, "error", err)
		serveError(w, err, http.StatusBadRequest)
		return
	}
	log15.Info("API: network created", "name", networkName)
	serveOK(w)
}

// networkRemove removes a docker network.
func (api *simAPI) networkRemove(w http.ResponseWriter, r *http.Request) {
	suiteID, err := api.requestSuite(r)
	if err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}

	network := mux.Vars(r)["network"]
	err = api.tm.RemoveNetwork(suiteID, network)
	if err != nil {
		log15.Error("API: failed to remove network", "network", network, "error", err)
		serveError(w, err, http.StatusInternalServerError)
		return
	}
	log15.Info("API: docker network removed", "network", network)
	serveOK(w)
}

// networkIPGet gets the IP address of a container on a network.
func (api *simAPI) networkIPGet(w http.ResponseWriter, r *http.Request) {
	suiteID, err := api.requestSuite(r)
	if err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}

	node := mux.Vars(r)["node"]
	network := mux.Vars(r)["network"]
	ipAddr, err := api.tm.ContainerIP(suiteID, network, node)
	if err != nil {
		log15.Error("API: failed to get container IP", "container", node, "error", err)
		serveError(w, err, http.StatusInternalServerError)
		return
	}
	log15.Info("API: container IP requested", "network", network, "container", node, "ip", ipAddr)
	serveJSON(w, ipAddr)
}

// networkConnect connects a container to a network.
func (api *simAPI) networkConnect(w http.ResponseWriter, r *http.Request) {
	suiteID, err := api.requestSuite(r)
	if err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}

	name := mux.Vars(r)["network"]
	containerID := mux.Vars(r)["node"]
	if err := api.tm.ConnectContainer(suiteID, name, containerID); err != nil {
		log15.Error("API: failed to connect container", "network", name, "container", containerID, "error", err)
		serveError(w, err, http.StatusInternalServerError)
		return
	}
	log15.Info("API: container connected to network", "network", name, "container", containerID)
	serveOK(w)
}

// networkDisconnect disconnects a container from a network.
func (api *simAPI) networkDisconnect(w http.ResponseWriter, r *http.Request) {
	suiteID, err := api.requestSuite(r)
	if err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}

	network := mux.Vars(r)["network"]
	containerID := mux.Vars(r)["node"]
	if err := api.tm.DisconnectContainer(suiteID, network, containerID); err != nil {
		log15.Error("API: disconnecting container failed", "network", network, "container", containerID, "error", err)
		serveError(w, err, http.StatusInternalServerError)
		return
	}
	log15.Info("API: container disconnected", "network", network, "container", containerID)
	serveOK(w)
}

// requestSuite returns the suite ID from the request body and checks that
// it corresponds to a running suite.
func (api *simAPI) requestSuite(r *http.Request) (TestSuiteID, error) {
	suite := mux.Vars(r)["suite"]

	testSuite, err := strconv.Atoi(suite)
	if err != nil {
		return 0, fmt.Errorf("invalid test suite %q", suite)
	}
	testSuiteID := TestSuiteID(testSuite)
	if _, running := api.tm.IsTestSuiteRunning(testSuiteID); !running {
		return 0, fmt.Errorf("test suite %d not running", testSuite)
	}
	return testSuiteID, nil
}

// requestTest returns the test ID from the request body and checks that it
// corresponds to a running test.
func (api *simAPI) requestTest(r *http.Request) (TestID, error) {
	testString := mux.Vars(r)["test"]

	testCase, err := strconv.Atoi(testString)
	if err != nil {
		return 0, fmt.Errorf("invalid test case id %q", testString)
	}
	testCaseID := TestID(testCase)
	if _, running := api.tm.IsTestRunning(testCaseID); !running {
		return 0, fmt.Errorf("test case %d is not running", testCaseID)
	}
	return testCaseID, nil
}

// requestSuiteAndTest returns the suite ID and test ID from the request body.
func (api *simAPI) requestSuiteAndTest(r *http.Request) (TestSuiteID, TestID, error) {
	suiteID, err := api.requestSuite(r)
	if err != nil {
		return 0, 0, err
	}
	testID, err := api.requestTest(r)
	return suiteID, testID, err
}

func serveJSON(w http.ResponseWriter, value interface{}) {
	resp, err := json.Marshal(value)
	if err != nil {
		log15.Error("API: internal error while encoding response", "error", err)
		serveError(w, errors.New("internal error"), http.StatusInternalServerError)
		return
	}
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

func serveOK(w http.ResponseWriter) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "null")
}

func serveError(w http.ResponseWriter, err error, status int) {
	resp, _ := json.Marshal(&simapi.Error{Error: err.Error()})
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	w.Write(resp)
}
