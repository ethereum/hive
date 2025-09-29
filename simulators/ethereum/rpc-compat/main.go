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
)

func main() {
	// Load fork environment.
	var clientEnv hivesim.Params
	err := common.LoadJSON("tests/forkenv.json", &clientEnv)
	if err != nil {
		panic(err)
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
	)

	for _, msg := range test.messages {
		if msg.send {
			// Send request.
			t.Log(">> ", msg.data)
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

			// Patch JSON to remove error messages. We only do this in the specific case
			// where an error is expected AND returned by the client.
			var errorRedacted bool
			hasError := gjson.Get(resp, "error").Exists()
			if hasError && gjson.Get(expectedData, "error").Exists() {
				resp, _ = sjson.Delete(resp, "error.message")
				expectedData, _ = sjson.Delete(expectedData, "error.message")
				errorRedacted = true
			}

			// For speconly tests, only check that keys are present, not values
			if !hasError && test.speconly {
				t.Log("note: speconly test - type-checking response")
				errors := checkJSONStructure(gjson.Parse(msg.data), gjson.Parse(resp), ".")
				if len(errors) > 0 {
					for _, err := range errors {
						t.Log(err)
					}
					return fmt.Errorf("response type does not match expected")
				}
				continue
			}

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

// checkJSONStructure checks whether the `actual` value matches the type structure
// of the `expected` value.
func checkJSONStructure(expected, actual gjson.Result, path string) []string {
	var errors []string

	buildPath := func(key string) string {
		if path != "." {
			return path + "." + key
		}
		return "." + key
	}

	if expected.Type != gjson.JSON {
		return errors
	}

	if expected.IsArray() {
		if !actual.IsArray() {
			errors = append(errors, fmt.Sprintf("%s: expected array but got %s", path, actual.Type))
		}
		return errors
	}

	// Check all expected keys exist with correct types
	expected.ForEach(func(key, value gjson.Result) bool {
		keyPath := buildPath(key.String())
		actualValue := actual.Get(key.String())

		if !actualValue.Exists() {
			errors = append(errors, fmt.Sprintf("%s: missing key", keyPath))
			return true
		}

		if value.Type != actualValue.Type && !(value.Type == gjson.JSON && actualValue.Type == gjson.JSON) {
			errors = append(errors, fmt.Sprintf("%s: type mismatch (expected %s, got %s)",
				keyPath, value.Type, actualValue.Type))
			return true
		}

		if value.IsObject() || value.IsArray() {
			errors = append(errors, checkJSONStructure(value, actualValue, keyPath)...)
		}
		return true
	})

	// Check for unexpected keys
	if actual.IsObject() {
		actual.ForEach(func(key, value gjson.Result) bool {
			if !expected.Get(key.String()).Exists() {
				errors = append(errors, fmt.Sprintf("%s: unexpected key in response", buildPath(key.String())))
			}
			return true
		})
	}

	return errors
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
