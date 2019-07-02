package local

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"math"
)

// LocalHostConfiguration is used to set up the local provider.
// It describes pre-supplied nodes. During tests and when nodes are requested,
// these pre-supplied nodes are selected
// according to the following rules:
// 1. Does the request general type (client/pseudo) match?
// 2. Does the type match? (Geth/Parity/Nethermind/Etc)
// 3. Does a configuration parameter exist in the supplied descriptor that is also
// in the request descriptor, and do they match?
// If multiple nodes are pre-supplied that fulfil requests, these are selected in round-robin
// method.
//
type LocalHostConfiguration struct {
	AvailableHosts []HostDescription `json:"hostDescription"`
}

type HostDescription struct {
	IsPseudo   bool              `json:"isPseudo"`
	ClientType string            `json:"clientType"`
	Parameters map[string]string `json:"parameters,omitempty"`
	Enode      string            `json:"Enode,omitempty"`
	IP         net.IP            `json:"IP"`
	Mac		   string			 `json:"Mac"`
	selectedCount int
}

type localHost struct {
	configuration *LocalHostConfiguration
	clientsByType  map[string][]int
	clientTypes []string
	mux sync.Mutex
}

var hostProxy *localHost
var once sync.Once

// GetLocalInstance returns the instance of a local provider, which uses presupplied node instances and creates logs to a local destination,
// and provides a single opportunity to configure it during initialisation.
// The configuration is supplied as a byte representation, obtained from a file usually.
func GetInstance(config []byte) TestSuiteHost {

	once.Do(func() {


		var result LocalHostConfiguration
		json.Unmarshal(config, &result)
		
		clientsByType, clientTypes = mapClients(result)

		hostProxy = &localHost{
			configuration : result,
			clientsByType: clientsByType
			clientTypes: clientTypes
		}
		
	})
	return hostProxy
}

func mapClients()  (map[string][]int,[]string ){

	clientsByType:= make(map[string][]int)
	clientTypes:= make([]string],0)

	for i,v:= range(hostProxy.configuration.AvailableHosts){
		clientsByType[v.ClientType]= append(clientsByType[v.ClientType],i)
	}

	for k,v := range(clientsByType){
		clientTypes= append(clientTypes,k)
	}

	return clientsByType,clientTypes
}



//GetClientEnode Get the client enode for the specified node id, which in this case is just the index
func (sim localHost) GetClientEnode(test TestCase, node string) (*string, error) {
	//local nodes are identified by their index
	nodeIndex, err := strconv.Atoi(node)
	if err != nil {
		return _, err
	}
	// make sure it is within the bounds of the node list
	if nodeIndex < 0 || nodeIndex > len(AvailableHosts) {
		return _, errors.New("no such node")
	}
	//return the enode
	return hostProxy.configuration.AvailableHosts[nodeIndex].Enode, nil
}

// EndTest finishes the test case, cleaning up everything, logging results, and returning an error if the process could not be completed
func (sim localHost) EndTest(test int, summaryResult TestResult, clientResults map[string]TestResult) error {

//TODO ---------------


	//TODO ----------------



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
func (sim localHost) StartTest(name string, description string) (int, error) {

//TODO ---------------


	//TODO ----------------




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
func (sim localHost) GetClientTypes() (availableClients []string, err error) {
	return hostProxy.clientTypes
}

//StartNewNode attempts to acquire a new node matching the given parameters
//One parameter must be named CLIENT and should be a known type returned by GetClientTypes
//If there are multiple nodes, they will be selected round-robin  
//Returns container id, ip, mac
func (sim localHost) GetNode(test int, parms map[string]string) (string, net.IP, string, error) {
	hostProxy.mux.Lock()
	defer hostProxy.mux.Unlock()

	client,ok:= parms["CLIENT"]
	if !ok {
		return _,_,_,errors.New("Unknown client")
	}

	availableClients,ok:= hostProxy.clientsByType[client]
	if !ok || len(availableClients)==0 {
		return _,_,_,errors.New("No available clients")
	}

	leastUsedCt:= math.MaxUint32
	var leastUsed *HostDescription
	var leastUsedIndex int
	for _,v := range(availableClients){
		node:= &hostProxy.configuration.AvailableHosts[v]
		if node.selectedCount < leastUsedCt{
			leastUsed = node
			leastUserCt= node.selectedCount
			leastUsedIndex= v
		}
	}

	
	return strconv.Itoa(leastUsedIndex),leastUsed.IP,leastUsed.Mac,nil
	

}

// GetPseudo
func (sim localHost) GetPseudo(test int, parms map[string]string) (string, net.IP, string, error) {
	//TODO ---------------


	//TODO ----------------


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
func (sim localHost) KillNode(test int, nodeid string) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/delete/%s/%s", strconv.Itoa(test), *sim.HostURI, nodeid), nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
}
