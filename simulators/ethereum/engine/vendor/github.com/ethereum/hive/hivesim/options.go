package hivesim

import (
	"io"
	"os"

	"github.com/ethereum/hive/internal/simapi"
)

// clientSetup collects client options.
type clientSetup struct {
	config simapi.NodeConfig
	// destination path -> open data function
	files map[string]func() (io.ReadCloser, error)
}

// StartOption is a parameter for starting a client.
type StartOption interface {
	apply(setup *clientSetup)
}

type optionFunc func(setup *clientSetup)

func (fn optionFunc) apply(setup *clientSetup) { fn(setup) }

func fileAsSrc(path string) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		return os.Open(path)
	}
}

// WithInitialNetworks configures networks that the client is initially connected to.
func WithInitialNetworks(networks []string) StartOption {
	return optionFunc(func(setup *clientSetup) {
		setup.config.Networks = make([]string, len(networks))
		copy(setup.config.Networks, networks)
	})
}

// WithStaticFiles adds files from the local filesystem to the client. Map: destination file path -> source file path.
func WithStaticFiles(initFiles map[string]string) StartOption {
	return optionFunc(func(setup *clientSetup) {
		for k, v := range initFiles {
			setup.files[k] = fileAsSrc(v)
		}
	})
}

// WithDynamicFile adds a file to a client, sourced dynamically from the given src function,
// called upon usage of the returned StartOption.
//
// A StartOption, and thus the src function, should be reusable and safe to use in parallel.
// Dynamic files can override static file sources (see WithStaticFiles) and vice-versa.
func WithDynamicFile(dstPath string, src func() (io.ReadCloser, error)) StartOption {
	return optionFunc(func(setup *clientSetup) {
		setup.files[dstPath] = src
	})
}

// Bundle combines start options, e.g. to bundle files together as option.
func Bundle(option ...StartOption) StartOption {
	return optionFunc(func(setup *clientSetup) {
		for _, opt := range option {
			opt.apply(setup)
		}
	})
}

// Params contains client launch parameters.
// This exists because tests usually want to define common parameters as
// a global variable and then customize them for specific clients.
type Params map[string]string

var _ StartOption = (Params)(nil)

// apply implements StartOption.
func (p Params) apply(setup *clientSetup) {
	for k, v := range p {
		setup.config.Environment[k] = v
	}
}

// Set returns a copy of the parameters with 'key' set to 'value'.
func (p Params) Set(key, value string) Params {
	cpy := p.Copy()
	cpy[key] = value
	return cpy
}

// Copy returns a copy of the parameters.
func (p Params) Copy() Params {
	cpy := make(Params, len(p))
	for k, v := range p {
		cpy[k] = v
	}
	return cpy
}
