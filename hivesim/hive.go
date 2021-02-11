package hivesim

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Simulation wraps the simulation HTTP API provided by hive.
type Simulation struct {
	url string
}

// New looks up the hive host URI using the HIVE_SIMULATOR environment variable
// and connects to it. It will panic if HIVE_SIMULATOR is not set.
func New() *Simulation {
	simulator, isSet := os.LookupEnv("HIVE_SIMULATOR")
	if !isSet {
		panic("HIVE_SIMULATOR environment variable not set")
	}
	return &Simulation{url: simulator}
}

// NewAt creates a simulation connected to the given API endpoint. You'll will rarely need
// to use this. In simulations launched by hive, use New() instead.
func NewAt(url string) *Simulation {
	return &Simulation{url: url}
}

// EndTest finishes the test case, cleaning up everything, logging results, and returning
// an error if the process could not be completed.
func (sim *Simulation) EndTest(testSuite SuiteID, test TestID, summaryResult TestResult) error {
	// post results (which deletes the test case - because DELETE message body is not always supported)
	summaryResultData, err := json.Marshal(summaryResult)
	if err != nil {
		return err
	}

	vals := make(url.Values)
	vals.Add("summaryresult", string(summaryResultData))

	_, err = wrapHTTPErrorsPost(fmt.Sprintf("%s/testsuite/%d/test/%d", sim.url, testSuite, test), vals)
	return err
}

// StartSuite signals the start of a test suite.
func (sim *Simulation) StartSuite(name, description, simlog string) (SuiteID, error) {
	vals := make(url.Values)
	vals.Add("name", name)
	vals.Add("description", description)
	vals.Add("simlog", simlog)
	idstring, err := wrapHTTPErrorsPost(fmt.Sprintf("%s/testsuite", sim.url), vals)
	if err != nil {
		return 0, err
	}
	id, err := strconv.Atoi(idstring)
	if err != nil {
		return 0, err
	}
	return SuiteID(id), nil
}

// EndSuite signals the end of a test suite.
func (sim *Simulation) EndSuite(testSuite SuiteID) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/testsuite/%d", sim.url, testSuite), nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
}

// StartTest starts a new test case, returning the testcase id as a context identifier.
func (sim *Simulation) StartTest(testSuite SuiteID, name string, description string) (TestID, error) {
	vals := make(url.Values)
	vals.Add("name", name)
	vals.Add("description", description)

	idstring, err := wrapHTTPErrorsPost(fmt.Sprintf("%s/testsuite/%d/test", sim.url, testSuite), vals)
	if err != nil {
		return 0, err
	}
	testID, err := strconv.Atoi(idstring)
	if err != nil {
		return 0, err
	}
	return TestID(testID), nil
}

// ClientMetadata is part of the ClientDefinition and lists metadata
type ClientMetadata struct {
	Roles []string `yaml:"roles" json:"roles"`
}

// ClientDefinition is served by the /clients API endpoint to list the available clients
type ClientDefinition struct {
	Name    string         `json:"name"`
	Version string         `json:"version"`
	Meta    ClientMetadata `json:"meta"`
}

func (m *ClientDefinition) HasRole(role string) bool {
	for _, m := range m.Meta.Roles {
		if m == role {
			return true
		}
	}
	return false
}

// ClientTypes returns all client types available to this simulator run. This depends on
// both the available client set and the command line filters.
func (sim *Simulation) ClientTypes() (availableClients []*ClientDefinition, err error) {
	resp, err := http.Get(fmt.Sprintf("%s/clients?metadata=1", sim.url))
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, &availableClients)
	if err != nil {
		return nil, err
	}
	return
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
	setup := &clientSetup{
		parameters: make(map[string]string),
		files:      make(map[string]func() (io.ReadCloser, error)),
	}
	setup.parameters["CLIENT"] = clientType
	for _, opt := range options {
		opt.Apply(setup)
	}
	data, err := setup.postWithFiles(fmt.Sprintf("%s/testsuite/%d/test/%d/node", sim.url, testSuite, test))
	if err != nil {
		return "", nil, err
	}
	if idip := strings.Split(data, "@"); len(idip) >= 1 {
		return idip[0], net.ParseIP(idip[1]), nil
	}
	return data, net.IP{}, fmt.Errorf("no ip address returned: %v", data)
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

// ClientEnodeURL returns the enode URL of a running client.
func (sim *Simulation) ClientEnodeURL(testSuite SuiteID, test TestID, node string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("%s/testsuite/%d/test/%d/node/%s", sim.url, testSuite, test, node))
	if err != nil {
		return "", err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	res := strings.TrimRight(string(body), "\r\n")
	return res, nil
}

// CreateNetwork sends a request to the hive server to create a docker network by
// the given name.
func (sim *Simulation) CreateNetwork(testSuite SuiteID, networkName string) error {
	_, err := http.Post(fmt.Sprintf("%s/testsuite/%d/network/%s", sim.url, testSuite, networkName), "application/json", nil)
	return err
}

// RemoveNetwork sends a request to the hive server to remove the given network.
func (sim *Simulation) RemoveNetwork(testSuite SuiteID, network string) error {
	endpoint := fmt.Sprintf("%s/testsuite/%d/network/%s", sim.url, testSuite, network)
	req, err := http.NewRequest(http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
}

// ConnectContainer sends a request to the hive server to connect the given
// container to the given network.
func (sim *Simulation) ConnectContainer(testSuite SuiteID, network, containerID string) error {
	endpoint := fmt.Sprintf("%s/testsuite/%d/network/%s/%s", sim.url, testSuite, network, containerID)
	_, err := http.Post(endpoint, "application/json", nil)
	return err
}

// DisconnectContainer sends a request to the hive server to disconnect the given
// container from the given network.
func (sim *Simulation) DisconnectContainer(testSuite SuiteID, network, containerID string) error {
	endpoint := fmt.Sprintf("%s/testsuite/%d/network/%s/%s", sim.url, testSuite, network, containerID)
	req, err := http.NewRequest(http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
}

// ContainerNetworkIP returns the IP address of a container on the given network. If the
// container ID is "simulation", it returns the IP address of the simulator container.
func (sim *Simulation) ContainerNetworkIP(testSuite SuiteID, network, containerID string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("%s/testsuite/%d/network/%s/%s", sim.url, testSuite, network, containerID))
	if err != nil {
		return "", err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// collects client setup options
type clientSetup struct {
	parameters map[string]string
	// destination path -> open data function
	files map[string]func() (io.ReadCloser, error)
}

type StartOption interface {
	Apply(setup *clientSetup)
}

type StartOptionFn func(setup *clientSetup)

func (fn StartOptionFn) Apply(setup *clientSetup) {
	fn(setup)
}

// Bundle combines start options, e.g. to bundle files together as option.
func Bundle(option ...StartOption) StartOption {
	return StartOptionFn(func(setup *clientSetup) {
		for _, opt := range option {
			opt.Apply(setup)
		}
	})
}

func fileAsSrc(path string) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		return os.Open(path)
	}
}

// WithStaticFiles adds files from the local filesystem to the client. Map: destination file path -> source file path.
func WithStaticFiles(initFiles map[string]string) StartOption {
	return StartOptionFn(func(setup *clientSetup) {
		for k, v := range initFiles {
			setup.files[k] = fileAsSrc(v)
		}
	})
}

// WithDynamicFile adds a file to a client, sourced dynamically from the given src function,
// called upon usage of the returned StartOption.
//
// A StartOption, and thus the src function, should be reusable and safe to use in parallel.
// Dynamic files can override static file sources (see WithStaticFiles) and vice-versa.
func WithDynamicFile(dstPath string, src func() (io.ReadCloser, error)) StartOption {
	return StartOptionFn(func(setup *clientSetup) {
		setup.files[dstPath] = src
	})
}

func (setup *clientSetup) postWithFiles(url string) (string, error) {
	var err error

	// make a dictionary of readers
	formValues := make(map[string]io.Reader)
	for key, s := range setup.parameters {
		formValues[key] = strings.NewReader(s)
	}
	for key, src := range setup.files {
		filereader, err := src()
		if err != nil {
			return "", err
		}
		formValues[key] = filereader
	}

	// send them
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for key, r := range formValues {
		var fw io.Writer
		if x, ok := r.(io.Closer); ok {
			defer x.Close()
		}
		if _, ok := setup.files[key]; ok {
			if fw, err = w.CreateFormFile(key, filepath.Base(key)); err != nil {
				return "", err
			}
		} else {
			if fw, err = w.CreateFormField(key); err != nil {
				return "", err
			}
		}
		if _, err = io.Copy(fw, r); err != nil {
			return "", err
		}
	}

	// this must be closed or the request will be missing the terminating boundary
	w.Close()

	// Can't use http.PostForm because we need to change the content header
	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		return "", err
	}
	// Set the content type, this will contain the boundary.
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Submit the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 200 && resp.StatusCode <= 300 {
		return string(body), nil
	}
	return "", fmt.Errorf("request failed (%d): %v", resp.StatusCode, string(body))
}

// wrapHttpErrorsPost wraps http.PostForm to convert responses that are not 200 OK into errors
func wrapHTTPErrorsPost(url string, data url.Values) (string, error) {
	resp, err := http.PostForm(url, data)
	if err != nil {
		return "", err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 200 && resp.StatusCode <= 300 {
		return string(body), nil
	}
	return "", fmt.Errorf("request failed (%d): %v", resp.StatusCode, string(body))
}
