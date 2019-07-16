package common

// TestSuiteHostProvider returns a singleton testsuitehost given an
// initial configuration
type TestSuiteHostProvider func(config []byte) (TestSuiteHost, error)

// TestSuiteHostProviders is the dictionary of test suit host providers
var testSuiteHostProviders map[string]TestSuiteHostProvider

func RegisterProvider(key string, provider TestSuiteHostProvider) {
	testSuiteHostProviders[key] = provider
}

// GetProvider selects the provider singleton configurator for a simulation backend
func GetProvider(providerType string) (TestSuiteHostProvider, error) {
	provider, ok := testSuiteHostProviders[providerType]
	if !ok {
		return nil, ErrNoSuchProviderType
	}
	return provider, nil
}
