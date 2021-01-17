package libhive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/p2p/enode"
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

	// Collect client types.
	for name := range env.Images {
		api.clientTypes = append(api.clientTypes, name)
	}
	sort.Strings(api.clientTypes)

	// API routes.
	router := mux.NewRouter()
	router.HandleFunc("/clients", api.getClientTypes).Methods("GET")
	router.HandleFunc("/testsuite/{suite}/test/{test}/node/{node}", api.getEnodeURL).Methods("GET")
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
	clientTypes []string
	backend     ContainerBackend
	env         SimEnv
	tm          *TestManager
}

// getClientTypes returns all known client types.
func (api *simAPI) getClientTypes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(api.clientTypes)
}

// startSuite starts a suite.
func (api *simAPI) startSuite(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := r.Form.Get("name")
	desc := r.Form.Get("description")
	simlog := r.Form.Get("simlog")
	suiteID, err := api.tm.StartTestSuite(name, desc, simlog)
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
		clientResults   map[string]*TestResult
		responseWritten = false // Have we already committed a response?
	)
	defer func() {
		err := api.tm.EndTest(suiteID, testID, &summary, clientResults)
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
	// Client results are optional.
	if crdata := r.Form.Get("clientresults"); crdata != "" {
		if err := json.Unmarshal([]byte(crdata), &clientResults); err != nil {
			log15.Error("API: invalid 'clientresults'", "test", testID, "error", err)
		}
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
	envs := make(map[string]string)
	for key, vals := range r.MultipartForm.Value {
		if strings.HasPrefix(key, hiveEnvvarPrefix) {
			envs[key] = vals[0]
		}
	}

	// Get the client name.
	name, ok := api.checkClient(r, w)
	if !ok {
		return
	}

	// Start it!
	info, err := api.startClientContainer(name, envs, files)
	if info != nil {
		clientInfo := &ClientInfo{
			ID:              info.ID,
			IP:              info.IP,
			MAC:             info.MAC,
			Name:            name,
			VersionInfo:     api.env.ClientVersions[name],
			InstantiatedAt:  time.Now(),
			LogFile:         info.LogFile,
			WasInstantiated: true, // ???
		}
		api.tm.RegisterNode(testID, info.ID, clientInfo)
	}
	if err != nil {
		log15.Error("API: could not start client", "client", name, "error", err)
		http.Error(w, "client did not start: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log15.Info("API: client "+name+" started", "suite", suiteID, "test", testID, "container", info.ID)
	fmt.Fprintf(w, "%s@%s@%s", info.ID, info.IP, info.MAC)
}

func (api *simAPI) startClientContainer(name string, env map[string]string, files map[string]*multipart.FileHeader) (*ContainerInfo, error) {
	safeName := strings.Replace(name, string(filepath.Separator), "_", -1)
	opts := ContainerOptions{
		LogDir:        filepath.Join(api.env.LogDir, safeName),
		LogFilePrefix: "client-",
		CheckLive:     true,
		Env:           env,
		Files:         files,
	}

	// Set up the timeout.
	timeout := api.env.ClientStartTimeout
	if timeout == 0 {
		timeout = defaultStartTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return api.backend.StartContainer(ctx, api.env.Images[name], opts)
}

func (api *simAPI) checkClient(r *http.Request, w http.ResponseWriter) (string, bool) {
	name := r.FormValue("CLIENT")
	if name == "" {
		log15.Error("API: missing client name in start node request")
		http.Error(w, "missing 'CLIENT' in request", http.StatusBadRequest)
		return "", false
	}
	for _, cn := range api.clientTypes {
		if cn == name {
			return name, true
		}
	}
	// Client name not found.
	log15.Error("API: unknown client name in start node request")
	http.Error(w, "unknown 'CLIENT' type in request", http.StatusBadRequest)
	return "", false
}

// stopClient terminates a client container.
func (api *simAPI) stopClient(w http.ResponseWriter, r *http.Request) {
	suiteID, testID, err := api.requestSuiteAndTest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	node := mux.Vars(r)["node"]

	// Get the node.
	nodeInfo, err := api.tm.GetNodeInfo(suiteID, testID, node)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Stop the container.
	if err = api.backend.StopContainer(nodeInfo.ID); err != nil {
		msg := fmt.Sprintf("unable to stop client: %v", err)
		http.Error(w, msg, http.StatusInternalServerError)
	}
}

// getEnodeURL gets the enode URL of the client.
func (api *simAPI) getEnodeURL(w http.ResponseWriter, r *http.Request) {
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
	output, err := api.backend.RunEnodeSh(nodeInfo.ID)
	if err != nil {
		log15.Error("API: error running enode.sh", "node", node, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Check that the container returned a valid enode URL.
	output = strings.TrimSpace(output)
	n, err := enode.ParseV4(output)
	if err != nil {
		log15.Error("API: enode.sh returned bad URL", "node", node, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tcpPort := n.TCP()
	if tcpPort == 0 {
		log15.Warn("API: enode.sh returned TCP port zero", "node", node, "client", nodeInfo.Name)
		tcpPort = 30303
	}
	udpPort := n.UDP()
	if udpPort == 0 {
		log15.Warn("API: enode.sh returned UDP port zero", "node", node, "client", nodeInfo.Name)
		udpPort = 30303
	}

	// Switch out the IP with the container's IP on the primary network.
	// This is required because the client usually doesn't know its own IP.
	fixedIP := enode.NewV4(n.Pubkey(), net.ParseIP(nodeInfo.IP), tcpPort, udpPort)
	io.WriteString(w, fixedIP.URLv4())
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
