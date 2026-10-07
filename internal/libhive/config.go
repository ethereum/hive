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

// Config holds the client and simulator build configurations of a --config file.
type Config struct {
	Clients    []ClientDesignator
	Simulators []SimulatorDesignator
}

// configEntry is one entry of the configuration file. It describes a client when the
// client field is set and a simulator when the simulator field is set. Both kinds share
// the dockerfile and build_args fields; nametag applies to clients only.
type configEntry struct {
	Client        string            `yaml:"client,omitempty"`
	Simulator     string            `yaml:"simulator,omitempty"`
	Nametag       string            `yaml:"nametag,omitempty"`
	DockerfileExt string            `yaml:"dockerfile,omitempty"`
	BuildArgs     map[string]string `yaml:"build_args,omitempty"`
}

// ParseConfigYAML reads a YAML list of client and simulator build configurations. Client
// and simulator entries may be mixed freely, so a file containing only client entries is
// the format accepted by --client-file. File order is preserved within each kind.
func ParseConfigYAML(inv *Inventory, file io.Reader) (*Config, error) {
	var entries []configEntry
	dec := yaml.NewDecoder(file)
	dec.KnownFields(true)
	if err := dec.Decode(&entries); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("unable to parse config file: %w", err)
	}
	if len(entries) == 0 {
		return nil, errors.New("config file is empty")
	}
	var cfg Config
	for i, e := range entries {
		switch {
		case e.Client != "" && e.Simulator != "":
			return nil, fmt.Errorf("config entry %d: client and simulator are mutually exclusive", i+1)
		case e.Client != "":
			cfg.Clients = append(cfg.Clients, ClientDesignator{
				Client:        e.Client,
				Nametag:       e.Nametag,
				DockerfileExt: e.DockerfileExt,
				BuildArgs:     e.BuildArgs,
			})
		case e.Simulator != "":
			if e.Nametag != "" {
				return nil, fmt.Errorf("config entry %d: simulator %s doesn't support nametag", i+1, e.Simulator)
			}
			cfg.Simulators = append(cfg.Simulators, SimulatorDesignator{
				Simulator:     e.Simulator,
				DockerfileExt: e.DockerfileExt,
				BuildArgs:     e.BuildArgs,
			})
		default:
			return nil, fmt.Errorf("config entry %d: missing client or simulator", i+1)
		}
	}
	if err := validateClients(inv, cfg.Clients); err != nil {
		return nil, err
	}
	if err := validateSimulators(inv, cfg.Simulators); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// validateSimulators checks that every simulator exists in the inventory, appears once,
// and has the selected Dockerfile.
func validateSimulators(inv *Inventory, list []SimulatorDesignator) error {
	seen := make(map[string]bool)
	for _, sim := range list {
		if _, ok := inv.Simulators[sim.Simulator]; !ok {
			return fmt.Errorf("unknown simulator %q", sim.Simulator)
		}
		if seen[sim.Simulator] {
			return fmt.Errorf("duplicate simulator %q", sim.Simulator)
		}
		seen[sim.Simulator] = true
		if sim.DockerfileExt != "" {
			if strings.ContainsAny(sim.DockerfileExt, `/\`) {
				return fmt.Errorf("invalid Dockerfile extension %q", sim.DockerfileExt)
			}
			path := filepath.Join(inv.SimulatorDirectory(sim.Simulator), sim.Dockerfile())
			info, err := os.Stat(path)
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("simulator %s doesn't have %s", sim.Simulator, sim.Dockerfile())
			}
			if err != nil {
				return fmt.Errorf("simulator %s: %w", sim.Simulator, err)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("simulator %s: %s is not a regular file", sim.Simulator, sim.Dockerfile())
			}
		}
	}
	return nil
}

// FilterSimulators selects configured simulators using the same regexp semantics as
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
