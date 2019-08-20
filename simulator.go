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
	"time"

	"github.com/ethereum/hive/simulators/common"
	"github.com/gorilla/pat"

	docker "github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

var simListenerAddress string
var testManager *common.TestManager


// runSimulations runs each 'simulation' container, which are hosts for executing one or more test-suites
func runSimulations(simulatorPattern string, overrides []string, cacher *buildCacher) error {

	// Build all the simulators known to the test harness
	log15.Info("building simulators for testing", "pattern", simulatorPattern)
	simulators, err := buildSimulators(dockerClient, simulatorPattern, cacher)
	if err != nil {
		return nil, err
	}

	// Create a testcase manager
	testManager=common.NewTestManager(*testResultsRoot,killNodeCallbackHandler)

	// Start the simulator HTTP API
	simAPI, err := startTestSuiteAPI()
	if err != nil {
		logger.Error("failed to start simulator API", "error", err)
		return err
	}
	defer simApi.Close()

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

		err = simulate(*simLimiterFlag, simulatorImage, simulator, overrides, logger, logdir)
		if err != nil {
			return nil, err
		}

	}
	return results, nil
}

// simulate starts a simulator service locally, starts a controlling container
// and executes its commands until torn down. The exit status of the controller
// container will signal whether the simulation passed or failed.
func simulate(simDuration int, simulator string, simulatorLabel string, overrides []string, logger log15.Logger, logdir string) error {
	logger.Info(fmt.Fprintf("running client simulation: %s", simulatorLabel))



	// Start the simulator controller container
	logger.Debug("creating simulator container")
	hostConfig := &docker.HostConfig{Privileged: true, CapAdd: []string{"SYS_PTRACE"}, SecurityOpt: []string{"seccomp=unconfined"}}
	sc, err := dockerClient.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: simulator,
			Env: []string{"HIVE_SIMULATOR=http://" + sim.listener.Addr().String(),
				"HIVE_DEBUG=" + strconv.FormatBool(*hiveDebug),
				"HIVE_PARALLELISM=" + fmt.Sprintf("%d", *simulatorParallelism),
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
	//TODO - Simulator.log? Need to decide how to organise the execution logs.
	waiter, err := runContainer(sc.ID, slogger, filepath.Join(logdir, "simulator.log"), false, *loglevelFlag)
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
func startTestSuiteAPI() {
	// Find the IP address of the host container
	log15.Debug("looking up docker bridge IP")
	bridge, err := lookupBridgeIP(log15)
	if err != nil {
		log15.Error("failed to lookup bridge IP", "error", err)
		return nil, err
	}
	simListenerAddress= fmt.Sprintf("%s:0", bridge)
	log15.Debug("docker bridge IP found", "ip", bridge)
	
	
	// Start the API webserver for simulators to coordinate with
	
//	addr, _ := net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:0", bridge))

	// listener, err := net.ListenTCP("tcp4", addr)
	// if err != nil {
	// 	logger.Error("failed to listen on bridge adapter", "error", err)
	// 	return nil, err
	// }
	
	

	// Serve connections until the listener is terminated
	log15.Debug("starting simulator API server")
	
	var mux *pat.Router = pat.New()
	mux.Get("/testsuite/{suite}/test/{test}/node/{node}", nodeInfoGet)
	mux.Post("/testsuite/{suite}/test/{test}/node", nodeStart)
	mux.Post("/testsuite/{suite}/test/{test}/pseudo", pseudoStart)
	mux.Delete("/testsuite/{suite}/test/{test}/node/{node}", nodeKill)
	mux.Post("/testsuite/{suite}/test/{test}", testDelete) //post because the delete http verb does not always support a message body
	mux.Post("/testsuite/{suite}/test",testStart) 
	mux.Delete("/testsuite/{suite}",suiteEnd)
	mux.Post("/testsuite", suiteStart)
	mux.Get("/clients", clientTypesGet)
	go CheckTimeout()
	go http.ListenAndServe(addr, sim)
	

	return sim, nil
}

// nodeInfoGet tries to execute the mandatory enode.sh , which returns the enode id
func nodeInfoGet(w http.ResponseWriter, request *http.Request) {
	testSuiteString := request.URL.Query().Get(":suite")
	testString := request.URL.Query().Get(":test")
	node := request.URL.Query().Get(":node")

	testSuite,err := strconv.Atoi(testSuiteString)
	if err!=nil {
		log15.Error("invalid test suite.")	
		http.Error(w, "invalid test suite", http.StatusBadRequest)
		return
	}
	testCase,err := strconv.Atoi(testString)
	if err!=nil {
		log15.Error("invalid test case.")	
		http.Error(w, "invalid test case", http.StatusBadRequest)
		return
	}
	nodeInfo, err:= testManager.GetNode(common.TestSuiteID(testSuite), common.TestID(testCase), node )
	if err!=nil {
		log15.Errorf("unable to get node: %s", err.Error())
		http.Error(w, err.Error(), http.StatusNotFound)
	}
	exec, err := dockerClient.CreateExec(docker.CreateExecOptions{
		AttachStdout: true,
		AttachStderr: false,
		Tty:          false,
		Cmd:          []string{"/enode.sh"},
		Container:    nodeInfo.ID, //id is the container id
	})
	if err != nil {
		log15.Error("failed to create target enode exec", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = dockerClient.StartExec(exec.ID, docker.StartExecOptions{
		Detach:       false,
		OutputStream: w,
	})
	if err != nil {
		log15.Error("failed to start target enode exec", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
    
}

func nodeStart(w http.ResponseWriter, request *http.Request) {
	testSuiteString := request.URL.Query().Get(":suite")
	testString := request.URL.Query().Get(":test")
	
    
}
func pseudoStart(w http.ResponseWriter, request *http.Request) {
	testSuiteString := request.URL.Query().Get(":suite")
	testString := request.URL.Query().Get(":test")
	
    
}
func nodeKill(w http.ResponseWriter, request *http.Request) {
	testSuiteString := request.URL.Query().Get(":suite")
	testString := request.URL.Query().Get(":test")
	node := request.URL.Query().Get(":node")
    
}
func testDelete(w http.ResponseWriter, request *http.Request) {
	testSuiteString := request.URL.Query().Get(":suite")
	testString := request.URL.Query().Get(":test")
	
    
}
func testStart(w http.ResponseWriter, request *http.Request) {
	testSuiteString := request.URL.Query().Get(":suite")
	
	
    
}
func suiteEnd(w http.ResponseWriter, request *http.Request) {
	testSuiteString := request.URL.Query().Get(":suite")
	
	
    
}
func suiteStart(w http.ResponseWriter, request *http.Request) {

	
	
    
}
func clients(w http.ResponseWriter, request *http.Request){

}

// ServeHTTP handles all the simulator API requests and executes them.
func (h *testSuiteAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	
	log15.Debug("new simulator request", "from", r.RemoteAddr, "method", r.Method, "endpoint", r.URL.Path)

	switch r.Method {
	case "GET":
		// Information retrieval, fetch whatever's needed and return it
		switch {
		case r.URL.Path == "/docker":
			// Docker infos requested, gather and send them back
			info, err := dockerClient.Info()
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
			containerInfo, ok := h.nodes[id]
			h.lock.Unlock()
			if !ok {
				logger.Error("unknown client requested", "id", id)
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			container, err := dockerClient.InspectContainer(containerInfo.container.ID)
			if err != nil {
				logger.Error("failed to inspect client", "error", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			fmt.Fprintf(w, "%s", container.NetworkSettings.IPAddress)

		case strings.HasPrefix(r.URL.Path, "/enodes/"):
			
			

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

			h.newNode(w, r, logger, h.availableClients, true, true)
			return
		case "/pseudos":

			h.newNode(w, r, logger, h.availablePseudos, false, false)
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
			containerInfo, exist := h.nodes[nodeid]
			if !exist {
				// Add an error even so
				if containerInfo, exist := h.timedOutNodes[nodeid]; exist {
					delete(h.timedOutNodes, nodeid)
					res := h.result[containerInfo.name][h.simulatorLabel]
					res.Subresults = append(res.Subresults, simulationSubresult{
						Name:    r.Form.Get("name"),
						Success: success,
						Error:   fmt.Sprintf("%s (killed after timeout by Hive)", r.Form.Get("error")),
						Details: details,
					})
				}
				h.lock.Unlock()
				http.Error(w, fmt.Sprintf("unknown node %v", nodeid), http.StatusBadRequest)
				return
			}
			res := h.result[containerInfo.name][h.simulatorLabel]
			res.Subresults = append(res.Subresults, simulationSubresult{
				Name:    r.Form.Get("name"),
				Success: success,
				Error:   r.Form.Get("error"),
				Details: details,
			})
			// Also terminate the container now
			delete(h.nodes, nodeid)
			logger.Debug("deleting client container", "id", nodeid)
			h.lock.Unlock()
			if err := dockerClient.RemoveContainer(docker.RemoveContainerOptions{ID: containerInfo.container.ID, Force: true}); err != nil {
				logger.Error("failed to delete client ", "id", nodeid, "error", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

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


//TODO: revisit :

type containerInfo struct {
	container  *docker.Container
	name       string
	timeout    time.Time
	useTimeout bool
}

//TODO: revisit :

// CheckTimeout is a goroutine that checks if the timeout has passed and stops
// container if it has.
func  CheckTimeout() {
	for {
		h.lock.Lock()
		for id, cInfo := range h.nodes {

			cont, err := dockerClient.InspectContainer(cInfo.container.ID)
			if err != nil {
				//container already gone
				h.logger.Info("Container already deleted. ", "Container", cInfo.container.ID)
			} else {
				cInfo.container = cont
				if !cInfo.container.State.Running || (time.Now().Sub(cInfo.timeout) >= 0 && cInfo.useTimeout) {

					h.logger.Info("Timing out. ", "Running", cInfo.container.State.Running)
					h.timeoutContainer(id, nil)

					// remember this container, for when the subresult comes in later
					h.timedOutNodes[id] = cInfo
				}

			}

		}
		h.lock.Unlock()
		time.Sleep(timeoutCheckDuration)
	}
}

// timeoutContainer terminates a container. OBS! It assumes that the caller already holds h.lock
func (h *testSuiteAPIHandler) timeoutContainer(id string, w http.ResponseWriter) {
	containerInfo, ok := h.nodes[id]

	if !ok {
		h.logger.Error("unknown client deletion requested", "id", id)
		if w != nil {
			http.Error(w, "not found", http.StatusNotFound)
		}
		return
	}
	delete(h.nodes, id)
	h.logger.Debug("deleting client container on timeout", "id", id)
	if err := dockerClient.RemoveContainer(docker.RemoveContainerOptions{ID: containerInfo.container.ID, Force: true}); err != nil {
		h.logger.Error("failed to delete client ", "id", id, "error", err)
		if w != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// terminateContainer terminates a container. OBS! It assumes that the caller already holds h.lock
func (h *testSuiteAPIHandler) terminateContainer(id string, w http.ResponseWriter) {
	containerInfo, ok := h.nodes[id]
	if !ok {
		h.logger.Error("unknown client deletion requested", "id", id)
		if w != nil {
			http.Error(w, "not found", http.StatusNotFound)
		}
		return
	}
	delete(h.nodes, id)
	h.logger.Debug("deleting client container", "id", id)
	if err := dockerClient.RemoveContainer(docker.RemoveContainerOptions{ID: containerInfo.container.ID, Force: true}); err != nil {
		h.logger.Error("failed to delete client ", "id", id, "error", err)
		if w != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func (h *testSuiteAPIHandler) newNode(w http.ResponseWriter, r *http.Request, logger log15.Logger, checkliveness bool, useTimeout bool) {
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

	//default the loglevel to the simulator log level setting (different from the sysem log level setting)
	logLevel := *simloglevelFlag
	logLevelString, in := envs["HIVE_LOGLEVEL"]
	if !in {
		envs["HIVE_LOGLEVEL"] = strconv.Itoa(logLevel)
	} else {
		var err error
		if logLevel, err = strconv.Atoi(logLevelString); err != nil {
			logger.Error("Simulator client HIVE_LOGLEVEL is not an integer", "error", nil)
		}
	}

	//the simulation host may prevent or be unaware of the simulation controller's requested client
	imageName, in := allClients[clientName]
	if !in {
		logger.Error("Unknown or forbidden client type", "error", nil)
		http.Error(w, "Unknown or forbidden client type", http.StatusBadRequest)
		return
	}

	// Create and start the requested client container
	logger.Debug("starting new client")
	container, err := createClientContainer(imageName,  envs)
	if err != nil {
		logger.Error("failed to create client", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	containerID := container.ID[:8]
	containerIP := ""
	containerMAC := ""
	logger = logger.New("client started with id", containerID)

	logfile := fmt.Sprintf("client-%s.log", containerID)

	waiter, err := runContainer(container.ID, logger, filepath.Join(h.logdir, strings.Replace(clientName, string(filepath.Separator), "_", -1), logfile), false, logLevel)
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
		// Update the container state
		container, err = dockerClient.InspectContainer(container.ID)
		if err != nil {
			logger.Error("failed to inspect client", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !container.State.Running {
			logger.Error("client container terminated")
			http.Error(w, "terminated unexpectedly", http.StatusInternalServerError)
			return
		}

		containerIP = container.NetworkSettings.IPAddress
		containerMAC = container.NetworkSettings.MacAddress
		if checkliveness {
			logger.Debug("Checking container online....")
			// Container seems to be alive, check whether the RPC is accepting connections
			if conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", containerIP, 8545)); err == nil {
				logger.Debug("client container online", "time", time.Since(start))
				conn.Close()
				break
			}
		} else {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}
	h.lock.Lock()


	// TODO - replace this with adding into the test case structure

	// h.nodes[containerID] = &containerInfo{
	// 	container:  container,
	// 	name:       clientName,
	// 	timeout:    time.Now().Add(dockerTimeoutDuration),
	// 	useTimeout: useTimeout,
	// }
	h.lock.Unlock()
	//  Container online and responsive, return its ID, IP and MAC for later reference
	fmt.Fprintf(w, "%s@%s@%s", containerID, containerIP, containerMAC)
	return
}



// Close terminates all running containers and tears down the API server.
func (h *testSuiteAPIHandler) Close() {
	h.logger.Debug("terminating simulator server")
	h.listener.Close()

	for _, containerInfo := range h.nodes {
		id := containerInfo.container.ID
		h.logger.Debug("deleting client container", "id", id[:8])
		if err := dockerClient.RemoveContainer(docker.RemoveContainerOptions{ID: id, Force: true}); err != nil {
			h.logger.Error("failed to delete client container", "id", id[:8], "error", err)
		}
	}
}
