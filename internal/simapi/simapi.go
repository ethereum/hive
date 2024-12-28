// Package simapi contains definitions of JSON objects used in the simulation API.
package simapi

type TestRequest struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Location    string `json:"location"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

// NodeConfig contains the launch parameters for a client container.
type NodeConfig struct {
	Client      string            `json:"client"`
	Networks    []string          `json:"networks"`
	Environment map[string]string `json:"environment"`
}

// StartNodeResponse is returned by the client startup endpoint.
type StartNodeResponse struct {
	ID string `json:"id"` // Container ID.
	IP string `json:"ip"` // IP address in bridge network
}

// NodeResponse is the description of a running client as returned by the API.
type NodeResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ExecRequest struct {
	Command []string `json:"command"`
}

type Error struct {
	Error string `json:"error"`
}
