package libhive

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"gopkg.in/inconshreveable/log15.v2"
	"gopkg.in/yaml.v3"
)

// ClientDesignator specifies a client and build parameters for it.
type ClientDesignator struct {
	// Client is the client name.
	// This must refer to a subdirectory of clients/
	Client string `yaml:"client"`

	// DockerfileExt is the extension of the Docker that should be used to build the
	// client. Example: setting this to "git" will build using "Dockerfile.git".
	DockerfileExt string `yaml:"dockerfile,omitempty"`

	// Arguments passed to the docker build.
	BuildArgs map[string]string `yaml:"build_args,omitempty"`
}

// String returns a unique string representation of the client build configuration.
func (c ClientDesignator) String() string {
	var b strings.Builder
	b.WriteString(c.Client)
	if c.DockerfileExt != "" {
		b.WriteString("_")
		b.WriteString(c.DockerfileExt)
	}
	keys := maps.Keys(c.BuildArgs)
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString("_")
		b.WriteString(k)
		b.WriteString("_")
		b.WriteString(c.BuildArgs[k])
	}
	return b.String()
}

// Dockerfile gives the name of the Dockerfile to use when building the client.
func (c ClientDesignator) Dockerfile() string {
	if c.DockerfileExt == "" {
		return "Dockerfile"
	}
	return "Dockerfile." + c.DockerfileExt
}

// ParseClientList reads a comma-separated list of client names. Each client name may
// optionally contain a branch specifier separated from the name by underscore, e.g.
// "besu_nightly".
func ParseClientList(inv *Inventory, arg string) ([]ClientDesignator, error) {
	var res []ClientDesignator
	for _, name := range strings.Split(arg, clientDelimiter) {
		c, err := parseClientDesignator(name)
		if err != nil {
			return nil, err
		}
		res = append(res, c)
	}
	if err := validateClients(inv, res); err != nil {
		return nil, err
	}
	return res, nil
}

const (
	branchDelimiter = "_" // separates the client name and branch, eg: besu_nightly
	clientDelimiter = "," // separates client names in a list
)

// parseClientDesignator parses a client name string.
func parseClientDesignator(fullString string) (ClientDesignator, error) {
	var res ClientDesignator
	if strings.Count(fullString, branchDelimiter) > 0 {
		substrings := strings.Split(fullString, branchDelimiter)
		res.Client = strings.Join(substrings[0:len(substrings)-1], "_")
		tag := substrings[len(substrings)-1]
		if tag == "" {
			return res, fmt.Errorf("invalid branch: %s", tag)
		}
		res.BuildArgs = map[string]string{"branch": tag}
	} else {
		res.Client = fullString
	}
	if res.Client == "" {
		return res, fmt.Errorf("invalid client name: %s", fullString)
	}
	return res, nil
}

// ParseClientListYAML reads a YAML document containing a list of clients.
func ParseClientListYAML(inv *Inventory, file io.Reader) ([]ClientDesignator, error) {
	var res []ClientDesignator
	dec := yaml.NewDecoder(file)
	dec.KnownFields(true)
	if err := dec.Decode(&res); err != nil {
		return nil, fmt.Errorf("unable to parse clients file: %w", err)
	}
	if err := validateClients(inv, res); err != nil {
		return nil, err
	}
	return res, nil
}

var knownBuildArgs = map[string]struct{}{
	"branch": {},
	"user":   {},
	"repo":   {},
}

func validateClients(inv *Inventory, list []ClientDesignator) error {
	for _, c := range list {
		// Validate client exists.
		ic, ok := inv.Clients[c.Client]
		if !ok {
			return fmt.Errorf("unknown client %q", c.Client)
		}
		// Validate DockerfileExt.
		if c.DockerfileExt != "" {
			if !slices.Contains(ic.Dockerfiles, c.DockerfileExt) {
				return fmt.Errorf("client %s doesn't have Dockerfile.%s", c.Client, c.DockerfileExt)
			}
		}
		// Check build arguments.
		for key := range c.BuildArgs {
			if _, ok := knownBuildArgs[key]; !ok {
				log15.Warn(fmt.Sprintf("unknown build arg %q in clients.yaml file", key))
			}
		}
	}
	return nil
}

// Inventory keeps names of clients and simulators.
type Inventory struct {
	BaseDir    string
	Clients    map[string]InventoryClient
	Simulators map[string]struct{}
}

type InventoryClient struct {
	Dockerfiles []string
}

// HasClient returns true if the inventory contains the given client.
func (inv Inventory) HasClient(client ClientDesignator) bool {
	ic, ok := inv.Clients[client.Client]
	if !ok {
		return false
	}
	if client.DockerfileExt != "" {
		return slices.Contains(ic.Dockerfiles, client.DockerfileExt)
	}
	return ok
}

// ClientDirectory returns the directory containing the given client's Dockerfile.
// The client name may contain a branch specifier.
func (inv Inventory) ClientDirectory(client ClientDesignator) string {
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
func (inv *Inventory) AddClient(name string, ic *InventoryClient) {
	if inv.Clients == nil {
		inv.Clients = make(map[string]InventoryClient)
	}
	var icv InventoryClient
	if ic != nil {
		icv = *ic
	}
	inv.Clients[name] = icv
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
	inv.Clients, err = findClients(filepath.Join(basedir, "clients"))
	if err != nil {
		return inv, err
	}
	inv.Simulators, err = findSimulators(filepath.Join(basedir, "simulators"))
	return inv, err
}

func findClients(dir string) (map[string]InventoryClient, error) {
	clients := make(map[string]InventoryClient)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || path == dir {
			return err
		}
		rel, err := filepath.Rel(dir, filepath.Dir(path))
		if err != nil {
			return err
		}
		clientName := filepath.ToSlash(rel)

		// Skip client sub-directories.
		if info.IsDir() && path != dir {
			if _, ok := clients[clientName]; ok {
				return filepath.SkipDir
			}
		}
		// Add Dockerfiles.
		file := info.Name()
		if file == "Dockerfile" || strings.HasPrefix(file, "Dockerfile.") {
			if file == "Dockerfile" {
				clients[clientName] = InventoryClient{}
			} else {
				client := clients[clientName]
				client.Dockerfiles = append(client.Dockerfiles, strings.TrimPrefix(file, "Dockerfile."))
				clients[clientName] = client
			}
		}
		return nil
	})
	return clients, err
}

func findSimulators(dir string) (map[string]struct{}, error) {
	names := make(map[string]struct{})
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		name := info.Name()
		// If we hit a dockerfile, add the parent and stop looking in this directory.
		if name == "Dockerfile" {
			rel, _ := filepath.Rel(dir, filepath.Dir(path))
			name := filepath.ToSlash(rel)
			names[name] = struct{}{}
			return filepath.SkipDir
		}
		return nil
	})
	return names, err
}
