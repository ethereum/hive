package common

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

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
	AddResults(bool, string, string) error
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
func (sim SimulatorHost) GetClientIP(string) (*string, error) {
	//TODO
	return nil, nil
}

//GetClientEnode Get the client enode for the specified container id
func (sim SimulatorHost) GetClientEnode(string) (*string, error) {
	//calls request to create node, which returns node local identifier, and
	//should be updated to a) copy the enode.sh to the local container
	//add client api function to get enode id

	return nil, nil
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
func (sim SimulatorHost) StartNewNode(map[string]string) (*string, error) {
	return nil, nil
}

//Log Submit log info to the simulator log
func (sim SimulatorHost) Log(string) error {
	return nil
}

//AddResults //Submit node test results
//	Success flag
//	Node id
//  Details
func (sim SimulatorHost) AddResults(bool, string, string) error {
	return nil
}

//KillNode Stop and delete the specified container
func (sim SimulatorHost) KillNode(string) error {
	return nil
}
