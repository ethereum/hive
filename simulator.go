package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

// simulateClients runs a batch of simulation tests matched by simulatorPattern
// against all clients matching clientPattern.
func simulateClients(daemon *docker.Client, clientPattern, simulatorPattern string, overrides []string) error {
	// Build all the clients matching the validation pattern
	log15.Info("building clients for simulation", "pattern", clientPattern)
	clients, err := buildClients(daemon, clientPattern)
	if err != nil {
		return err
	}
	// Build all the validators known to the test harness
	log15.Info("building simulators for testing", "pattern", simulatorPattern)
	simulators, err := buildSimulators(daemon, simulatorPattern)
	if err != nil {
		return err
	}
	// Iterate over all client and simulator combos and cross-execute them
	results := make(map[string]map[string][]string)

	for client, clientImage := range clients {
		results[client] = make(map[string][]string)

		for simulator, simulatorImage := range simulators {
			logger := log15.New("client", client, "simulator", simulator)

			logdir := filepath.Join(hiveLogsFolder, "simulations", fmt.Sprintf("%s[%s]", strings.Replace(simulator, "/", ":", -1), client))
			os.RemoveAll(logdir)

			start := time.Now()
			if pass, err := simulate(daemon, clientImage, simulatorImage, overrides, logger, logdir); pass {
				logger.Info("simulation passed", "time", time.Since(start))
				results[client]["pass"] = append(results[client]["pass"], simulator)
			} else {
				logger.Error("simulation failed", "time", time.Since(start))
				fail := simulator
				if err != nil {
					fail += ": " + err.Error()
				}
				results[client]["fail"] = append(results[client]["fail"], fail)
			}
		}
	}
	// Print the validation logs
	out, _ := json.MarshalIndent(results, "", "  ")
	fmt.Printf("Simulation results:\n%s\n", string(out))

	return nil
}

// simulate starts a simulator service locally, starts a controlling container
// and executes its commands until torn down. The exit statis of the controller
// container will signal whether the simulation passed or failed.
func simulate(daemon *docker.Client, client, simulator string, overrides []string, logger log15.Logger, logdir string) (bool, error) {
	logger.Info("running client simulation")

	// Start the simulator HTTP API
	sim, err := startSimulatorAPI(daemon, client, simulator, overrides, logger, logdir)
	if err != nil {
		logger.Error("failed to start simulator API", "error", err)
		return false, err
	}
	defer sim.Close()

	// Start the simulator controller container
	logger.Debug("creating simulator container")
	sc, err := daemon.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: simulator,
			Env:   []string{"HIVE_SIMULATOR=http://" + sim.listener.Addr().String()},
		},
	})
	if err != nil {
		logger.Error("failed to create simulator", "error", err)
		return false, err
	}
	slogger := logger.New("id", sc.ID[:8])
	slogger.Debug("created simulator container")
	defer func() {
		slogger.Debug("deleting simulator container")
		daemon.RemoveContainer(docker.RemoveContainerOptions{ID: sc.ID, Force: true})
	}()

	// Finish configuring the HTTP webserver with the controlled container
	sim.runner = sc

	// Start the tester container and wait until it finishes
	slogger.Debug("running simulator container")
	waiter, err := runContainer(daemon, sc.ID, slogger, filepath.Join(logdir, "simulator.log"), false)
	if err != nil {
		slogger.Error("failed to run simulator", "error", err)
		return false, err
	}
	waiter.Wait()

	// Retrieve the exist status to report pass of fail
	s, err := daemon.InspectContainer(sc.ID)
	if err != nil {
		slogger.Error("failed to inspect simulator", "error", err)
		return false, err
	}
	return s.State.ExitCode == 0, nil
}

// startSimulatorAPI starts an HTTP webserver listening for simulator commands
// on the docker bridge and executing them until it is torn down.
func startSimulatorAPI(daemon *docker.Client, client, simulator string, overrides []string, logger log15.Logger, logdir string) (*simulatorAPIHandler, error) {
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
		listener:  listener,
		daemon:    daemon,
		logger:    logger,
		logdir:    logdir,
		client:    client,
		simulator: simulator,
		overrides: overrides,
		nodes:     make(map[string]*docker.Container),
	}
	go http.Serve(listener, sim)

	return sim, nil
}

// simulatorAPIHandler is the HTTP request handler directing the docker engine
// with the commands from the simulator runner.
type simulatorAPIHandler struct {
	listener *net.TCPListener

	daemon    *docker.Client
	logger    log15.Logger
	logdir    string
	client    string
	simulator string
	overrides []string
	autoID    uint32

	runner *docker.Container
	nodes  map[string]*docker.Container
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
			node, ok := h.nodes[id]
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
			// Create and start a new client container
			logger.Debug("starting new client")
			container, err := createClientContainer(h.daemon, h.client, h.simulator, h.runner, h.overrides, envs)
			if err != nil {
				logger.Error("failed to create client", "error", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			logger = logger.New("id", container.ID[:8])

			logfile := fmt.Sprintf("client-%s.log", container.ID[:8])
			if _, err = runContainer(h.daemon, container.ID, logger, filepath.Join(h.logdir, logfile), false); err != nil {
				logger.Error("failed to start client", "error", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
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
			// Container online and responsive, return it's ID for later reference
			h.nodes[container.ID[:8]] = container
			fmt.Fprintf(w, "%s", container.ID[:8])
			return

		case "/logs":
			body, _ := ioutil.ReadAll(r.Body)
			h.logger.Info("message from simulator", "log", string(body))

		default:
			http.Error(w, "not found", http.StatusNotFound)
		}

	case "DELETE":
		// Data deletion, execute the request and return the results
		switch {
		case strings.HasPrefix(r.URL.Path, "/nodes/"):
			// Node deletion requested
			id := strings.TrimPrefix(r.URL.Path, "/nodes/")
			node, ok := h.nodes[id]
			if !ok {
				logger.Error("unknown client deletion requested", "id", id)
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			h.logger.Debug("deleting client container", "id", node.ID[:8])
			h.daemon.RemoveContainer(docker.RemoveContainerOptions{ID: node.ID, Force: true})
			delete(h.nodes, id)

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
		h.daemon.RemoveContainer(docker.RemoveContainerOptions{ID: node.ID, Force: true})
	}
}
