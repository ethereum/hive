package common

import (
	"io"

	"github.com/ethereum/hive/simulators/common"
	"github.com/ethereum/hive/simulators/common/providers/hive"
	"github.com/ethereum/hive/simulators/common/providers/local"
)

// TestSuiteHostProvider returns a singleton testsuitehost given an
// initial configuration and a test result output stream
type TestSuiteHostProvider func(config []byte, output io.Writer) (common.TestSuiteHost, error)

// TestSuiteHostProviders is the dictionary of test suit host providers
var TestSuiteHostProviders = map[string]TestSuiteHostProvider{
	"local": local.GetInstance,
	"hive":  hive.GetInstance,
}
