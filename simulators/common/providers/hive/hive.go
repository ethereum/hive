package hive

import (
	"bytes"
	"encoding/json"
	"fmt"

	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/ethereum/hive/simulators/common"
)

// HostConfiguration describes the json configuration for
// the Hive simulator Host, which uses Docker to manage simulated
// clients.
type HostConfiguration struct {
	HostURI string `json:"hostURI"`
}

type host struct {
	configuration *HostConfiguration
	outputStream  io.Writer
}

// New looks up the hive host URI using the HIVE_SIMULATOR environment variable
// and returns a new common.TestSuiteHost. It will panic if HIVE_SIMULATOR is not
// set.
func New() common.TestSuiteHost {
	simulator, isSet := os.LookupEnv("HIVE_SIMULATOR")
	if !isSet {
		panic("HIVE_SIMULATOR environment variable not set")
	}

	return &host{
		configuration: &HostConfiguration{
			HostURI: simulator,
		},
	}
}

// CreateNetwork sends a request to the hive server to create a docker network by
// the given name.
func (sim *host) CreateNetwork(testSuite common.TestSuiteID, networkName string) (string, error) {
	resp, err := http.Post(fmt.Sprintf("%s/testsuite/%s/network/%s", sim.configuration.HostURI, testSuite.String(), networkName), "application/json", nil)
	if err != nil {
		return "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// ConnectContainerToNetwork sends a request to the hive server to connect the given
// container to the given network.
func (sim *host) ConnectContainerToNetwork(testSuite common.TestSuiteID, networkName, containerName string) error {
	endpoint := fmt.Sprintf("%s/testsuite/%s/node/%s/network/%s", sim.configuration.HostURI, testSuite, containerName, networkName)
	resp, err := http.Post(endpoint, "application/json", nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { // TODO better err check?
		return fmt.Errorf("error posting connect container to network request, status code %d", resp.StatusCode)
	}
	return nil
}

// GetContainerNetworkIP sends a request to the hive server to get the IP address
// of the given container on the given network.
func (sim *host) GetContainerNetworkIP(testSuite common.TestSuiteID, networkID, node string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("%s/testsuite/%s/network/%s/node/%s", sim.configuration.HostURI, testSuite.String(), networkID, node))
	if err != nil {
		return "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

//GetClientEnode Get the client enode for the specified node id
func (sim *host) GetClientEnode(testSuite common.TestSuiteID, test common.TestID, node string) (*string, error) {
	resp, err := http.Get(fmt.Sprintf("%s/testsuite/%s/test/%s/node/%s", sim.configuration.HostURI, testSuite.String(), test.String(), node))
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	res := strings.TrimRight(string(body), "\r\n")
	return &res, nil
}

// EndTest finishes the test case, cleaning up everything, logging results, and returning an error if the process could not be completed
func (sim *host) EndTest(testSuite common.TestSuiteID, test common.TestID, summaryResult *common.TestResult, clientResults map[string]*common.TestResult) error {

	// post results (which deletes the test case - because DELETE message body is not always supported)
	summaryResultData, err := json.Marshal(summaryResult)
	if err != nil {
		return err
	}
	clientResultData, err := json.Marshal(clientResults)
	if err != nil {
		return err
	}

	vals := make(url.Values)

	vals.Add("summaryresult", string(summaryResultData))
	vals.Add("clientresults", string(clientResultData))
	_, err = wrapHTTPErrorsPost(fmt.Sprintf("%s/testsuite/%s/test/%s", sim.configuration.HostURI, testSuite.String(), test.String()), vals)

	return err
}

func (sim *host) StartTestSuite(name, description, simlog string) (common.TestSuiteID, error) {
	vals := make(url.Values)
	vals.Add("name", name)
	vals.Add("description", description)
	vals.Add("simlog", simlog)
	idstring, err := wrapHTTPErrorsPost(fmt.Sprintf("%s/testsuite", sim.configuration.HostURI), vals)
	if err != nil {
		return 0, err
	}
	id, err := strconv.Atoi(idstring)
	if err != nil {
		return 0, err
	}
	return common.TestSuiteID(id), nil
}

func (sim *host) EndTestSuite(testSuite common.TestSuiteID) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/testsuite/%s", sim.configuration.HostURI, testSuite.String()), nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
}

//StartTest starts a new test case, returning the testcase id as a context identifier
func (sim *host) StartTest(testSuite common.TestSuiteID, name string, description string) (common.TestID, error) {
	vals := make(url.Values)
	vals.Add("name", name)
	vals.Add("description", description)

	idstring, err := wrapHTTPErrorsPost(fmt.Sprintf("%s/testsuite/%s/test", sim.configuration.HostURI, testSuite.String()), vals)
	if err != nil {
		return 0, err
	}
	testID, err := strconv.Atoi(idstring)
	if err != nil {
		return 0, err
	}
	return common.TestID(testID), nil
}

//GetClientTypes Get all client types available to this simulator run
//this depends on both the available client set
//and the command line filters
func (sim *host) GetClientTypes() (availableClients []string, err error) {
	resp, err := http.Get(fmt.Sprintf("%s/clients", sim.configuration.HostURI))
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

//GetNode starts a new node (or other container) with the specified parameters
//One parameter must be named CLIENT and should contain one of the
//returned client types from GetClientTypes
//The input is used as environment variables in the new container
//Returns container id and ip
func (sim *host) GetNode(testSuite common.TestSuiteID, test common.TestID, parameters map[string]string, initFiles map[string]string) (string, net.IP, *string, error) {
	data, err := postWithFiles(fmt.Sprintf("%s/testsuite/%s/test/%s/node", sim.configuration.HostURI, testSuite.String(), test.String()), parameters, initFiles)
	if err != nil {
		return "", nil, nil, err
	}
	if idip := strings.Split(data, "@"); len(idip) > 1 {
		return idip[0], net.ParseIP(idip[1]), &idip[2], nil
	}
	return data, net.IP{}, nil, fmt.Errorf("no ip address returned: %v", data)
}

// GetSimContainerID sends a request to the hive server to get the
// container ID of the simulation container.
func (sim *host) GetSimContainerID(testSuite common.TestSuiteID) (string, error) {
	resp, err := http.Get(fmt.Sprintf("%s/testsuite/%s/simulator", sim.configuration.HostURI, testSuite))
	if err != nil {
		return "", err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

//GetPseudo starts a new pseudo-client with the specified parameters
//One parameter must be named CLIENT and should contain one of the
//returned client types from GetClientTypes
//The input is used as environment variables in the new container
//Returns container id , ip and mac
func (sim *host) GetPseudo(testSuite common.TestSuiteID, test common.TestID, parameters map[string]string) (string, net.IP, *string, error) {
	vals := make(url.Values)
	for k, v := range parameters {
		vals.Add(k, v)
	}
	//vals.Add("testcase", test.String())
	data, err := wrapHTTPErrorsPost(fmt.Sprintf("%s/testsuite/%s/test/%s/pseudo", sim.configuration.HostURI, testSuite.String(), test.String()), vals)
	if err != nil {
		return "", nil, nil, err
	}
	if idip := strings.Split(data, "@"); len(idip) > 1 {
		return idip[0], net.ParseIP(idip[1]), &idip[2], nil
	}
	return data, net.IP{}, nil, fmt.Errorf("no ip address returned: %v", data)
}

// KillNode signals to the host that the node is no longer required
func (sim *host) KillNode(testSuite common.TestSuiteID, test common.TestID, nodeid string) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/testsuite/%s/test/%s/node/%s", sim.configuration.HostURI, testSuite.String(), test.String(), nodeid), nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
}

func postWithFiles(url string, values map[string]string, files map[string]string) (string, error) {
	var err error
	//make a dictionary of readers
	formValues := make(map[string]io.Reader)
	for key, s := range values {
		formValues[key] = strings.NewReader(s)
	}
	for key, filename := range files {

		filereader, err := os.Open(filename)
		if err != nil {
			return "", err
		}
		//fi, err := filereader.Stat()

		formValues[key] = filereader
	}

	//send them
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for key, r := range formValues {

		var fw io.Writer
		if x, ok := r.(io.Closer); ok {
			defer x.Close()
		}
		if x, ok := r.(*os.File); ok {

			if fw, err = w.CreateFormFile(key, x.Name()); err != nil {
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
