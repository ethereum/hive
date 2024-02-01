package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/hivesim"
	diff "github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"
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
		client = &http.Client{
			Timeout: 5 * time.Second,
			Transport: &loggingRoundTrip{
				t:     t,
				inner: http.DefaultTransport,
			},
		}
		url  = fmt.Sprintf("http://%s", net.JoinHostPort(c.IP.String(), "8545"))
		err  error
		resp []byte
	)

	for _, msg := range test.messages {
		if msg.send {
			// Send request.
			resp, err = postHttp(client, url, strings.NewReader(msg.data))
			if err != nil {
				return err
			}
		} else {
			// Read response. Unmarshal to interface{} to verify deep equality. Marshal
			// again to remove padding differences and to print each field in the same
			// order. This makes it easy to spot any discrepancies.
			if resp == nil {
				return fmt.Errorf("invalid test, response before request")
			}
			want := []byte(msg.data)

			// Now compare.
			d, err := diff.New().Compare(resp, want)
			if err != nil {
				return fmt.Errorf("failed to unmarshal value: %s\n", err)
			}
			// If there is a discrepancy, return error.
			if d.Modified() {
				var got map[string]interface{}
				json.Unmarshal(resp, &got)
				config := formatter.AsciiFormatterConfig{
					ShowArrayIndex: true,
					Coloring:       false,
				}
				formatter := formatter.NewAsciiFormatter(got, config)
				diffString, _ := formatter.Format(d)
				return fmt.Errorf("response differs from expected (-- client, ++ test):\n%s", diffString)
			}
			resp = nil
		}
	}

	if resp != nil {
		t.Fatalf("unhandled response in test case")
	}
	return nil
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

// loggingRoundTrip writes requests and responses to the test log.
type loggingRoundTrip struct {
	t     *hivesim.T
	inner http.RoundTripper
}

func (rt *loggingRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	// Read and log the request body.
	reqBytes, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, err
	}
	rt.t.Logf(">>  %s", bytes.TrimSpace(reqBytes))
	reqCopy := *req
	reqCopy.Body = io.NopCloser(bytes.NewReader(reqBytes))

	// Do the round trip.
	resp, err := rt.inner.RoundTrip(&reqCopy)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read and log the response bytes.
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	respCopy := *resp
	respCopy.Body = io.NopCloser(bytes.NewReader(respBytes))
	rt.t.Logf("<<  %s", bytes.TrimSpace(respBytes))
	return &respCopy, nil
}
