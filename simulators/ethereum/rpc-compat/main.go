package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/hivesim"
	"github.com/nsf/jsondiff"
	openrpc "github.com/open-rpc/meta-schema"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
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

	// Load method result schemas from the OpenRPC spec.
	schemas, err := loadMethodSchemas("openrpc.json")
	if err != nil {
		panic(fmt.Sprintf("failed to load OpenRPC spec: %v", err))
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
			runAllTests(t, c, c.Type, schemas)
		},
		AlwaysRun: true,
	})
	sim := hivesim.New()
	hivesim.MustRunSuite(sim, suite)
}

func runAllTests(t *hivesim.T, c *hivesim.Client, clientName string, schemas map[string]openrpc.JSONSchemaObject) {
	_, testPattern := t.Sim.TestPattern()
	re := regexp.MustCompile(testPattern)
	tests := loadTests(t, "tests", re)
	for _, test := range tests {
		test := test
		t.Run(hivesim.TestSpec{
			Name:        fmt.Sprintf("%s (%s)", test.name, clientName),
			Description: test.comment,
			Run: func(t *hivesim.T) {
				if err := runTest(t, c, &test, schemas); err != nil {
					t.Fatal(err)
				}
			},
		})
	}
}

func runTest(t *hivesim.T, c *hivesim.Client, test *rpcTest, schemas map[string]openrpc.JSONSchemaObject) error {
	var (
		client     = &http.Client{Timeout: 5 * time.Second}
		url        = fmt.Sprintf("http://%s", net.JoinHostPort(c.IP.String(), "8545"))
		err        error
		respBytes  []byte
		lastMethod string
	)

	for _, msg := range test.messages {
		if msg.send {
			// Send request, track the method name for schema lookup.
			lastMethod = gjson.Get(msg.data, "method").String()
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

			// For speconly tests, validate the response result against the OpenRPC schema.
			hasError := gjson.Get(resp, "error").Exists()
			if !hasError && test.speconly {
				schema, ok := schemas[lastMethod]
				if !ok {
					return fmt.Errorf("no schema found for method %s", lastMethod)
				}
				result := json.RawMessage(gjson.Get(resp, "result").Raw)
				if err := validateResult(schema, result, lastMethod); err != nil {
					t.Log(err)
					return fmt.Errorf("response does not match schema for %s: %v", lastMethod, err)
				}
				respBytes = nil
				continue
			}

			// Patch JSON to remove error messages. We only do this in the specific case
			// where an error is expected AND returned by the client.
			var errorRedacted bool
			if hasError && gjson.Get(expectedData, "error").Exists() {
				resp, _ = sjson.Delete(resp, "error.message")
				expectedData, _ = sjson.Delete(expectedData, "error.message")
				errorRedacted = true
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

// loadMethodSchemas reads the dereferenced OpenRPC spec and returns a map of
// method name to its result JSON schema.
func loadMethodSchemas(path string) (map[string]openrpc.JSONSchemaObject, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc openrpc.OpenrpcDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	schemas := make(map[string]openrpc.JSONSchemaObject)
	for _, method := range *doc.Methods {
		if method.MethodObject == nil {
			continue
		}
		m := method.MethodObject
		if m.Result == nil || m.Result.ContentDescriptorObject == nil {
			continue
		}
		cd := m.Result.ContentDescriptorObject
		if cd.Schema == nil || cd.Schema.JSONSchemaObject == nil {
			continue
		}
		schemas[string(*m.Name)] = *cd.Schema.JSONSchemaObject
	}
	return schemas, nil
}

// validateResult validates the result value against the method's result schema.
func validateResult(schema openrpc.JSONSchemaObject, result json.RawMessage, method string) error {
	draft := openrpc.Schema("https://json-schema.org/draft/2019-09/schema")
	schema.Schema = &draft
	b, err := json.Marshal(schema)
	if err != nil {
		return fmt.Errorf("unable to marshal schema: %v", err)
	}
	s, err := jsonschema.CompileString(method+".result", string(b))
	if err != nil {
		return err
	}
	var x interface{}
	if err := json.Unmarshal(result, &x); err != nil {
		return err
	}
	return s.Validate(x)
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
