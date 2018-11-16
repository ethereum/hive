package common

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"github.com/fsouza/go-dockerclient"
)

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
	//Returns container id
	StartNewNode(map[string]string) (*string, error)
	//Submit log info to the simulator log
	Log(string) error
	//Submit node test results
	//	Success flag
	//	Node id
	//  Details
	AddResults(bool, string, string, string, string) error
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

	ip := string(body)

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

	res := string(body)

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
//Returns container id
func (sim SimulatorHost) StartNewNode(parms map[string]string) (*string, error) {

	vals := make(url.Values)
	for k, v := range parms {
		vals.Add(k, v)
	}

	resp, err := http.PostForm(*sim.HostURI+"/nodes", vals)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	res := string(body)
	return &res, nil

}

//Log Submit log info to the simulator log
func (sim SimulatorHost) Log(string) error {
	//TODO
	return nil
}

//AddResults //Submit node test results
//	Success flag
//	Node id
//  Details
func (sim SimulatorHost) AddResults(success bool, nodeID string, details string, name string, errMsg string) error {
	vals := make(url.Values)

	vals.Add("success", strconv.FormatBool(success))
	vals.Add("details", details)
	vals.Add("nodeid", nodeID)
	vals.Add("details", details)
	vals.Add("error", errMsg)

	_, err := http.PostForm(*sim.HostURI+"/nodes", vals)

	return err

}

//KillNode Stop and delete the specified container
func (sim SimulatorHost) KillNode(string) error {
	return nil
}
