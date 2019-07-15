package hive

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

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

var hostProxy *host
var once sync.Once

// GetInstance returns the instance of a proxy to the Hive simulator host, giving a single opportunity to configure it.
// The configuration is supplied as a byte representation, obtained from a file usually.
// Clients are generated as docker instances.
func GetInstance(config []byte) common.TestSuiteHost {

	once.Do(func() {
		var result HostConfiguration
		json.Unmarshal(config, &result)

		hostProxy = &host{
			configuration: &result
			//TODO - output stream
		}

	})
	return hostProxy
}

//GetClientEnode Get the client enode for the specified container id
func (sim *host) GetClientEnode(test common.TestID, node string) (*string, error) {
	resp, err := http.Get(sim.configuration.HostURI + "/enodes/" + test.String() + "/" + node)
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
func (sim *host) EndTest(test common.TestID, summaryResult *common.TestResult, clientResults map[string]*common.TestResult) error {

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
	vals.Add("testcase", test.String())
	vals.Add("summaryresult", string(summaryResultData))
	vals.Add("clientresults", string(clientResultData))
	_, err = wrapHTTPErrorsPost(sim.configuration.HostURI+"/tests", vals)
	if err != nil {
		return err
	}

	return err
}

func (sim *host) StartTestSuite(name string, description string) common.TestSuiteID {
	//TODO
	return 0
}

func (sim *host) EndTestSuite(testSuite common.TestSuiteID) error {
	//TODO
	return nil
}

//StartTest starts a new test case, returning the testcase id as a context identifier
func (sim *host) StartTest(testSuiteRun common.TestSuiteID, name string, description string) (common.TestID, error) {
	var testID common.TestID
	resp, err := http.Get(sim.configuration.HostURI + "/tests")
	if err != nil {
		return testID, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return testID, err
	}

	err = json.Unmarshal(body, &testID)
	if err != nil {
		return testID, err
	}
	return testID, nil
}

//GetClientTypes Get all client types available to this simulator run
//this depends on both the available client set
//and the command line filters
func (sim *host) GetClientTypes() (availableClients []string, err error) {
	resp, err := http.Get(sim.configuration.HostURI + "/clients")
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

//StartNewNode Start a new node (or other container) with the specified parameters
//One parameter must be named CLIENT and should contain one of the
//returned client types from GetClientTypes
//The input is used as environment variables in the new container
//Returns container id and ip
func (sim *host) GetNode(test common.TestID, parameters map[string]string) (string, net.IP, *string, error) {
	vals := make(url.Values)
	for k, v := range parameters {
		vals.Add(k, v)
	}
	vals.Add("testcase", test.String())
	data, err := wrapHTTPErrorsPost(sim.configuration.HostURI+"/nodes", vals)
	if err != nil {
		return "", nil, nil, err
	}
	if idip := strings.Split(data, "@"); len(idip) > 1 {
		return idip[0], net.ParseIP(idip[1]), &idip[2], nil
	}
	return data, net.IP{}, nil, fmt.Errorf("no ip address returned: %v", data)
}

//StartNewPseudo Start a new pseudo-client with the specified parameters
//One parameter must be named CLIENT and should contain one of the
//returned client types from GetClientTypes
//The input is used as environment variables in the new container
//Returns container id and ip
func (sim *host) GetPseudo(test common.TestID, parameters map[string]string) (string, net.IP, *string, error) {
	vals := make(url.Values)
	for k, v := range parameters {
		vals.Add(k, v)
	}
	vals.Add("testcase", test.String())
	data, err := wrapHTTPErrorsPost(sim.configuration.HostURI+"/pseudos", vals)
	if err != nil {
		return "", nil, nil, err
	}
	if idip := strings.Split(data, "@"); len(idip) > 1 {
		return idip[0], net.ParseIP(idip[1]), &idip[2], nil
	}
	return data, net.IP{}, nil, fmt.Errorf("no ip address returned: %v", data)
}

// KillNode signals to the host that the node is no longer required
func (sim *host) KillNode(test common.TestID, nodeid string) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/delete/%s/%s", test.String(), sim.configuration.HostURI, nodeid), nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
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
