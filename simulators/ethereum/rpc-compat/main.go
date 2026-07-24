package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/hivesim"
	"github.com/nsf/jsondiff"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var (
	files = map[string]string{
		"genesis.json": "./tests/genesis.json",
		"chain.rlp":    "./tests/chain.rlp",
	}

	// specMethods holds the compiled OpenRPC result schema for each method,
	// used to validate "speconly" tests against the spec.
	specMethods methodSchemas
)

func main() {
	// Load fork environment.
	var clientEnv hivesim.Params
	err := common.LoadJSON("tests/forkenv.json", &clientEnv)
	if err != nil {
		panic(err)
	}
	if _, ok := clientEnv["HIVE_TARGET_GAS_LIMIT"]; !ok {
		clientEnv["HIVE_TARGET_GAS_LIMIT"] = "60000000"
	}

	// Load the OpenRPC spec so speconly tests can be validated against the
	// schema. The spec is shipped alongside the fixtures (see Dockerfile).
	if specMethods, err = parseSpec("openrpc.json"); err != nil {
		panic(fmt.Errorf("unable to load openrpc.json: %w", err))
	}

	// Run the test suite.
	suite := hivesim.Suite{
		Name: "rpc-compat",
		Description: `
The RPC-compatibility test suite runs a set of RPC related tests against a
running node. It tests client implementations of the JSON-RPC API for
conformance with the execution API specification.`[1:],
	}
	suite.Add(&hivesim.ClientTestSpec{
		Role:        "eth1",
		Name:        "client launch",
		Description: `This test launches the client and collects its logs.`,
		Parameters:  clientEnv,
		Files:       files,
		Run: func(t *hivesim.T, c *hivesim.Client) {
			sendForkchoiceUpdated(t, c)
			runAllTests(t, c, c.Type)
		},
		AlwaysRun: true,
	})
	sim := hivesim.New()
	hivesim.MustRunSuite(sim, suite)
}

func runAllTests(t *hivesim.T, c *hivesim.Client, clientName string) {
	_, testPattern := t.Sim.TestPattern()
	re := regexp.MustCompile(testPattern)
	tests := loadTests(t, "tests", re)
	for _, test := range tests {
		test := test
		t.Run(hivesim.TestSpec{
			Name:        fmt.Sprintf("%s (%s)", test.name, clientName),
			Description: test.comment,
			Run: func(t *hivesim.T) {
				if err := runTest(t, c, &test); err != nil {
					t.Fatal(err)
				}
			},
		})
	}
}

func runTest(t *hivesim.T, c *hivesim.Client, test *rpcTest) error {
	var (
		client    = &http.Client{Timeout: 5 * time.Second}
		url       = fmt.Sprintf("http://%s", net.JoinHostPort(c.IP.String(), "8545"))
		err       error
		respBytes []byte
		method    string
	)

	for _, msg := range test.messages {
		if msg.send {
			// Send request.
			t.Log(">> ", msg.data)
			method = gjson.Get(msg.data, "method").String()
			respBytes, err = postHttp(client, url, strings.NewReader(msg.data))
			if err != nil {
				return err
			}
		} else {
			// Receive a response.
			if respBytes == nil {
				return fmt.Errorf("invalid test, response before request")
			}
			expectedData := msg.data
			resp := string(bytes.TrimSpace(respBytes))
			t.Log("<< ", resp)
			if !gjson.Valid(resp) {
				return fmt.Errorf("invalid JSON response")
			}

			// For speconly tests, the recorded value is just one valid example;
			// the response is client- or config-specific and must not be matched
			// exactly. Validate it against the method's OpenRPC result schema so
			// any spec-valid response passes regardless of which optional fields
			// the client includes.
			hasError := gjson.Get(resp, "error").Exists()
			if !hasError && test.speconly {
				schema := specMethods[method]
				if schema == nil {
					return fmt.Errorf("no result schema for speconly method %s in spec", method)
				}
				if err := validateResult(schema, []byte(gjson.Get(resp, "result").Raw)); err != nil {
					t.Log(err.Error())
					return fmt.Errorf("response does not conform to %s result schema: %w", method, err)
				}
				respBytes = nil
				continue
			}

			// Patch JSON to remove error messages wherever both the client and expected
			// response contain an error object. This handles both top-level JSON-RPC
			// errors and nested errors (e.g. eth_simulateV1 calls[].error.message).
			var errorRedacted bool
			resp, expectedData, errorRedacted = redactErrorMessages(resp, expectedData)

			// Compare responses.
			opts := &jsondiff.Options{
				Added:            jsondiff.Tag{Begin: "++ "},
				Removed:          jsondiff.Tag{Begin: "-- "},
				Changed:          jsondiff.Tag{Begin: "-- "},
				ChangedSeparator: " ++ ",
				Indent:           "  ",
				CompareNumbers:   numbersEqual,
			}
			diffStatus, diffText := jsondiff.Compare([]byte(resp), []byte(expectedData), opts)

			// If there is a discrepancy, return error.
			if diffStatus != jsondiff.FullMatch {
				if errorRedacted {
					t.Log("note: error messages removed from comparison")
				}
				return fmt.Errorf("response differs from expected (-- client, ++ test):\n%s", diffText)
			}
			respBytes = nil
		}
	}

	if respBytes != nil {
		t.Fatalf("unhandled response in test case")
	}
	return nil
}

// redactErrorMessages removes the "message" field from every "error" object
// present in both the response and expected JSON. Client-specific wording is
// thereby excluded from comparison while the shape and other fields of the
// error are still checked. Returns the modified payloads and whether any
// redaction occurred.
//
// Both top-level JSON-RPC errors and nested errors (e.g. eth_simulateV1
// calls[].error.message) are handled. The walk does not descend into "error"
// objects themselves, so errors nested inside another error's payload are not
// touched.
func redactErrorMessages(resp, expected string) (string, string, bool) {
	paths := collectErrorMessagePaths(nil, "", gjson.Parse(resp), gjson.Parse(expected))
	if len(paths) == 0 {
		return resp, expected, false
	}
	// Deleting "<path>.message" never changes object keys or array indices
	// elsewhere, so the collected paths remain valid regardless of order.
	for _, p := range paths {
		resp, _ = sjson.Delete(resp, p)
		expected, _ = sjson.Delete(expected, p)
	}
	return resp, expected, true
}

// collectErrorMessagePaths walks the expected tree in parallel with the
// response tree and appends the sjson path of every "error.message" field
// present on both sides.
func collectErrorMessagePaths(paths []string, path string, respVal, expectedVal gjson.Result) []string {
	switch {
	case expectedVal.IsObject():
		expectedVal.ForEach(func(key, val gjson.Result) bool {
			k := key.String()
			respChild := respVal.Get(k)
			if !respChild.Exists() {
				return true
			}
			childPath := joinPath(path, k)
			if k == "error" {
				if val.Get("message").Exists() && respChild.Get("message").Exists() {
					paths = append(paths, childPath+".message")
				}
				// Do not descend into the error object itself.
				return true
			}
			paths = collectErrorMessagePaths(paths, childPath, respChild, val)
			return true
		})
	case expectedVal.IsArray():
		i := 0
		expectedVal.ForEach(func(_, val gjson.Result) bool {
			idx := strconv.Itoa(i)
			i++
			respChild := respVal.Get(idx)
			if !respChild.Exists() {
				return true
			}
			paths = collectErrorMessagePaths(paths, joinPath(path, idx), respChild, val)
			return true
		})
	}
	return paths
}

func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

func numbersEqual(a, b json.Number) bool {
	af, err1 := a.Float64()
	bf, err2 := b.Float64()
	if err1 == nil && err2 == nil {
		return af == bf || math.IsNaN(af) && math.IsNaN(bf)
	}
	return a == b
}

// sendHttp sends an HTTP POST with the provided json data and reads the
// response into a byte slice and returns it.
func postHttp(c *http.Client, url string, d io.Reader) ([]byte, error) {
	req, err := http.NewRequest("POST", url, d)
	if err != nil {
		return nil, fmt.Errorf("error building request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("write error: %v", err)
	}
	return io.ReadAll(resp.Body)
}

// sendForkchoiceUpdated delivers the initial FcU request to the client.
func sendForkchoiceUpdated(t *hivesim.T, client *hivesim.Client) {
	var request struct {
		Method string
		Params []any
	}
	if err := common.LoadJSON("tests/headfcu.json", &request); err != nil {
		t.Fatal("error loading forkchoiceUpdated:", err)
	}
	t.Logf("sending %s: %v", request.Method, request.Params)
	var resp any
	err := client.EngineAPI().Call(&resp, request.Method, request.Params...)
	if err != nil {
		t.Fatal("client rejected forkchoiceUpdated:", err)
	}
	t.Logf("response: %v", resp)
}
