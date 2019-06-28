package hive

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

type HiveHostConfiguration struct {
	hostURI string `json:"hostURI"`
}

var hostProxy *HiveHostConfiguration
var once sync.Once

// GetInstance returns the instance of a proxy to the Hive simulator host, giving a single opportunity to configure it.
// The configuration is supplied as a byte representation, obtained from a file usually.
// Clients are generated as docker instances.
func GetInstance(config []byte) TestSuiteHost {

	once.Do(func() {
		var result HiveHostConfiguration
		json.Unmarshal(config, &result)
		hostProxy = &result

	})
	return hostProxy
}

//GetClientEnode Get the client enode for the specified container id
func (sim hiveHost) GetClientEnode(test TestCase, node string) (*string, error) {
	resp, err := http.Get(*sim.HostURI + "/enodes/" + strconv.Itoa(test.ID()) + "/" + node)
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
func (sim hiveHost) EndTest(test int, summaryResult TestResult, clientResults map[string]TestResult) error {

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
	vals.Add("testcase", strconv.Itoa(test))
	vals.Add("summaryresult", string(summaryResultData))
	vals.Add("clientresults", string(clientResultData))
	_, err = wrapHTTPErrorsPost(*sim.HostURI+"/tests", vals)
	if err != nil {
		return err
	}

	return err
}

//StartTest starts a new test case, returning the testcase id as a context identifier
func (sim hiveHost) StartTest(name string, description string) (int, error) {
	var testID int
	resp, err := http.Get(*sim.HostURI + "/tests")
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
func (sim hiveHost) GetClientTypes() (availableClients []string, err error) {
	resp, err := http.Get(*sim.HostURI + "/clients")
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
func (sim hiveHost) GetNode(test int, parms map[string]string) (string, net.IP, string, error) {
	vals := make(url.Values)
	for k, v := range parms {
		vals.Add(k, v)
	}
	vals.Add("testcase", strconv.Itoa(test))
	data, err := wrapHTTPErrorsPost(*sim.HostURI+"/nodes", vals)
	if err != nil {
		return "", nil, "", err
	}
	if idip := strings.Split(data, "@"); len(idip) > 1 {
		return idip[0], net.ParseIP(idip[1]), idip[2], nil
	}
	return data, net.IP{}, "", fmt.Errorf("no ip address returned: %v", data)
}

//StartNewPseudo Start a new pseudo-client with the specified parameters
//One parameter must be named CLIENT and should contain one of the
//returned client types from GetClientTypes
//The input is used as environment variables in the new container
//Returns container id and ip
func (sim hiveHost) GetPseudo(test int, parms map[string]string) (string, net.IP, string, error) {
	vals := make(url.Values)
	for k, v := range parms {
		vals.Add(k, v)
	}
	vals.Add("testcase", strconv.Itoa(test))
	data, err := wrapHTTPErrorsPost(*sim.HostURI+"/pseudos", vals)
	if err != nil {
		return "", nil, "", err
	}
	if idip := strings.Split(data, "@"); len(idip) > 1 {
		return idip[0], net.ParseIP(idip[1]), idip[2], nil
	}
	return data, net.IP{}, "", fmt.Errorf("no ip address returned: %v", data)
}

// KillNode signals to the host that the node is no longer required
func (sim hiveHost) KillNode(test int, nodeid string) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/delete/%s/%s", strconv.Itoa(test), *sim.HostURI, nodeid), nil)
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
