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

// Inventory keeps names of clients and simulators.
type Inventory struct {
	BaseDir    string
	Clients    map[string]InventoryClient
	Simulators map[string]struct{}
}

type InventoryClient struct {
	Dockerfiles []string
	Meta        ClientMetadata
}

// ClientDirectory returns the directory containing the given client's Dockerfile.
// The client name may contain a branch specifier.
func (inv Inventory) ClientDirectory(client ClientDesignator) string {
	return filepath.Join(inv.BaseDir, "clients", filepath.FromSlash(client.Client))
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
		switch {
		case file == "Dockerfile":
			clients[clientName] = InventoryClient{
				Meta: ClientMetadata{
					Roles: []string{"eth1"}, // default role
				},
			}
		case strings.HasPrefix(file, "Dockerfile."):
			client, ok := clients[clientName]
			if !ok {
				log15.Warn(fmt.Sprintf("found %s in directory without Dockerfile", file), "path", filepath.Dir(path))
				return nil
			}
			client.Dockerfiles = append(client.Dockerfiles, strings.TrimPrefix(file, "Dockerfile."))
			clients[clientName] = client
		case file == "hive.yaml":
			client, ok := clients[clientName]
			if !ok {
				log15.Warn("found hive.yaml in directory without Dockerfile", "path", filepath.Dir(path))
				return nil
			}
			md, err := loadClientMetadata(path)
			if err != nil {
				return err
			}
			client.Meta = md
			clients[clientName] = client
		}
		return nil
	})
	return clients, err
}

func loadClientMetadata(path string) (m ClientMetadata, err error) {
	f, err := os.Open(path)
	if err != nil {
		return m, err
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return m, fmt.Errorf("error in %s: %v", path, err)
	}
	return m, nil
}

// ClientDesignator specifies a client and build parameters for it.
type ClientDesignator struct {
	// Client is the client name.
	// This must refer to a subdirectory of clients/
	Client string `yaml:"client"`

	// Nametag is used in the name of the client image.
	// This is for assigning meaningful names to different builds of the same client.
	// If unspecified, a default value is chosen to make client names unique.
	Nametag string `yaml:"nametag,omitempty"`

	// DockerfileExt is the extension of the Docker that should be used to build the
	// client. Example: setting this to "git" will build using "Dockerfile.git".
	DockerfileExt string `yaml:"dockerfile,omitempty"`

	// Arguments passed to the docker build.
	BuildArgs map[string]string `yaml:"build_args,omitempty"`
}

func (c ClientDesignator) buildString() string {
	var values []string
	if c.DockerfileExt != "" {
		values = append(values, c.DockerfileExt)
	}
	if c.BuildArgs["tag"] != "" {
		values = append(values, c.BuildArgs["tag"])
	}
	keys := maps.Keys(c.BuildArgs)
	sort.Strings(keys)
	for _, k := range keys {
		if k == "tag" {
			continue
		}
		values = append(values, k, c.BuildArgs[k])
	}
	return strings.Join(values, "_")
}

// Dockerfile gives the name of the Dockerfile to use when building the client.
func (c ClientDesignator) Dockerfile() string {
	if c.DockerfileExt == "" {
		return "Dockerfile"
	}
	return "Dockerfile." + c.DockerfileExt
}

// Name returns the full client name including nametag.
func (c ClientDesignator) Name() string {
	if c.Nametag == "" {
		return c.Client
	}
	return c.Client + "_" + c.Nametag
}

// ParseClientList reads a comma-separated list of client names. Each client name may
// optionally contain a branch/tag specifier separated from the name by underscore, e.g.
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
			return res, fmt.Errorf("invalid branch/tag: %s", tag)
		}
		res.BuildArgs = map[string]string{"tag": tag}
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

// FilterClients trims the given list to only include clients matching the 'filter list'.
func FilterClients(list []ClientDesignator, filter []string) []ClientDesignator {
	accept := make(set[string])
	for _, f := range filter {
		accept.add(strings.TrimSpace(f))
	}
	var res []ClientDesignator
	for _, c := range list {
		if accept.contains(c.Client) || accept.contains(c.Name()) {
			res = append(res, c)
		}
	}
	return res
}

var knownBuildArgs = map[string]struct{}{
	"tag":        {}, // this is the branch/version specifier when pulling the git repo or docker base image
	"github":     {}, // (for git pull) github repo to clone
	"baseimage":  {}, // (for dockerhub-based clients) name of the client image
	"local_path": {}, // (for builds from local source) path to the source directory
}

func validateClients(inv *Inventory, list []ClientDesignator) error {
	occurrences := make(map[string]int)
	clientTags := make(map[string]set[string])

	for _, c := range list {
		occurrences[c.Client]++

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
		clientTags[c.Client] = clientTags[c.Client].add(c.BuildArgs["tag"])
	}

	// Assign nametags.
	usednames := make(set[string], len(list))
	for i := range list {
		c := &list[i]
		if occurrences[c.Client] == 1 {
			continue
		}
		if c.Nametag == "" {
			// Try assigning nametag based on "tag" argument.
			if len(clientTags[c.Client]) == occurrences[c.Client] {
				c.Nametag = c.BuildArgs["tag"]
			} else {
				// Fall back to using all build arguments as nametag.
				c.Nametag = c.buildString()
			}
		}
		name := c.Name()
		if usednames.contains(name) {
			return fmt.Errorf("duplicate client name %q", name)
		}
		usednames.add(c.Name())
	}

	return nil
}

type set[X comparable] map[X]struct{}

func (s set[X]) add(x X) set[X] {
	if s == nil {
		s = make(map[X]struct{})
	}
	s[x] = struct{}{}
	return s
}

func (s set[X]) contains(x X) bool {
	_, ok := s[x]
	return ok
}
