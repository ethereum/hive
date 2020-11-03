package hivesim

import (
	"github.com/ethereum/hive/internal/hive"
)

// SuiteID identifies a test suite context.
type SuiteID = hive.TestSuiteID

// TestID identifies a test case context.
type TestID = hive.TestID

// TestResult describes the outcome of a test.
type TestResult struct {
	Pass    bool   `json:"pass"`
	Details string `json:"details"`
}

// Params contains client launch parameters.
// This exists because tests usually want to define common parameters as
// a global variable and then customize them for specific clients.
type Params map[string]string

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
