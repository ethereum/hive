package libhive

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// branchDelimiter is what separates the client name from the branch, eg: besu_nightly, go-ethereum_master.
const branchDelimiter = "_"

// ClientBuildInfo represents the build configuration of a client.
// The parameters set here are used when building the client docker image.
type ClientBuildInfo struct {
	Client string `yaml:"client"`

	// Dockerfile is the name of the Dockerfile to use for building the client.
	// E.g. using Dockerfile == 'git' will build using Dockerfile.git.
	Dockerfile string `yaml:"dockerfile"`

	// Parameters passed as environment to the docker build.
	BuildArguments map[string]string `yaml:"build_args"`
}

func (c ClientBuildInfo) String() string {
	var b strings.Builder
	b.WriteString(c.Client)
	if c.Dockerfile != "" {
		b.WriteString("_")
		b.WriteString(c.Dockerfile)
	}

	var keys []string
	for k := range c.BuildArguments {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString("_")
		b.WriteString(k)
		b.WriteString("_")
		b.WriteString(c.BuildArguments[k])
	}
	return b.String()
}

// Parses client build info from a string.
func ParseClientBuildInfoString(fullString string) (ClientBuildInfo, error) {
	res := ClientBuildInfo{}
	if strings.Count(fullString, branchDelimiter) > 0 {
		substrings := strings.Split(fullString, branchDelimiter)
		res.Client = strings.Join(substrings[0:len(substrings)-1], "_")
		tag := substrings[len(substrings)-1]
		if tag == "" {
			return res, fmt.Errorf("invalid branch: %s", tag)
		}
		res.BuildArguments = map[string]string{"branch": tag}
	} else {
		res.Client = fullString
	}
	if res.Client == "" {
		return res, fmt.Errorf("invalid client name: %s", fullString)
	}
	return res, nil
}

// clientDelimiter separates multiple clients in the parameter string.
const clientDelimiter = ","

func ClientsBuildInfoFromString(arg string) ([]ClientBuildInfo, error) {
	var res []ClientBuildInfo
	for _, name := range strings.Split(arg, clientDelimiter) {
		if clientBuildInfo, err := ParseClientBuildInfoString(name); err != nil {
			return nil, err
		} else {
			res = append(res, clientBuildInfo)
		}
	}
	return res, nil
}

func ClientsBuildInfoFromFile(file io.Reader) ([]ClientBuildInfo, error) {
	var res []ClientBuildInfo
	err := yaml.NewDecoder(file).Decode(&res)
	if err != nil {
		return nil, fmt.Errorf("unable to parse clients file: %w", err)
	}
	return res, nil
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
	_, ok := inv.Clients[client.Client]
	return ok
}

// ClientDirectory returns the directory containing the given client's Dockerfile.
// The client name may contain a branch specifier.
func (inv Inventory) ClientDirectory(client ClientBuildInfo) string {
	return filepath.Join(inv.BaseDir, "clients", filepath.FromSlash(client.Client))
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
