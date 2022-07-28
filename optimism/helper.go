package optimism

import (
	"bytes"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/hivesim"
	"io"
	"io/ioutil"
)

func bytesFile(name string, data []byte) hivesim.StartOption {
	return hivesim.WithDynamicFile(name, func() (io.ReadCloser, error) {
		return ioutil.NopCloser(bytes.NewReader(data)), nil
	})
}

func stringFile(name string, data string) hivesim.StartOption {
	return bytesFile(name, []byte(data))
}

var defaultJWTPath = "/hive/input/jwt-secret.txt"
var defaultJWTSecret = common.Hash{42}
var defaultJWTFile = stringFile(defaultJWTPath, defaultJWTSecret.String())

// UnprefixedParams are hivesim.Params that have yet to be prefixed with "HIVE_".
//
// In optimism monorepo we have many flags packages that define flags with namespaced env vars.
// We use these same env vars to configure the hive clients.
// But we need to add "HIVE_" to them, and then unpack them again in the hive client entrypoint, to use them.
//
// Within the client only params starting with "HIVE_<namespace>_" will be unpacked.
// E.g. "HIVE_ROLLUP_NODE_HELLO_WORLD" will become "ROLLUP_NODE_HELLO_WORLD"
type UnprefixedParams hivesim.Params

func (u UnprefixedParams) Params() hivesim.Params {
	out := make(hivesim.Params)
	for k, v := range u {
		out["HIVE_"+k] = v
	}
	return out
}
func (u UnprefixedParams) Merge(other UnprefixedParams) {
	for k, v := range other {
		u[k] = v
	}
}
