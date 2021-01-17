package libhive

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// branchDelimiter is what separates the client name from the branch, eg: aleth_nightly, go-ethereum_master.
const branchDelimiter = "_"

// SplitClientName returns the name and branch components of 'name'.
func SplitClientName(name string) (string, string) {
	if ix := strings.LastIndex(name, branchDelimiter); ix > 0 {
		return name[:ix], name[ix:]
	}
	return name, ""
}

// Inventory keeps names of clients and simulators.
type Inventory struct {
	BaseDir    string
	Clients    map[string]struct{}
	Simulators map[string]struct{}
}

// HasClient returns true if the inventory contains the given client.
// The client name may contain a branch specifier.
func (inv Inventory) HasClient(name string) bool {
	name, _ = SplitClientName(name)
	_, ok := inv.Clients[name]
	return ok
}

// ClientDirectory returns the directory containing the given client's Dockerfile.
// The client name may contain a branch specifier.
func (inv Inventory) ClientDirectory(name string) string {
	name, _ = SplitClientName(name)
	return filepath.Join(inv.BaseDir, "clients", filepath.FromSlash(name))
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
