package libhive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/hive/internal/simapi"
	"github.com/gorilla/mux"
)

const (
	// hiveEnvvarPrefix is the prefix of the environment variables names that should
	// be moved from test images to client container to fine tune their setup.
	hiveEnvvarPrefix = "HIVE_"

	// defaultStartTimeout is the default timeout for starting clients.
	defaultStartTimeout = 60 * time.Second

	// maxMultipartMemory limits the memory used for parsing multipart forms
	maxMultipartMemory = 8 * 1024 * 1024

	// defaultCheckLivePort is the default port to check for client liveness
	defaultCheckLivePort = 8545
)

// newSimulationAPI creates handlers for the simulation API.
func newSimulationAPI(b ContainerBackend, env SimEnv, tm *TestManager, hive HiveInfo) http.Handler {
	api := &simAPI{
		backend: b,
		env:     env,
		tm:      tm,
		hive:    hive,
	}

	router := mux.NewRouter()
	api.registerRoutes(router)
	return router
}

type simAPI struct {
	backend ContainerBackend
	env     SimEnv
	tm      *TestManager
	hive    HiveInfo
}

// registerRoutes sets up all API routes
func (api *simAPI) registerRoutes(router *mux.Router) {
	// Info endpoints
	router.HandleFunc("/hive", api.getHiveInfo).Methods("GET")
	router.HandleFunc("/clients", api.getClientTypes).Methods("GET")

	// Test suite lifecycle
	router.HandleFunc("/testsuite", api.startSuite).Methods("POST")
	router.HandleFunc("/testsuite/{suite}", api.endSuite).Methods("DELETE")

	// Test case lifecycle
	router.HandleFunc("/testsuite/{suite}/test", api.startTest).Methods("POST")
	router.HandleFunc("/testsuite/{suite}/test/{test}", api.endTest).Methods("POST")

	// Node management
	router.HandleFunc("/testsuite/{suite}/test/{test}/node", api.startClient).Methods("POST")
	router.HandleFunc("/testsuite/{suite}/test/{test}/node/{node}", api.stopClient).Methods("DELETE")
	router.HandleFunc("/testsuite/{suite}/test/{test}/node/{node}", api.getNodeStatus).Methods("GET")
	router.HandleFunc("/testsuite/{suite}/test/{test}/node/{node}/pause", api.pauseClient).Methods("POST")
	router.HandleFunc("/testsuite/{suite}/test/{test}/node/{node}/pause", api.unpauseClient).Methods("DELETE")
	router.HandleFunc("/testsuite/{suite}/test/{test}/node/{node}/exec", api.execInClient).Methods("POST")

	// Network management
	router.HandleFunc("/testsuite/{suite}/network/{network}", api.networkCreate).Methods("POST")
	router.HandleFunc("/testsuite/{suite}/network/{network}", api.networkRemove).Methods("DELETE")
	router.HandleFunc("/testsuite/{suite}/network/{network}/{node}", api.networkIPGet).Methods("GET")
	router.HandleFunc("/testsuite/{suite}/network/{network}/{node}", api.networkConnect).Methods("POST")
	router.HandleFunc("/testsuite/{suite}/network/{network}/{node}", api.networkDisconnect).Methods("DELETE")
}

// getHiveInfo returns information about the hive server instance.
func (api *simAPI) getHiveInfo(w http.ResponseWriter, r *http.Request) {
	slog.Info("API: hive info requested")
	serveJSON(w, api.hive)
}

// getClientTypes returns all known client types.
func (api *simAPI) getClientTypes(w http.ResponseWriter, r *http.Request) {
	serveJSON(w, api.tm.clientDefs)
}

// startSuite starts a test suite.
func (api *simAPI) startSuite(w http.ResponseWriter, r *http.Request) {
	var suite simapi.TestRequest
	if err := json.NewDecoder(r.Body).Decode(&suite); err != nil {
		serveError(w, fmt.Errorf("invalid JSON in request body: %w", err), http.StatusBadRequest)
		return
	}

	if suite.Name == "" {
		serveError(w, errors.New("suite name is empty"), http.StatusBadRequest)
		return
	}

	suiteID, err := api.tm.StartTestSuite(suite.Name, suite.Description)
	if err != nil {
		slog.Error("API: StartTestSuite failed", "error", err)
		serveError(w, fmt.Errorf("failed to start test suite: %w", err), http.StatusInternalServerError)
		return
	}

	slog.Info("API: suite started", "suite", suiteID, "name", suite.Name)
	serveJSON(w, suiteID)
}

// endSuite ends a test suite.
func (api *simAPI) endSuite(w http.ResponseWriter, r *http.Request) {
	suiteID, err := api.requestSuite(r)
	if err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}

	if err := api.tm.EndTestSuite(suiteID); err != nil {
		slog.Error("API: EndTestSuite failed", "suite", suiteID, "error", err)
		serveError(w, fmt.Errorf("failed to end test suite: %w", err), http.StatusInternalServerError)
		return
	}

	slog.Info("API: suite ended", "suite", suiteID)
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
		serveError(w, fmt.Errorf("invalid JSON in request body: %w", err), http.StatusBadRequest)
		return
	}

	if test.Name == "" {
		serveError(w, errors.New("test name is empty"), http.StatusBadRequest)
		return
	}

	testID, err := api.tm.StartTest(suiteID, test.Name, test.Description)
	if err != nil {
		serveError(w, fmt.Errorf("can't start test case: %w", err), http.StatusInternalServerError)
		return
	}

	slog.Info("API: test started", "suite", suiteID, "test", testID, "name", test.Name)
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
		slog.Error("API: invalid result data in endTest", "suite", suiteID, "test", testID, "error", err)
		serveError(w, fmt.Errorf("can't unmarshal result: %w", err), http.StatusBadRequest)
		return
	}

	if err := api.tm.EndTest(suiteID, testID, &result); err != nil {
		slog.Error("API: EndTest failed", "suite", suiteID, "test", testID, "error", err)
		serveError(w, fmt.Errorf("can't end test case: %w", err), http.StatusInternalServerError)
		return
	}

	slog.Info("API: test ended", "suite", suiteID, "test", testID, "pass", result.Pass)
	serveOK(w)
}

// startClient starts a client container.
func (api *simAPI) startClient(w http.ResponseWriter, r *http.Request) {
	suiteID, testID, err := api.requestSuiteAndTest(r)
	if err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}

	clientConfig, files, err := api.parseClientRequest(r)
	if err != nil {
		slog.Error("API: failed to parse client request", "error", err)
		serveError(w, err, http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	clientDef, err := api.validateClient(clientConfig)
	if err != nil {
		slog.Error("API: client validation failed", "error", err)
		serveError(w, err, http.StatusBadRequest)
		return
	}

	networks, err := api.validateClientNetworks(clientConfig, suiteID)
	if err != nil {
		slog.Error("API: network validation failed", "client", clientDef.Name, "error", err)
		serveError(w, err, http.StatusBadRequest)
		return
	}

	env := api.prepareClientEnvironment(clientConfig)
	timeout := api.getClientStartTimeout()

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	containerID, clientInfo, err := api.createAndStartClient(ctx, clientDef, env, files, networks, suiteID, testID)
	if err != nil {
		slog.Error("API: failed to start client", "client", clientDef.Name, "error", err)
		serveError(w, err, http.StatusInternalServerError)
		return
	}

	slog.Info("API: client started", "client", clientDef.Name, "suite", suiteID, "test", testID, "container", containerID[:8])
	serveJSON(w, &simapi.StartNodeResponse{ID: clientInfo.ID, IP: clientInfo.IP})
}

// parseClientRequest parses the multipart form data for client configuration
func (api *simAPI) parseClientRequest(r *http.Request) (*simapi.NodeConfig, map[string]*multipart.FileHeader, error) {
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		return nil, nil, fmt.Errorf("could not parse multipart form: %w", err)
	}

	if !r.Form.Has("config") {
		return nil, nil, errors.New("missing 'config' parameter in node request")
	}

	var clientConfig simapi.NodeConfig
	if err := json.Unmarshal([]byte(r.Form.Get("config")), &clientConfig); err != nil {
		return nil, nil, fmt.Errorf("invalid 'config' parameter: %w", err)
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

	return &clientConfig, files, nil
}

// validateClient checks if the requested client type is available
func (api *simAPI) validateClient(config *simapi.NodeConfig) (*ClientDefinition, error) {
	if config.Client == "" {
		return nil, errors.New("missing client type in start request")
	}

	for _, client := range api.tm.clientDefs {
		if client.Name == config.Client {
			return client, nil
		}
	}

	return nil, fmt.Errorf("unknown client type '%s' in start request", config.Client)
}

// validateClientNetworks pre-checks the existence of initial networks for a client container.
func (api *simAPI) validateClientNetworks(config *simapi.NodeConfig, suiteID TestSuiteID) ([]string, error) {
	for _, network := range config.Networks {
		if !api.tm.NetworkExists(suiteID, network) {
			return nil, fmt.Errorf("invalid network name '%s' in client start request", network)
		}
	}
	return config.Networks, nil
}

// prepareClientEnvironment sanitizes and prepares the environment variables
func (api *simAPI) prepareClientEnvironment(config *simapi.NodeConfig) map[string]string {
	env := make(map[string]string)

	// Copy only allowed environment variables
	for k, v := range config.Environment {
		if strings.HasPrefix(k, hiveEnvvarPrefix) {
			env[k] = v
		}
	}

	// Set default client loglevel to sim loglevel if not specified
	if env["HIVE_LOGLEVEL"] == "" {
		env["HIVE_LOGLEVEL"] = strconv.Itoa(api.env.SimLogLevel)
	}

	return env
}

// getClientStartTimeout returns the configured timeout or default
func (api *simAPI) getClientStartTimeout() time.Duration {
	if api.env.ClientStartTimeout == 0 {
		return defaultStartTimeout
	}
	return api.env.ClientStartTimeout
}

// createAndStartClient handles the container creation and startup process
func (api *simAPI) createAndStartClient(ctx context.Context, clientDef *ClientDefinition, env map[string]string, files map[string]*multipart.FileHeader, networks []string, suiteID TestSuiteID, testID TestID) (string, *ClientInfo, error) {
	// Create labels for client container
	labels := NewBaseLabels(api.tm.hiveInstanceID, api.tm.hiveVersion)
	labels[LabelHiveType] = ContainerTypeClient
	labels[LabelHiveTestSuite] = suiteID.String()
	labels[LabelHiveTestCase] = testID.String()
	labels[LabelHiveClientName] = clientDef.Name
	labels[LabelHiveClientImage] = clientDef.Image

	containerName := GenerateClientContainerName(clientDef.Name, suiteID, testID)

	// Create the client container
	options := ContainerOptions{
		Env:    env,
		Files:  files,
		Labels: labels,
		Name:   containerName,
	}

	containerID, err := api.backend.CreateContainer(ctx, clientDef.Image, options)
	if err != nil {
		return "", nil, fmt.Errorf("client container create failed: %w", err)
	}

	// Set up logging
	logPath, logFilePath := api.clientLogFilePaths(clientDef.Name, containerID)
	options.LogFile = logFilePath

	// Connect to networks before starting
	for _, network := range networks {
		if err := api.tm.ConnectContainer(suiteID, network, containerID); err != nil {
			return "", nil, fmt.Errorf("failed to connect container to network %s: %w", network, err)
		}
	}

	// Set up liveness check port
	options.CheckLive = defaultCheckLivePort
	if portStr := env["HIVE_CHECK_LIVE_PORT"]; portStr != "" {
		if port, err := strconv.ParseUint(portStr, 10, 16); err != nil {
			return "", nil, fmt.Errorf("invalid check-live port: %w", err)
		} else {
			options.CheckLive = uint16(port)
		}
	}

	// Start the container
	info, err := api.backend.StartContainer(ctx, containerID, options)
	if err != nil {
		return containerID, nil, fmt.Errorf("client did not start: %w", err)
	}

	clientInfo := &ClientInfo{
		ID:             info.ID,
		IP:             info.IP,
		Name:           clientDef.Name,
		InstantiatedAt: time.Now(),
		LogFile:        logPath,
		wait:           info.Wait,
	}

	// Add client version to the test suite
	api.tm.testSuiteMutex.Lock()
	if suite, ok := api.tm.runningTestSuites[suiteID]; ok {
		suite.ClientVersions[clientDef.Name] = clientDef.Version
	}
	api.tm.testSuiteMutex.Unlock()

	// Register the node
	api.tm.RegisterNode(testID, info.ID, clientInfo)

	return containerID, clientInfo, nil
}

// clientLogFilePaths determines the log file path of a client container.
// Note that jsonPath gets written to the result JSON and always uses '/' as the separator.
// The filePath is passed to the docker backend and uses the platform separator.
func (api *simAPI) clientLogFilePaths(clientName, containerID string) (jsonPath, filePath string) {
	// TODO: might be nice to put timestamp into the filename as well.
	safeDir := strings.ReplaceAll(clientName, string(filepath.Separator), "_")
	jsonPath = path.Join(safeDir, fmt.Sprintf("client-%s.log", containerID))
	filePath = filepath.Join(api.env.LogDir, filepath.FromSlash(jsonPath))
	return jsonPath, filePath
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
	case errors.Is(err, ErrNoSuchNode):
		serveError(w, err, http.StatusNotFound)
	case err != nil:
		serveError(w, err, http.StatusInternalServerError)
	default:
		serveOK(w)
	}
}

// pauseClient pauses a client container.
func (api *simAPI) pauseClient(w http.ResponseWriter, r *http.Request) {
	api.handleNodeOperation(w, r, api.tm.PauseNode, "pause")
}

// unpauseClient unpauses a client container.
func (api *simAPI) unpauseClient(w http.ResponseWriter, r *http.Request) {
	api.handleNodeOperation(w, r, api.tm.UnpauseNode, "unpause")
}

// handleNodeOperation is a helper for node operations (pause/unpause)
func (api *simAPI) handleNodeOperation(w http.ResponseWriter, r *http.Request, operation func(TestID, string) error, operationName string) {
	_, testID, err := api.requestSuiteAndTest(r)
	if err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}

	node := mux.Vars(r)["node"]
	err = operation(testID, node)

	switch {
	case errors.Is(err, ErrNoSuchNode):
		serveError(w, err, http.StatusNotFound)
	case err != nil:
		slog.Error("API: node operation failed", "operation", operationName, "node", node, "error", err)
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
		slog.Error("API: can't find node", "node", node, "error", err)
		serveError(w, err, http.StatusNotFound)
		return
	}

	serveJSON(w, &simapi.NodeResponse{ID: nodeInfo.ID, Name: nodeInfo.Name})
}

// execInClient executes a command in a client container.
func (api *simAPI) execInClient(w http.ResponseWriter, r *http.Request) {
	suiteID, testID, err := api.requestSuiteAndTest(r)
	if err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}

	node := mux.Vars(r)["node"]
	nodeInfo, err := api.tm.GetNodeInfo(suiteID, testID, node)
	if err != nil {
		slog.Error("API: can't find node", "node", node, "error", err)
		serveError(w, err, http.StatusNotFound)
		return
	}

	commandline, err := parseExecRequest(r.Body)
	if err != nil {
		slog.Error("API: invalid exec request", "node", node, "error", err)
		serveError(w, err, http.StatusBadRequest)
		return
	}

	info, err := api.backend.RunProgram(r.Context(), nodeInfo.ID, commandline)
	if err != nil {
		slog.Error("API: client script exec error", "node", node, "error", err)
		serveError(w, err, http.StatusInternalServerError)
		return
	}

	serveJSON(w, &info)
}

// parseExecRequest decodes and validates a client script exec request.
func parseExecRequest(reader io.Reader) ([]string, error) {
	var request simapi.ExecRequest
	if err := json.NewDecoder(reader).Decode(&request); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
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

// Network management handlers

// networkCreate creates a docker network.
func (api *simAPI) networkCreate(w http.ResponseWriter, r *http.Request) {
	suiteID, err := api.requestSuite(r)
	if err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}

	networkName := mux.Vars(r)["network"]
	if err := api.tm.CreateNetwork(suiteID, networkName); err != nil {
		slog.Error("API: failed to create network", "network", networkName, "error", err)
		serveError(w, fmt.Errorf("failed to create network: %w", err), http.StatusBadRequest)
		return
	}

	slog.Info("API: network created", "name", networkName)
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
	if err := api.tm.RemoveNetwork(suiteID, network); err != nil {
		slog.Error("API: failed to remove network", "network", network, "error", err)
		serveError(w, fmt.Errorf("failed to remove network: %w", err), http.StatusInternalServerError)
		return
	}

	slog.Info("API: docker network removed", "network", network)
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
		slog.Error("API: failed to get container IP", "container", node, "error", err)
		serveError(w, fmt.Errorf("failed to get container IP: %w", err), http.StatusInternalServerError)
		return
	}

	slog.Info("API: container IP requested", "network", network, "container", node, "ip", ipAddr)
	serveJSON(w, ipAddr)
}

// networkConnect connects a container to a network.
func (api *simAPI) networkConnect(w http.ResponseWriter, r *http.Request) {
	api.handleNetworkOperation(w, r, api.tm.ConnectContainer, "connect")
}

// networkDisconnect disconnects a container from a network.
func (api *simAPI) networkDisconnect(w http.ResponseWriter, r *http.Request) {
	api.handleNetworkOperation(w, r, api.tm.DisconnectContainer, "disconnect")
}

// handleNetworkOperation is a helper for network connect/disconnect operations
func (api *simAPI) handleNetworkOperation(w http.ResponseWriter, r *http.Request, operation func(TestSuiteID, string, string) error, operationName string) {
	suiteID, err := api.requestSuite(r)
	if err != nil {
		serveError(w, err, http.StatusBadRequest)
		return
	}

	network := mux.Vars(r)["network"]
	containerID := mux.Vars(r)["node"]

	if err := operation(suiteID, network, containerID); err != nil {
		slog.Error("API: network operation failed", "operation", operationName, "network", network, "container", containerID, "error", err)
		serveError(w, fmt.Errorf("failed to %s container: %w", operationName, err), http.StatusInternalServerError)
		return
	}

	slog.Info("API: container network operation completed", "operation", operationName, "network", network, "container", containerID)
	serveOK(w)
}

// Helper methods for request parsing

// requestSuite returns the suite ID from the request body and checks that
// it corresponds to a running suite.
func (api *simAPI) requestSuite(r *http.Request) (TestSuiteID, error) {
	suite := mux.Vars(r)["suite"]

	testSuite, err := strconv.Atoi(suite)
	if err != nil {
		return 0, fmt.Errorf("invalid test suite %q: %w", suite, err)
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
		return 0, fmt.Errorf("invalid test case id %q: %w", testString, err)
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
	if err != nil {
		return 0, 0, err
	}

	return suiteID, testID, nil
}

// Response helpers

// serveJSON marshals the value to JSON and writes it to the response.
func serveJSON(w http.ResponseWriter, value interface{}) {
	resp, err := json.Marshal(value)
	if err != nil {
		slog.Error("API: internal error while encoding response", "error", err)
		serveError(w, errors.New("internal error"), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

// serveOK writes a successful null JSON response.
func serveOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "null")
}

// serveError writes an error response in JSON format.
func serveError(w http.ResponseWriter, err error, status int) {
	resp, _ := json.Marshal(&simapi.Error{Error: err.Error()})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(resp)
}