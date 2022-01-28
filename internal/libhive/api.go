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
	w.Header().Set("Content-Type", "application/json")

	if r.URL.Query().Get("metadata") != "" {
		// New-style response with metadata included.
		clients := make([]*ClientDefinition, 0, len(api.env.Definitions))
		for _, def := range api.env.Definitions {
			clients = append(clients, def)
		}
		sort.Slice(clients, func(i, j int) bool { return clients[i].Name < clients[j].Name })
		json.NewEncoder(w).Encode(clients)
	} else {
		clients := make([]string, 0, len(api.env.Definitions))
		for name := range api.env.Definitions {
			clients = append(clients, name)
		}
		sort.Strings(clients)
		json.NewEncoder(w).Encode(clients)
	}
}

// startSuite starts a suite.
func (api *simAPI) startSuite(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := r.Form.Get("name")
	desc := r.Form.Get("description")
	suiteID, err := api.tm.StartTestSuite(name, desc)
	if err != nil {
		log15.Error("API: StartTestSuite failed", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	log15.Info("API: suite started", "suite", suiteID, "name", name)
	fmt.Fprintf(w, "%d", suiteID)
}

// endSuite ends a suite.
func (api *simAPI) endSuite(w http.ResponseWriter, r *http.Request) {
	suiteID, err := api.requestSuite(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := api.tm.EndTestSuite(suiteID); err != nil {
		log15.Error("API: EndTestSuite failed", "suite", suiteID, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log15.Info("API: suite ended", "suite", suiteID)
}

// startTest signals the start of a test case.
func (api *simAPI) startTest(w http.ResponseWriter, r *http.Request) {
	suiteID, err := api.requestSuite(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := r.Form.Get("name")
	testID, err := api.tm.StartTest(suiteID, name, r.Form.Get("description"))
	if err != nil {
		msg := fmt.Sprintf("can't start test case: %s", err.Error())
		http.Error(w, msg, http.StatusInternalServerError)
	}
	log15.Info("API: test started", "suite", suiteID, "test", testID, "name", name)
	fmt.Fprintf(w, "%d", testID)
}

// endTest signals the end of a test case. It also shuts down all clients
// associated with the test.
func (api *simAPI) endTest(w http.ResponseWriter, r *http.Request) {
	suiteID, testID, err := api.requestSuiteAndTest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var (
		summary         = TestResult{Pass: false}
		responseWritten = false // Have we already committed a response?
	)
	defer func() {
		err := api.tm.EndTest(suiteID, testID, &summary)
		if err == nil {
			log15.Info("API: test ended", "suite", suiteID, "test", testID, "pass", summary.Pass)
			return
		}
		log15.Error("API: EndTest failed", "suite", suiteID, "test", testID, "error", err)
		if !responseWritten {
			msg := fmt.Sprintf("can't end test case: %v", err)
			http.Error(w, msg, http.StatusInternalServerError)
		}
	}()

	// Summary is required.
	summaryData := r.Form.Get("summaryresult")
	if summaryData == "" {
		http.Error(w, "missing 'summaryresult' in request", http.StatusBadRequest)
		responseWritten = true
		return
	}
	if err = json.Unmarshal([]byte(summaryData), &summary); err != nil {
		log15.Error("API: invalid summary data in endTest", "test", testID, "error", err)
		msg := fmt.Sprintf("can't unmarshal 'summaryresult': %v", err)
		http.Error(w, msg, http.StatusBadRequest)
		responseWritten = true
		return
	}
}

// startClient starts a client container.
func (api *simAPI) startClient(w http.ResponseWriter, r *http.Request) {
	suiteID, testID, err := api.requestSuiteAndTest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Client launch parameters are given as multipart/form-data.
	if err := r.ParseMultipartForm((1 << 10) * 4); err != nil {
		log15.Error("API: could not parse node request", "error", err)
		http.Error(w, "could not parse node request", http.StatusBadRequest)
		return
	}
	files := make(map[string]*multipart.FileHeader)
	for key, fheaders := range r.MultipartForm.File {
		if len(fheaders) > 0 {
			files[key] = fheaders[0]
		}
	}
	env := make(map[string]string)
	for key, vals := range r.MultipartForm.Value {
		if strings.HasPrefix(key, hiveEnvvarPrefix) {
			env[key] = vals[0]
		}
	}
	// Set default client loglevel to sim loglevel.
	if env["HIVE_LOGLEVEL"] == "" {
		env["HIVE_LOGLEVEL"] = strconv.Itoa(api.env.SimLogLevel)
	}

	// Get the client name.
	clientDef, ok := api.checkClient(r, w)
	if !ok {
		return
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
		http.Error(w, "client container create failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Set the log file. We need the container ID for this,
	// so it can only be set after creating the container.
	logPath, logFilePath := api.clientLogFilePaths(clientDef.Name, containerID)
	options.LogFile = logFilePath

	// by default: check the eth1 port
	options.CheckLive = 8545
	if portStr := env["HIVE_CHECK_LIVE_PORT"]; portStr != "" {
		v, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			log15.Error("API: could not parse check-live port", "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
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
		api.tm.testSuiteMutex.Lock()

		// log client version in test suite
		if suite, ok := api.tm.runningTestSuites[suiteID]; !ok {
			http.Error(w, ErrNoSuchTestSuite.Error(), http.StatusNotFound)
			api.tm.testSuiteMutex.Unlock()
			return
		} else {
			suite.ClientVersions[clientDef.Name] = clientDef.Version
		}
		api.tm.testSuiteMutex.Unlock()

		// register the node
		api.tm.RegisterNode(testID, info.ID, clientInfo)
	}
	if err != nil {
		log15.Error("API: could not start client", "client", clientDef.Name, "container", containerID[:8], "error", err)
		http.Error(w, "client did not start: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log15.Info("API: client "+clientDef.Name+" started", "suite", suiteID, "test", testID, "container", containerID[:8])
	fmt.Fprintf(w, "%s@%s@%s", info.ID, info.IP, info.MAC)
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

func (api *simAPI) checkClient(r *http.Request, w http.ResponseWriter) (*ClientDefinition, bool) {
	name := r.FormValue("CLIENT")
	if name == "" {
		log15.Error("API: missing client name in start node request")
		http.Error(w, "missing 'CLIENT' in request", http.StatusBadRequest)
		return nil, false
	}
	def, ok := api.env.Definitions[name]
	if ok {
		return def, true
	}
	// Client name not found.
	log15.Error("API: unknown client name in start node request")
	http.Error(w, "unknown 'CLIENT' type in request", http.StatusBadRequest)
	return nil, false
}

// stopClient terminates a client container.
func (api *simAPI) stopClient(w http.ResponseWriter, r *http.Request) {
	_, testID, err := api.requestSuiteAndTest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	node := mux.Vars(r)["node"]

	err = api.tm.StopNode(testID, node)
	if err == ErrNoSuchNode {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// getNodeStatus returns the status of a client container.
func (api *simAPI) getNodeStatus(w http.ResponseWriter, r *http.Request) {
	suiteID, testID, err := api.requestSuiteAndTest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	node := mux.Vars(r)["node"]
	nodeInfo, err := api.tm.GetNodeInfo(suiteID, testID, node)
	if err != nil {
		log15.Error("API: can't find node", "node", node, "error", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	status := apiNodeInfo{ID: nodeInfo.ID, Name: nodeInfo.Name}
	statusJSON, _ := json.Marshal(&status)

	w.Header().Set("content-type", "application/json")
	w.Write(statusJSON)
}

func (api *simAPI) execInClient(w http.ResponseWriter, r *http.Request) {
	suiteID, testID, err := api.requestSuiteAndTest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	node := mux.Vars(r)["node"]
	nodeInfo, err := api.tm.GetNodeInfo(suiteID, testID, node)
	if err != nil {
		log15.Error("API: can't find node", "node", node, "error", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Parse and validate the exec request.
	commandline, err := parseExecRequest(r.Body)
	if err != nil {
		log15.Error("API: invalid exec request", "node", node, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	info, err := api.backend.RunProgram(r.Context(), nodeInfo.ID, commandline)
	if err != nil {
		log15.Error("API: client script exec error", "node", node, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(&info)
}

// parseExecRequest decodes and validates a client script exec request.
func parseExecRequest(r io.Reader) ([]string, error) {
	var request struct {
		Command []string `json:"command"`
	}
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	networkName := mux.Vars(r)["network"]
	err = api.tm.CreateNetwork(suiteID, networkName)
	if err != nil {
		log15.Error("API: failed to create network", "network", networkName, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log15.Info("API: network created", "name", networkName)
	fmt.Fprint(w, "success")
}

// networkRemove removes a docker network.
func (api *simAPI) networkRemove(w http.ResponseWriter, r *http.Request) {
	suiteID, err := api.requestSuite(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	network := mux.Vars(r)["network"]
	err = api.tm.RemoveNetwork(suiteID, network)
	if err != nil {
		log15.Error("API: failed to remove network", "network", network, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log15.Info("API: docker network removed", "network", network)
	fmt.Fprint(w, "success")
}

// networkIPGet gets the IP address of a container on a network.
func (api *simAPI) networkIPGet(w http.ResponseWriter, r *http.Request) {
	suiteID, err := api.requestSuite(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	node := mux.Vars(r)["node"]
	network := mux.Vars(r)["network"]
	ipAddr, err := api.tm.ContainerIP(suiteID, network, node)
	if err != nil {
		log15.Error("API: failed to get container IP", "container", node, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log15.Info("API: container IP requested", "network", network, "container", node, "ip", ipAddr)
	fmt.Fprint(w, ipAddr)
}

// networkConnect connects a container to a network.
func (api *simAPI) networkConnect(w http.ResponseWriter, r *http.Request) {
	suiteID, err := api.requestSuite(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := mux.Vars(r)["network"]
	containerID := mux.Vars(r)["node"]
	if err := api.tm.ConnectContainer(suiteID, name, containerID); err != nil {
		log15.Error("API: failed to connect container", "network", name, "container", containerID, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log15.Info("API: container connected to network", "network", name, "container", containerID)
}

// networkDisconnect disconnects a container from a network.
func (api *simAPI) networkDisconnect(w http.ResponseWriter, r *http.Request) {
	suiteID, err := api.requestSuite(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	network := mux.Vars(r)["network"]
	containerID := mux.Vars(r)["node"]
	if err := api.tm.DisconnectContainer(suiteID, network, containerID); err != nil {
		log15.Error("API: disconnecting container failed", "network", network, "container", containerID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log15.Info("API: container disconnected", "network", network, "container", containerID)
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
