package hivesim

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/hive/internal/simapi"
)

// Simulation wraps the simulation HTTP API provided by hive.
type Simulation struct {
	url string
	m   testMatcher
}

// New looks up the hive host URI using the HIVE_SIMULATOR environment variable
// and connects to it. It will panic if HIVE_SIMULATOR is not set.
func New() *Simulation {
	url, isSet := os.LookupEnv("HIVE_SIMULATOR")
	if !isSet {
		panic("HIVE_SIMULATOR environment variable not set")
	}
	if url == "" {
		panic("HIVE_SIMULATOR environment variable is empty")
	}
	sim := &Simulation{url: url}
	if p := os.Getenv("HIVE_TEST_PATTERN"); p != "" {
		m, err := parseTestPattern(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Warning: ignoring invalid test pattern regexp: "+err.Error())
		}
		sim.m = m
	}
	return sim
}

// NewAt creates a simulation connected to the given API endpoint. You'll will rarely need
// to use this. In simulations launched by hive, use New() instead.
func NewAt(url string) *Simulation {
	return &Simulation{url: url}
}

// SetTestPattern sets the regular expression that enables/skips suites and test cases.
// This method is provided for use in unit tests. For simulator runs launched by hive, the
// test pattern is set automatically in New().
func (sim *Simulation) SetTestPattern(p string) {
	m, err := parseTestPattern(p)
	if err != nil {
		panic("invalid test pattern regexp: " + err.Error())
	}
	sim.m = m
}

// TestPattern returns the regular expressions used to enable/skip suite and test names.
func (sim *Simulation) TestPattern() (suiteExpr string, testNameExpr string) {
	se := ""
	if sim.m.suite != nil {
		se = sim.m.suite.String()
	}
	te := ""
	if sim.m.test != nil {
		te = sim.m.test.String()
	}
	return se, te
}

// EndTest finishes the test case, cleaning up everything, logging results, and returning
// an error if the process could not be completed.
func (sim *Simulation) EndTest(testSuite SuiteID, test TestID, testResult TestResult) error {
	url := fmt.Sprintf("%s/testsuite/%d/test/%d", sim.url, testSuite, test)
	return post(url, &testResult, nil)
}

// StartSuite signals the start of a test suite.
func (sim *Simulation) StartSuite(name, description, simlog string) (SuiteID, error) {
	var (
		url  = fmt.Sprintf("%s/testsuite", sim.url)
		req  = &simapi.TestRequest{Name: name, Description: description}
		resp SuiteID
	)
	err := post(url, req, &resp)
	return resp, err
}

// EndSuite signals the end of a test suite.
func (sim *Simulation) EndSuite(testSuite SuiteID) error {
	url := fmt.Sprintf("%s/testsuite/%d", sim.url, testSuite)
	return requestDelete(url)
}

// StartTest starts a new test case, returning the testcase id as a context identifier.
func (sim *Simulation) StartTest(testSuite SuiteID, name string, description string) (TestID, error) {
	var (
		url  = fmt.Sprintf("%s/testsuite/%d/test", sim.url, testSuite)
		req  = &simapi.TestRequest{Name: name, Description: description}
		resp TestID
	)
	err := post(url, req, &resp)
	return resp, err
}

// ClientTypes returns all client types available to this simulator run. This depends on
// both the available client set and the command line filters.
func (sim *Simulation) ClientTypes() ([]*ClientDefinition, error) {
	var (
		url  = fmt.Sprintf("%s/clients", sim.url)
		resp []*ClientDefinition
	)
	err := get(url, &resp)
	return resp, err
}

// StartClient starts a new node (or other container) with the specified parameters. One
// parameter must be named CLIENT and should contain one of the client types from
// GetClientTypes. The input is used as environment variables in the new container.
// Returns container id and ip.
func (sim *Simulation) StartClient(testSuite SuiteID, test TestID, parameters map[string]string, initFiles map[string]string) (string, net.IP, error) {
	clientType, ok := parameters["CLIENT"]
	if !ok {
		return "", nil, errors.New("missing 'CLIENT' parameter")
	}
	return sim.StartClientWithOptions(testSuite, test, clientType, Params(parameters), WithStaticFiles(initFiles))
}

// StartClientWithOptions starts a new node (or other container) with specified options.
// Returns container id and ip.
func (sim *Simulation) StartClientWithOptions(testSuite SuiteID, test TestID, clientType string, options ...StartOption) (string, net.IP, error) {
	var (
		url  = fmt.Sprintf("%s/testsuite/%d/test/%d/node", sim.url, testSuite, test)
		resp simapi.StartNodeResponse
	)

	setup := &clientSetup{
		files: make(map[string]func() (io.ReadCloser, error)),
		config: simapi.NodeConfig{
			Client:      clientType,
			Environment: make(map[string]string),
		},
	}
	for _, opt := range options {
		opt.apply(setup)
	}

	err := setup.postWithFiles(url, &resp)
	if err != nil {
		return "", nil, err
	}
	ip := net.ParseIP(resp.IP)
	if ip == nil {
		return resp.ID, nil, fmt.Errorf("no IP address returned")
	}
	return resp.ID, ip, nil
}

// StopClient signals to the host that the node is no longer required.
func (sim *Simulation) StopClient(testSuite SuiteID, test TestID, nodeid string) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/testsuite/%d/test/%d/node/%s", sim.url, testSuite, test, nodeid), nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
}

// PauseClient signals to the host that the node needs to be paused.
func (sim *Simulation) PauseClient(testSuite SuiteID, test TestID, nodeid string) error {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/testsuite/%d/test/%d/node/%s/pause", sim.url, testSuite, test, nodeid), nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
}

// UnpauseClient signals to the host that the node needs to be unpaused.
func (sim *Simulation) UnpauseClient(testSuite SuiteID, test TestID, nodeid string) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/testsuite/%d/test/%d/node/%s/pause", sim.url, testSuite, test, nodeid), nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
}

// ClientEnodeURL returns the enode URL of a running client.
func (sim *Simulation) ClientEnodeURL(testSuite SuiteID, test TestID, node string) (string, error) {
	return sim.ClientEnodeURLNetwork(testSuite, test, node, "bridge")
}

// ClientEnodeURLCustomNetwork returns the enode URL of a running client in a custom network.
func (sim *Simulation) ClientEnodeURLNetwork(testSuite SuiteID, test TestID, node string, network string) (string, error) {
	resp, err := sim.ClientExec(testSuite, test, node, []string{"enode.sh"})
	if err != nil {
		return "", err
	}
	if resp.ExitCode != 0 {
		return "", errors.New("unexpected exit code for enode.sh")
	}

	// Check that the container returned a valid enode URL.
	output := strings.TrimSpace(resp.Stdout)
	n, err := enode.ParseV4(output)
	if err != nil {
		return "", err
	}

	// Check ports returned
	tcpPort := n.TCP()
	if tcpPort == 0 {
		tcpPort = 30303
	}
	udpPort := n.UDP()
	if udpPort == 0 {
		udpPort = 30303
	}

	// Get the actual IP for the container
	ip, err := sim.ContainerNetworkIP(testSuite, network, node)
	if err != nil {
		return "", err
	}

	// Change IP with the real IP for the desired docker network
	fixedIP := enode.NewV4(n.Pubkey(), net.ParseIP(ip), tcpPort, udpPort)
	return fixedIP.URLv4(), nil
}

// ClientExec runs a command in a running client.
func (sim *Simulation) ClientExec(testSuite SuiteID, test TestID, nodeid string, cmd []string) (*ExecInfo, error) {
	var (
		url  = fmt.Sprintf("%s/testsuite/%d/test/%d/node/%s/exec", sim.url, testSuite, test, nodeid)
		req  = &simapi.ExecRequest{Command: cmd}
		resp *ExecInfo
	)
	err := post(url, req, &resp)
	return resp, err
}

// CreateNetwork sends a request to the hive server to create a docker network by
// the given name.
func (sim *Simulation) CreateNetwork(testSuite SuiteID, networkName string) error {
	url := fmt.Sprintf("%s/testsuite/%d/network/%s", sim.url, testSuite, networkName)
	return post(url, nil, nil)
}

// RemoveNetwork sends a request to the hive server to remove the given network.
func (sim *Simulation) RemoveNetwork(testSuite SuiteID, network string) error {
	url := fmt.Sprintf("%s/testsuite/%d/network/%s", sim.url, testSuite, network)
	return requestDelete(url)
}

// ConnectContainer sends a request to the hive server to connect the given
// container to the given network.
func (sim *Simulation) ConnectContainer(testSuite SuiteID, network, containerID string) error {
	url := fmt.Sprintf("%s/testsuite/%d/network/%s/%s", sim.url, testSuite, network, containerID)
	return post(url, nil, nil)
}

// DisconnectContainer sends a request to the hive server to disconnect the given
// container from the given network.
func (sim *Simulation) DisconnectContainer(testSuite SuiteID, network, containerID string) error {
	url := fmt.Sprintf("%s/testsuite/%d/network/%s/%s", sim.url, testSuite, network, containerID)
	return requestDelete(url)
}

// ContainerNetworkIP returns the IP address of a container on the given network. If the
// container ID is "simulation", it returns the IP address of the simulator container.
func (sim *Simulation) ContainerNetworkIP(testSuite SuiteID, network, containerID string) (string, error) {
	var (
		url  = fmt.Sprintf("%s/testsuite/%d/network/%s/%s", sim.url, testSuite, network, containerID)
		resp string
	)
	err := get(url, &resp)
	return resp, err
}

func (setup *clientSetup) postWithFiles(url string, result interface{}) error {
	var (
		pipeR, pipeW = io.Pipe()
		bufW         = bufio.NewWriter(pipeW)
		pipeErrCh    = make(chan error, 1)
		form         = multipart.NewWriter(bufW)
	)

	go func() (err error) {
		defer func() { pipeErrCh <- err }()
		defer pipeW.Close()

		// Write 'config' parameter first.
		fw, err := form.CreateFormField("config")
		if err != nil {
			return err
		}
		if err := json.NewEncoder(fw).Encode(&setup.config); err != nil {
			return err
		}

		// Now upload the files.
		for filename, open := range setup.files {
			fw, err := form.CreateFormFile(filename, filepath.Base(filename))
			if err != nil {
				return err
			}
			fileReader, err := open()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: upload error for %s: %v\n", filename, err)
				return err
			}
			_, copyErr := io.Copy(fw, fileReader)
			fileReader.Close()
			if copyErr != nil {
				return copyErr
			}
		}

		// Form must be closed or the request will be missing the terminating boundary.
		if err := form.Close(); err != nil {
			return err
		}
		return bufW.Flush()
	}()

	// Send the request.
	req, err := http.NewRequest("POST", url, pipeR)
	if err != nil {
		return err
	}
	req.Header.Set("content-type", form.FormDataContentType())
	httpErr := request(req, result)

	// Wait for the uploader goroutine to finish.
	uploadErr := <-pipeErrCh
	if httpErr == nil && uploadErr != nil {
		return uploadErr
	}
	return httpErr
}

func get(url string, result interface{}) error {
	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(fmt.Errorf("can't create HTTP request: %v", err))
	}
	return request(httpReq, result)
}

func requestDelete(url string) error {
	httpReq, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		panic(fmt.Errorf("can't create HTTP request: %v", err))
	}
	return request(httpReq, nil)
}

func post(url string, requestObj interface{}, result interface{}) error {
	var reqBody []byte
	if requestObj != nil {
		var err error
		if reqBody, err = json.Marshal(requestObj); err != nil {
			panic(fmt.Errorf("error encoding request body: %v", err))
		}
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		panic(fmt.Errorf("can't create HTTP request: %v", err))
	}
	if len(reqBody) > 0 {
		httpReq.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(reqBody)), nil
		}
		httpReq.Header.Set("content-type", "application/json")
	}
	return request(httpReq, result)
}

func request(httpReq *http.Request, result interface{}) error {
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)

	switch {
	case resp.StatusCode >= 400:
		// It's an error response.
		switch resp.Header.Get("content-type") {
		case "application/json":
			var errobj simapi.Error
			if err := dec.Decode(&errobj); err != nil {
				return fmt.Errorf("request failed (status %d) and can't decode error message: %v", resp.StatusCode, err)
			}
			return errors.New(errobj.Error)
		default:
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			if len(respBody) == 0 {
				return fmt.Errorf("request failed (status %d)", resp.StatusCode)
			}
			return fmt.Errorf("request failed (status %d): %s", resp.StatusCode, respBody)
		}
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		// Request was successful.
		if result != nil {
			if err := dec.Decode(result); err != nil {
				return fmt.Errorf("invalid response (status %d): %v", resp.StatusCode, err)
			}
		}
		return nil
	default:
		// 1xx and 3xx should never happen.
		return fmt.Errorf("invalid response status code %d", resp.StatusCode)
	}
}
