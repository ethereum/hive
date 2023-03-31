package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ethereum/hive/hivesim"
	diff "github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"
)

var (
	clientEnv = hivesim.Params{
		"HIVE_NODETYPE":       "full",
		"HIVE_NETWORK_ID":     "1337",
		"HIVE_CHAIN_ID":       "1337",
		"HIVE_FORK_HOMESTEAD": "0",
		//"HIVE_FORK_DAO_BLOCK":      2000,
		"HIVE_FORK_TANGERINE":                   "0",
		"HIVE_FORK_SPURIOUS":                    "0",
		"HIVE_FORK_BYZANTIUM":                   "0",
		"HIVE_FORK_CONSTANTINOPLE":              "0",
		"HIVE_FORK_PETERSBURG":                  "0",
		"HIVE_FORK_ISTANBUL":                    "0",
		"HIVE_FORK_BERLIN":                      "0",
		"HIVE_FORK_LONDON":                      "0",
		"HIVE_SHANGHAI_TIMESTAMP":               "0",
		"HIVE_TERMINAL_TOTAL_DIFFICULTY":        "0",
		"HIVE_TERMINAL_TOTAL_DIFFICULTY_PASSED": "1",
	}
	files = map[string]string{
		"genesis.json": "./tests/genesis.json",
		"chain.rlp":    "./tests/chain.rlp",
	}
)

type test struct {
	Name string
	Data []byte
}

func main() {
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
		t.Run(hivesim.TestSpec{
			Name: fmt.Sprintf("%s (%s)", test.Name, clientName),
			Run: func(t *hivesim.T) {
				if err := runTest(t, c, test.Data); err != nil {
					t.Fatal(err)
				}
			},
		})
	}
}

func runTest(t *hivesim.T, c *hivesim.Client, data []byte) error {
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

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case len(line) == 0 || strings.HasPrefix(line, "//"):
			// Skip comments, blank lines.
			continue
		case strings.HasPrefix(line, ">> "):
			// Send request.
			resp, err = postHttp(client, url, []byte(line[3:]))
			if err != nil {
				return err
			}
		case strings.HasPrefix(line, "<< "):
			// Read response. Unmarshal to interface{} to verify deep equality. Marshal
			// again to remove padding differences and to print each field in the same
			// order. This makes it easy to spot any discrepancies.
			if resp == nil {
				return fmt.Errorf("invalid test, response before request")
			}
			want := []byte(strings.TrimSpace(line)[3:]) // trim leading "<< "

			// Unmarshal to map[string]interface{} to compare.
			var wantMap map[string]interface{}
			if err := json.Unmarshal(want, &wantMap); err != nil {
				return fmt.Errorf("failed to unmarshal value: %s\n", err)
			}

			var respMap map[string]interface{}
			if err := json.Unmarshal(resp, &respMap); err != nil {
				return fmt.Errorf("failed to unmarshal value: %s\n", err)
			}

			if c.Type == "reth" {
				// If errors exist in both, make them equal.
				// While error comparison might be desirable, error text across
				// clients is not standardized, so we should not compare them.
				if wantMap["error"] != nil && respMap["error"] != nil {
					respError := respMap["error"].(map[string]interface{})
					wantError := wantMap["error"].(map[string]interface{})
					respError["message"] = wantError["message"]
					// cast back into the any type
					respMap["error"] = respError
				}
			}

			// Now compare.
			d := diff.New().CompareObjects(respMap, wantMap)

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
				return fmt.Errorf("response differs from expected:\n%s", diffString)
			}
			resp = nil
		default:
			t.Fatalf("invalid line in test script: %s", line)
		}
	}
	if resp != nil {
		t.Fatalf("unhandled response in test case")
	}
	return nil
}

// sendHttp sends an HTTP POST with the provided json data and reads the
// response into a byte slice and returns it.
func postHttp(c *http.Client, url string, d []byte) ([]byte, error) {
	data := bytes.NewBuffer(d)
	req, err := http.NewRequest("POST", url, data)
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

// loggingRoundTrip writes requests and responses to the test log.
type loggingRoundTrip struct {
	t     *hivesim.T
	inner http.RoundTripper
}

func (rt *loggingRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	// Read and log the request body.
	reqBytes, err := ioutil.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, err
	}
	rt.t.Logf(">>  %s", bytes.TrimSpace(reqBytes))
	reqCopy := *req
	reqCopy.Body = ioutil.NopCloser(bytes.NewReader(reqBytes))

	// Do the round trip.
	resp, err := rt.inner.RoundTrip(&reqCopy)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read and log the response bytes.
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	respCopy := *resp
	respCopy.Body = ioutil.NopCloser(bytes.NewReader(respBytes))
	rt.t.Logf("<<  %s", bytes.TrimSpace(respBytes))
	return &respCopy, nil
}

// loadTests walks the given directory looking for *.io files to load.
func loadTests(t *hivesim.T, root string, re *regexp.Regexp) []test {
	tests := make([]test, 0)
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			t.Logf("unable to walk path: %s", err)
			return err
		}
		if info.IsDir() {
			return nil
		}
		if fname := info.Name(); !strings.HasSuffix(fname, ".io") {
			return nil
		}
		pathname := strings.TrimSuffix(strings.TrimPrefix(path, root), ".io")
		if !re.MatchString(pathname) {
			fmt.Println("skip", pathname)
			return nil // skip
		}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		tests = append(tests, test{strings.TrimLeft(pathname, "/"), data})
		return nil
	})
	return tests
}
