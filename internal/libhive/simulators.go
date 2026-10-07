package libhive

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SimulatorDesignator specifies a simulator and its build parameters.
type SimulatorDesignator struct {
	// Simulator names a subdirectory of simulators/.
	Simulator string `yaml:"simulator" json:"simulator"`

	// DockerfileExt selects an alternate Dockerfile, e.g. "git" for Dockerfile.git.
	DockerfileExt string `yaml:"dockerfile,omitempty" json:"dockerfile,omitempty"`

	// BuildArgs contains arguments passed to the Dockerfile.
	BuildArgs map[string]string `yaml:"build_args,omitempty" json:"build_args,omitempty"`
}

// Dockerfile returns the name of the Dockerfile used to build the simulator.
func (s SimulatorDesignator) Dockerfile() string {
	if s.DockerfileExt == "" {
		return "Dockerfile"
	}
	return "Dockerfile." + s.DockerfileExt
}

// ParseSimulatorListYAML reads and validates a YAML list of simulators.
func ParseSimulatorListYAML(inv *Inventory, file io.Reader) ([]SimulatorDesignator, error) {
	var list []SimulatorDesignator
	dec := yaml.NewDecoder(file)
	dec.KnownFields(true)
	if err := dec.Decode(&list); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("unable to parse simulators file: %w", err)
	}
	if len(list) == 0 {
		return nil, errors.New("simulator list is empty")
	}
	seen := make(map[string]bool)
	for _, sim := range list {
		if _, ok := inv.Simulators[sim.Simulator]; !ok {
			return nil, fmt.Errorf("unknown simulator %q", sim.Simulator)
		}
		if seen[sim.Simulator] {
			return nil, fmt.Errorf("duplicate simulator %q", sim.Simulator)
		}
		seen[sim.Simulator] = true
		if sim.DockerfileExt != "" {
			if strings.ContainsAny(sim.DockerfileExt, `/\`) {
				return nil, fmt.Errorf("invalid Dockerfile extension %q", sim.DockerfileExt)
			}
			path := filepath.Join(inv.SimulatorDirectory(sim.Simulator), sim.Dockerfile())
			info, err := os.Stat(path)
			if errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("simulator %s doesn't have %s", sim.Simulator, sim.Dockerfile())
			}
			if err != nil {
				return nil, fmt.Errorf("simulator %s: %w", sim.Simulator, err)
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("simulator %s: %s is not a regular file", sim.Simulator, sim.Dockerfile())
			}
		}
	}
	return list, nil
}

// FilterSimulators selects file entries using the same regexp semantics as
// MatchSimulators. An empty pattern keeps all entries. File order is preserved.
func FilterSimulators(list []SimulatorDesignator, pattern string) ([]SimulatorDesignator, error) {
	re, err := compileSimulatorPattern(pattern)
	if err != nil {
		return nil, err
	}
	if re == nil {
		return list, nil
	}
	var result []SimulatorDesignator
	for _, sim := range list {
		if re.MatchString(sim.Simulator) {
			result = append(result, sim)
		}
	}
	return result, nil
}
