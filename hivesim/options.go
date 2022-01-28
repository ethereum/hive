package hivesim

import (
	"io"
	"os"
	"strings"
)

// clientSetup collects client options.
type clientSetup struct {
	parameters map[string]string
	// destination path -> open data function
	files map[string]func() (io.ReadCloser, error)
}

// StartOption is a parameter for starting a client.
type StartOption interface {
	Apply(setup *clientSetup)
}

type optionFunc func(setup *clientSetup)

func (fn optionFunc) Apply(setup *clientSetup) { fn(setup) }

func fileAsSrc(path string) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		return os.Open(path)
	}
}

// WithInitialNetworks adds a list of network names as the initial networks the client must be connected before starting.
func WithInitialNetworks(networks []string) StartOption {
	return optionFunc(func(setup *clientSetup) {
		setup.parameters["NETWORKS"] = strings.Join(networks, ",")
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
			opt.Apply(setup)
		}
	})
}
