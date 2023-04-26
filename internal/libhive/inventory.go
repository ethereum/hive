package libhive

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// argDelimiter is what separates the client name from its build arguments.
//
// All arguments start with a letter followed by a colon.
// The last argument, when no prefix is specified, is the branch or tag name.
// Supported prefixes are:
//
//	f: - docker file name (used to build the client image)
//	u: - user name (owner of git repository)
//	r: - repository name
//	b: - branch or tag name
//
// Examples:
//
//	besu_nightly -> client: besu, branch: nightly
//	besu_u:hyperledger_b:master -> client: besu, user: hyperledger, branch: master
//	go-ethereum_f:git -> client: go-ethereum, dockerfile: Dockerfile.git
const clientDelimiter = ","
const argDelimiter = "_"

type ClientBuildInfo struct {
	Name       string `json:"name"       yaml:"name"`
	DockerFile string `json:"dockerfile" yaml:"dockerfile"`
	User       string `json:"user"       yaml:"user"`
	Repo       string `json:"repo"       yaml:"repo"`
	TagBranch  string `json:"branch"     yaml:"branch"`
}

func (c ClientBuildInfo) String() string {
	s := c.Name
	if c.DockerFile != "" {
		s = s + "_" + c.DockerFile
	}
	if c.User != "" {
		s = s + "_" + c.User
	}
	if c.Repo != "" {
		s = s + "_" + c.Repo
	}
	if c.TagBranch != "" {
		s = s + "_" + c.TagBranch
	}
	return s
}

func (c *ClientBuildInfo) ParseSubstring(s string, isLast bool) {
	if strings.HasPrefix(s, "u:") {
		c.User = s[2:]
	} else if strings.HasPrefix(s, "r:") {
		c.Repo = s[2:]
	} else if strings.HasPrefix(s, "f:") {
		c.DockerFile = s[2:]
	} else if isLast || strings.HasPrefix(s, "b:") {
		// Last substring is the branch if it doesn't have a prefix.
		c.TagBranch = strings.TrimPrefix(s, "b:")
	} else {
		c.Name = c.Name + argDelimiter + s
	}
}

// Parses client build info from a string.
func ParseClientBuildInfoString(fullString string) ClientBuildInfo {
	res := &ClientBuildInfo{}
	if strings.Count(fullString, argDelimiter) > 0 {
		substrings := strings.Split(fullString, argDelimiter)
		res.Name = substrings[0]
		for i := len(substrings) - 1; i > 0; i-- {
			res.ParseSubstring(substrings[i], i == (len(substrings)-1))
		}
	} else {
		res.Name = fullString

	}
	return *res
}

type ClientsBuildInfo []ClientBuildInfo

func (c ClientsBuildInfo) Names() []string {
	names := make([]string, len(c))
	for i, client := range c {
		names[i] = client.Name
	}
	return names
}

func ClientsBuildInfoFromString(arg string) (ClientsBuildInfo, error) {
	var res ClientsBuildInfo
	for _, name := range strings.Split(arg, clientDelimiter) {
		res = append(res, ParseClientBuildInfoString(name))
	}
	return res, nil
}

func ClientsBuildInfoFromFile(file io.Reader) (ClientsBuildInfo, error) {
	// Read the YAML file
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	var res ClientsBuildInfo
	// First try to unmarshal as yaml
	errYaml := yaml.Unmarshal(data, &res)
	if errYaml == nil {
		return res, nil
	}

	// If that fails, try to unmarshal as a json
	errJson := json.Unmarshal(data, &res)
	if errJson == nil {
		return res, nil
	}

	// Combine the errors
	return nil, fmt.Errorf("unable to parse clients file: %s, json: %s", errYaml.Error(), errJson.Error())
}

// Inventory keeps names of clients and simulators.
type Inventory struct {
	BaseDir    string
	Clients    map[string]struct{}
	Simulators map[string]struct{}
}

// HasClient returns true if the inventory contains the given client.
// The client name may contain a branch specifier.
func (inv Inventory) HasClient(client ClientBuildInfo) bool {
	_, ok := inv.Clients[client.Name]
	return ok
}

// ClientDirectory returns the directory containing the given client's Dockerfile.
// The client name may contain a branch specifier.
func (inv Inventory) ClientDirectory(client ClientBuildInfo) string {
	return filepath.Join(inv.BaseDir, "clients", filepath.FromSlash(client.Name))
}

// HasSimulator returns true if the inventory contains the given simulator.
func (inv Inventory) HasSimulator(name string) bool {
	_, ok := inv.Simulators[name]
	return ok
}

// SimulatorDirectory returns the directory of containing the given simulator's Dockerfile.
func (inv Inventory) SimulatorDirectory(name string) string {
	return filepath.Join(inv.BaseDir, "simulators", filepath.FromSlash(name))
}

// AddClient ensures the given client name is known to the inventory.
// This method exists for unit testing purposes only.
func (inv *Inventory) AddClient(name string) {
	if inv.Clients == nil {
		inv.Clients = make(map[string]struct{})
	}
	inv.Clients[name] = struct{}{}
}

// AddSimulator ensures the given simulator name is known to the inventory.
// This method exists for unit testing purposes only.
func (inv *Inventory) AddSimulator(name string) {
	if inv.Simulators == nil {
		inv.Simulators = make(map[string]struct{})
	}
	inv.Simulators[name] = struct{}{}
}

// MatchSimulators returns matching simulator names.
func (inv *Inventory) MatchSimulators(expr string) ([]string, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, nil
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, err
	}
	var result []string
	for sim := range inv.Simulators {
		if re.MatchString(sim) {
			result = append(result, sim)
		}
	}
	sort.Strings(result)
	return result, nil
}

// LoadInventory finds all clients and simulators in basedir.
func LoadInventory(basedir string) (Inventory, error) {
	var err error
	inv := Inventory{BaseDir: basedir}
	inv.Clients, err = findDockerfiles(filepath.Join(basedir, "clients"))
	if err != nil {
		return inv, err
	}
	inv.Simulators, err = findDockerfiles(filepath.Join(basedir, "simulators"))
	return inv, err
}

func findDockerfiles(dir string) (map[string]struct{}, error) {
	names := make(map[string]struct{})
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// If we hit a dockerfile, add the parent and stop looking in this directory.
		if strings.HasSuffix(path, "Dockerfile") {
			rel, _ := filepath.Rel(dir, filepath.Dir(path))
			name := filepath.ToSlash(rel)
			names[name] = struct{}{}
			return filepath.SkipDir
		}
		return nil
	})
	return names, err
}
