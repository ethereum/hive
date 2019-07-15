package common

import (
	"github.com/ethereum/hive/simulators/common"
	"github.com/ethereum/hive/simulators/common/providers/hive"
	"github.com/ethereum/hive/simulators/common/providers/local"
)

// TestSuiteHostProvider returns a singleton testsuitehost given an
// initial configuration
type TestSuiteHostProvider func(config []byte) (common.TestSuiteHost, error)

// TestSuiteHostProviders is the dictionary of test suit host providers
var testSuiteHostProviders = map[string]TestSuiteHostProvider{
	"local": local.GetInstance,
	"hive":  hive.GetInstance,
}

// GetProvider selects the provider singleton configurator for a simulation backend
func GetProvider(providerType string) (TestSuiteHostProvider, error) {
	provider, ok := testSuiteHostProviders[providerType]
	if !ok {
		return nil, ErrNoSuchProviderType
	}
	return provider, nil
}
