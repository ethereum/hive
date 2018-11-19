package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

// simulationResult represents the results of a simulation run, containing
// various metadata as well as possibly multiple sub-results in case where
// the same simulator tested multiple things in one go.
type simulationResult struct {
	Start   time.Time `json:"start"`           // Time instance when the simulation ended
	End     time.Time `json:"end"`             // Time instance when the simulation ended
	Success bool      `json:"success"`         // Whether the entire simulation succeeded
	Error   error     `json:"error,omitempty"` // Potential hive failure during simulation

	Subresults []simulationSubresult `json:"subresults,omitempty"` // Optional list of subresults to report

}

type simulationResultSummary struct {
	Start   time.Time `json:"start"`           // Time instance when the simulation ended
	End     time.Time `json:"end"`             // Time instance when the simulation ended
	Success bool      `json:"success"`         // Whether the entire simulation succeeded
	Error   error     `json:"error,omitempty"` // Potential hive failure during simulation

	summaryData
}

// simulationSubresult represents a sub-test a simulation may run and report.
type simulationSubresult struct {
	Name string `json:"name"` // Unique name for a sub-test within a simulation

	Success bool            `json:"success"`           // Whether the sub-test succeeded or not
	Error   string          `json:"error,omitempty"`   // Textual details to explain a failure
	Details json.RawMessage `json:"details,omitempty"` // Structured infos a tester mightw wish to surface
}

// simulateClients runs a batch of simulation tests matched by simulatorPattern
// against a set of clients matching clientPattern, where  the simulator decides
// which of those clients to invoke
func simulateClients(daemon *docker.Client, clientPattern, simulatorPattern string, overrides []string, cacher *buildCacher) (map[string]map[string]*simulationResult, error) {
	// Build all the clients matching the validation pattern
	log15.Info("building clients for simulation", "pattern", clientPattern)
	clients, err := buildClients(daemon, clientPattern, cacher)
	if err != nil {
		return nil, err
	}

	// Build all the simulators known to the test harness
	log15.Info("building simulators for testing", "pattern", simulatorPattern)
	simulators, err := buildSimulators(daemon, simulatorPattern, cacher)
	if err != nil {
		return nil, err
	}

	// The results are a map of clients=>simulators=>results
	results := make(map[string]map[string]*simulationResult)

	//build the per-client simulator result set
	for client := range clients {
		results[client] = make(map[string]*simulationResult)
	}

	//set the end time of the test
	defer func() {
		for _, cv := range results {
			for _, sv := range cv {
				sv.End = time.Now()
			}
		}
	}()

	for simulator, simulatorImage := range simulators {

		logdir, err := makeTestOutputDirectory(strings.Replace(simulator, string(filepath.Separator), "_", -1), "simulator", clients)
		if err != nil {
			return nil, err
		}

		logger := log15.New("simulator", simulator)

		for client := range clients {
			results[client][simulator] = &simulationResult{
				Start: time.Now(),
			}

		}

		err = simulate(daemon, clients, simulatorImage, simulator, overrides, logger, logdir, results) //filepath.Join(logdir, strings.Replace(client, string(filepath.Separator), "_", -1)))
		if err != nil {
			return nil, err
		}

	}
	return results, nil
}

// simulate starts a simulator service locally, starts a controlling container
// and executes its commands until torn down. The exit status of the controller
// container will signal whether the simulation passed or failed.
func simulate(daemon *docker.Client, clients map[string]string, simulator string, simulatorLabel string, overrides []string, logger log15.Logger, logdir string, results map[string]map[string]*simulationResult) error {
	logger.Info("running client simulation")

	// Start the simulator HTTP API
	sim, err := startSimulatorAPI(daemon, clients, simulator, simulatorLabel, overrides, logger, logdir, results)
	if err != nil {
		logger.Error("failed to start simulator API", "error", err)
		return err

	}
	defer sim.Close()

	// Start the simulator controller container
	logger.Debug("creating simulator container")
	hostConfig := &docker.HostConfig{Privileged: true, CapAdd: []string{"SYS_PTRACE"}, SecurityOpt: []string{"seccomp=unconfined"}}
	sc, err := daemon.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: simulator,
			Env: []string{"HIVE_SIMULATOR=http://" + sim.listener.Addr().String(),
				"HIVE_DEBUG=" + strconv.FormatBool(*hiveDebug),
				"HIVE_PARALLELISM=" + fmt.Sprintf("%d", simulatorParallelism),
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
		if err := daemon.RemoveContainer(docker.RemoveContainerOptions{ID: sc.ID, Force: true}); err != nil {
			slogger.Error("failed to delete simulator container", "error", err)
		}
	}()

	// Finish configuring the HTTP webserver with the controlled container
	sim.runner = sc

	// Start the tester container and wait until it finishes
	slogger.Debug("running simulator container")
	waiter, err := runContainer(daemon, sc.ID, slogger, filepath.Join(logdir, "simulator.log"), false)
	if err != nil {
		slogger.Error("failed to run simulator", "error", err)
		return err
	}
	waiter.Wait()

	return nil

}

// startSimulatorAPI starts an HTTP webserver listening for simulator commands
// on the docker bridge and executing them until it is torn down.
func startSimulatorAPI(daemon *docker.Client, clients map[string]string, simulator string, simulatorLabel string, overrides []string, logger log15.Logger, logdir string, results map[string]map[string]*simulationResult) (*simulatorAPIHandler, error) {
	// Find the IP address of the host container
	logger.Debug("looking up docker bridge IP")
	bridge, err := lookupBridgeIP(logger)
	if err != nil {
		logger.Error("failed to lookup bridge IP", "error", err)
		return nil, err
	}
	logger.Debug("docker bridge IP found", "ip", bridge)

	// Start a tiny API webserver for simulators to coordinate with
	logger.Debug("opening TCP socket for simulator")

	addr, _ := net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:0", bridge))
	listener, err := net.ListenTCP("tcp4", addr)
	if err != nil {
		logger.Error("failed to listen on bridge adapter", "error", err)
		return nil, err
	}
	logger.Debug("listening for simulator commands", "ip", bridge, "port", listener.Addr().(*net.TCPAddr).Port)

	// Serve connections until the listener is terminated
	logger.Debug("starting simulator API server")
	sim := &simulatorAPIHandler{
		listener:         listener,
		daemon:           daemon,
		logger:           logger,
		logdir:           logdir,
		availableClients: clients,
		simulator:        simulator,
		simulatorLabel:   simulatorLabel,
		overrides:        overrides,
		nodes:            make(map[string]*docker.Container),
		nodeNames:        make(map[string]string),
		nodesTimeout:     make(map[string]time.Time),
		result:           results, //the simulator now has access to a map of results-by-client. The simulator decides which clients to run/
	}
	go sim.CheckTimeout()
	go http.Serve(listener, sim)

	return sim, nil
}

// simulatorAPIHandler is the HTTP request handler directing the docker engine
// with the commands from the simulator runner.
type simulatorAPIHandler struct {
	listener *net.TCPListener

	daemon           *docker.Client
	logger           log15.Logger
	logdir           string
	availableClients map[string]string //the client filter specified by the host. Simulations may not execute other clients.
	simulator        string            //the image name
	simulatorLabel   string            //the simulator label
	overrides        []string
	autoID           uint32

	runner       *docker.Container
	nodes        map[string]*docker.Container
	nodeNames    map[string]string
	nodesTimeout map[string]time.Time

	result map[string]map[string]*simulationResult //simulation result log per client name
	lock   sync.RWMutex
}

// CheckTimeout is a goroutine that checks if the timeout has passed and stops
// container if it has.
func (h *simulatorAPIHandler) CheckTimeout() {
	for {
		h.lock.Lock()
		for id, c := range h.nodes {
			if !c.State.Running || (time.Now().After(h.nodesTimeout[id])) {
				h.terminateContainer(id, nil)
			}
		}
		h.lock.Unlock()
		time.Sleep(timeoutCheckDuration)
	}
}

func (h *simulatorAPIHandler) terminateContainer(id string, w http.ResponseWriter) {
	node, ok := h.nodes[id]
	delete(h.nodes, id) // Almost correct, removal may fail. Lock is too expensive though
	delete(h.nodesTimeout, id)

	if !ok {
		h.logger.Error("unknown client deletion requested", "id", id)
		if w != nil {
			http.Error(w, "not found", http.StatusNotFound)
		}
		return
	}
	h.logger.Debug("deleting client container", "id", node.ID[:8])
	if err := h.daemon.RemoveContainer(docker.RemoveContainerOptions{ID: node.ID, Force: true}); err != nil {
		h.logger.Error("failed to delete client ", "id", id, "error", err)
		if w != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// ServeHTTP handles all the simulator API requests and executes them.
func (h *simulatorAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.New("req-id", atomic.AddUint32(&h.autoID, 1))
	logger.Debug("new simulator request", "from", r.RemoteAddr, "method", r.Method, "endpoint", r.URL.Path)

	switch r.Method {
	case "GET":
		// Information retrieval, fetch whatever's needed and return it
		switch {
		case r.URL.Path == "/docker":
			// Docker infos requested, gather and send them back
			info, err := h.daemon.Info()
			if err != nil {
				logger.Error("failed to gather docker infos", "error", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			out, _ := json.MarshalIndent(info, "", "  ")
			fmt.Fprintf(w, "%s\n", out)

		case strings.HasPrefix(r.URL.Path, "/nodes/"):
			// Node IP retrieval requested
			id := strings.TrimPrefix(r.URL.Path, "/nodes/")
			h.lock.Lock()
			node, ok := h.nodes[id]
			h.lock.Unlock()
			if !ok {
				logger.Error("unknown client requested", "id", id)
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			container, err := h.daemon.InspectContainer(node.ID)
			if err != nil {
				logger.Error("failed to inspect client", "error", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			fmt.Fprintf(w, "%s", container.NetworkSettings.IPAddress)

		//docker exec container bash -c 'echo "$ENV_VAR"'
		case strings.HasPrefix(r.URL.Path, "/enodes/"):
			// Node IP retrieval requested
			id := strings.TrimPrefix(r.URL.Path, "/enodes/")
			h.lock.Lock()
			container, ok := h.nodes[id]
			h.lock.Unlock()
			if !ok {
				logger.Error("unknown client for enode", "id", id)
				http.Error(w, "not found", http.StatusNotFound)
				return
			}

			exec, err := h.daemon.CreateExec(docker.CreateExecOptions{
				AttachStdout: true,
				AttachStderr: false,
				Tty:          false,
				Cmd:          []string{"/enode.sh"},
				Container:    container.ID,
			})
			if err != nil {
				logger.Error("failed to create target enode exec", "error", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			err = h.daemon.StartExec(exec.ID, docker.StartExecOptions{
				Detach:       false,
				OutputStream: w,
			})
			if err != nil {
				logger.Error("failed to start target enode exec", "error", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

		case strings.HasPrefix(r.URL.Path, "/clients"):
			w.Header().Set("Content-Type", "application/json")
			clients := make([]string, 0, len(h.availableClients))
			for client := range h.availableClients {
				clients = append(clients, client)
			}
			json.NewEncoder(w).Encode(clients)

		default:
			http.Error(w, "not found", http.StatusNotFound)
		}

	case "POST":
		// Data mutation, execute the request and return the results
		switch r.URL.Path {
		case "/nodes":
			// A new node startup was requested, fetch any envvar overrides from simulators
			r.ParseForm()
			envs := make(map[string]string)
			for key, vals := range r.Form {
				envs[key] = vals[0]
			}

			//the simulation controller needs to tell us now what client to run for the test
			clientName, in := envs["CLIENT"]
			if !in {
				logger.Error("Missing client type", "error", nil)
				http.Error(w, "Missing client type", http.StatusBadRequest)
				return
			}

			//the simulation host may prevent or be unaware of the simulation controller's requested client
			imageName, in := h.availableClients[clientName]
			if !in {
				logger.Error("Unknown or forbidden client type", "error", nil)
				http.Error(w, "Unknown or forbidden client type", http.StatusBadRequest)
				return
			}

			// Create and start the requested client container
			logger.Debug("starting new client")
			container, err := createClientContainer(h.daemon, imageName, h.simulator, h.runner, h.overrides, envs)
			if err != nil {
				logger.Error("failed to create client", "error", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			containerID := container.ID[:8]

			logger = logger.New("client started with id", containerID)

			logfile := fmt.Sprintf("client-%s.log", containerID)

			waiter, err := runContainer(h.daemon, container.ID, logger, filepath.Join(h.logdir, strings.Replace(clientName, string(filepath.Separator), "_", -1), logfile), false)
			if err != nil {
				logger.Error("failed to start client", "error", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
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
			for {
				// If the container died, bail out
				c, err := h.daemon.InspectContainer(container.ID)
				if err != nil {
					logger.Error("failed to inspect client", "error", err)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if !c.State.Running {
					logger.Error("client container terminated")
					http.Error(w, "terminated unexpectedly", http.StatusInternalServerError)
					return
				}
				// Container seems to be alive, check whether the RPC is accepting connections
				if conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", c.NetworkSettings.IPAddress, 8545)); err == nil {
					logger.Debug("client container online", "time", time.Since(start))
					conn.Close()
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			//  Container online and responsive, return its ID for later reference
			fmt.Fprintf(w, "%s", containerID)
			h.lock.Lock()
			h.nodes[containerID] = container
			h.nodeNames[containerID] = clientName
			h.nodesTimeout[containerID] = time.Now().Add(dockerTimeoutDuration)
			h.lock.Unlock()
			return

		case "/logs":
			body, _ := ioutil.ReadAll(r.Body)
			h.logger.Info("message from simulator", "log", string(body))

		case "/subresults":
			// Parse the subresult field into a hive struct
			r.ParseMultipartForm(1024 * 1024)

			success, err := strconv.ParseBool(r.Form.Get("success"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			//If there has been a failure, update the whole test result
			//which at present means updating the result set for each
			//known client. TODO: the output format should be
			//re-arranged so that it is grouped first by test and then by client instance type
			if !success {
				for _, resultset := range h.result {
					resultset[h.simulatorLabel].Success = false
				}
			}

			nodeid := r.Form.Get("nodeid")
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			var details json.RawMessage
			if blob := r.Form.Get("details"); blob != "" {
				if err := json.Unmarshal([]byte(blob), &details); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
			// If everything parsed correctly, append the subresult
			h.lock.Lock()

			imageName := h.nodeNames[nodeid]

			h.result[imageName][h.simulatorLabel].Subresults = append(h.result[imageName][h.simulatorLabel].Subresults, simulationSubresult{
				Name:    r.Form.Get("name"),
				Success: success,
				Error:   r.Form.Get("error"),
				Details: details,
			})
			h.lock.Unlock()
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}

	case "DELETE":
		// Data deletion, execute the request and return the results
		switch {
		case strings.HasPrefix(r.URL.Path, "/nodes/"):
			// Node deletion requested
			id := strings.TrimPrefix(r.URL.Path, "/nodes/")

			h.lock.Lock()
			h.terminateContainer(id, w)
			h.lock.Unlock()

		default:
			http.Error(w, "not found", http.StatusNotFound)
		}

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// Close terminates all running containers and tears down the API server.
func (h *simulatorAPIHandler) Close() {
	h.logger.Debug("terminating simulator server")
	h.listener.Close()

	for _, node := range h.nodes {
		h.logger.Debug("deleting client container", "id", node.ID[:8])
		if err := h.daemon.RemoveContainer(docker.RemoveContainerOptions{ID: node.ID, Force: true}); err != nil {
			h.logger.Error("failed to delete client container", "id", node.ID[:8], "error", err)
		}
	}
}
