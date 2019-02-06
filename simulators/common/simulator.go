package common

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fsouza/go-dockerclient"
)

//MacENREntry a type of ENR record for holding mac addresses
type MacENREntry string

//ENRKey the key for this type of ENR record
func (v MacENREntry) ENRKey() string { return "mac" }

//SimulatorHost A simulator host
type SimulatorHost struct {
	HostURI *string
}

//SimulatorAPI The simulator host remote API
type SimulatorAPI interface {
	//Retrieve docker daemon info
	GetDockerInfo() (*docker.DockerInfo, error)
	//Get a specific client's IP
	GetClientIP(string) (*string, error)
	//Get a specific client's enode
	GetClientEnode(string) (*string, error)
	//Get all client types available to this simulator run
	//this depends on both the available client set
	//and the command line filters
	GetClientTypes() ([]string, error)
	//Start a new node (or other container) with the specified parameters
	//One parameter must be named CLIENT and should contain one of the
	//returned client types from GetClientTypes
	//The input is used as environment variables in the new container
	//Returns container id, ip and mac
	StartNewNode(map[string]string) (string, net.IP, string, error)
	//Submit log info to the simulator log
	Log(string) error
	//Submit node test results
	//	Success flag
	//	Node id
	//  Details
	AddResults(success bool, nodeID string, name string, errMsg string, duration time.Duration) error
	//Stop and delete the specified container
	KillNode(string) error
}

//Logger a general logger interface
type Logger interface {
	Log(args ...interface{})
	Logf(format string, args ...interface{})
}

//GetDockerInfo Get the host's docker daemon info
func (sim SimulatorHost) GetDockerInfo() (*docker.DockerInfo, error) {
	//TODO
	d := &docker.DockerInfo{}

	return d, nil
}

//GetClientIP Get the client IP
func (sim SimulatorHost) GetClientIP(node string) (*string, error) {

	resp, err := http.Get(*sim.HostURI + "/nodes/" + node)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	ip := strings.TrimRight(string(body), "\r\n")

	return &ip, nil
}

//GetClientEnode Get the client enode for the specified container id
func (sim SimulatorHost) GetClientEnode(node string) (*string, error) {
	resp, err := http.Get(*sim.HostURI + "/enodes/" + node)
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

//GetClientTypes Get all client types available to this simulator run
//this depends on both the available client set
//and the command line filters
func (sim SimulatorHost) GetClientTypes() (availableClients []string, err error) {
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
func (sim SimulatorHost) StartNewNode(parms map[string]string) (string, net.IP, string, error) {
	vals := make(url.Values)
	for k, v := range parms {
		vals.Add(k, v)
	}
	data, err := wrapHttpErrorsPost(*sim.HostURI+"/nodes", vals)
	if err != nil {
		return "", nil, "", err
	}
	if idip := strings.Split(data, "@"); len(idip) > 1 {
		return idip[0], net.ParseIP(idip[1]), idip[2], nil
	}
	return data, net.IP{}, "", fmt.Errorf("no ip address returned: %v", data)
}

//Log Submit log info to the simulator log
func (sim SimulatorHost) Log(string) error {
	//TODO
	return nil
}

type resultDetails struct {
	Instanceid string   `json:"instanceid"`
	Errors     []string `json:"errors"`
	Ms         float64  `json:"ms"`
}

// wrapHttpErrorsPost wraps http.PostForm to convert responses that are not 200 OK into errors
func wrapHttpErrorsPost(url string, data url.Values) (string, error) {

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

//AddResults //Submit node test results
//	Success flag
//	Node id
//  Details
func (sim SimulatorHost) AddResults(success bool, nodeID string, name string, errMsg string, duration time.Duration) error {
	vals := make(url.Values)

	vals.Add("success", strconv.FormatBool(success))
	vals.Add("nodeid", nodeID)
	vals.Add("name", name)

	details := &resultDetails{
		Instanceid: nodeID,
		Ms:         float64(duration) / float64(time.Millisecond),
	}

	if len(errMsg) > 0 {
		vals.Add("error", errMsg)
		details.Errors = []string{errMsg}
	}

	detailsBytes, err := json.Marshal(details)
	if err != nil {
		return err
	}
	vals.Add("details", string(detailsBytes))

	_, error := wrapHttpErrorsPost(*sim.HostURI+"/subresults", vals)
	return error

}

//KillNode Stop and delete the specified container
func (sim SimulatorHost) KillNode(nodeid string) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/delete/%s", *sim.HostURI, nodeid), nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
}
