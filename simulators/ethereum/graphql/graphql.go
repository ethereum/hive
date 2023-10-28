package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/hive/hivesim"
	diff "github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"
)

func main() {
	suite := hivesim.Suite{
		Name: "graphql",
		Description: `Test suite covering the graphql API surface.
The GraphQL tests were initially imported from the Besu codebase.`,
	}
	suite.Add(hivesim.ClientTestSpec{
		Role: "eth1",
		Name: "client launch",
		Description: `This is a meta-test. It launches the client with the test chain
and reads the test case files. The individual test cases are run as sub-tests against
the client launched by this test.`,
		Parameters: hivesim.Params{
			// The graphql chain comes from the Besu codebase, and is built on Frontier.
			"HIVE_CHAIN_ID":                  "1",
			"HIVE_GRAPHQL_ENABLED":           "1",
			"HIVE_ALLOW_UNPROTECTED_TX":      "1",
			"HIVE_FORK_FRONTIER":             "0",
			"HIVE_FORK_HOMESTEAD":            "33",
			"HIVE_FORK_TANGERINE":            "33",
			"HIVE_FORK_SPURIOUS":             "33",
			"HIVE_FORK_BYZANTIUM":            "33",
			"HIVE_FORK_CONSTANTINOPLE":       "33",
			"HIVE_FORK_PETERSBURG":           "33",
			"HIVE_FORK_ISTANBUL":             "33",
			"HIVE_FORK_MUIR_GLACIER":         "33",
			"HIVE_FORK_BERLIN":               "33",
			"HIVE_FORK_LONDON":               "33",
			"HIVE_MERGE_BLOCK_ID":            "33",
			"HIVE_TERMINAL_TOTAL_DIFFICULTY": "4357120",
			"HIVE_SHANGHAI_TIMESTAMP":        "1444660030",
		},
		Files: map[string]string{
			"/genesis.json": "./tests/genesis.json",
			"/chain.rlp":    "./tests/chain.rlp",
		},
		Run: graphqlTest,
	})
	hivesim.MustRunSuite(hivesim.New(), suite)
}

func graphqlTest(t *hivesim.T, c *hivesim.Client) {
	parallelism := 16
	if val, ok := os.LookupEnv("HIVE_PARALLELISM"); ok {
		if p, err := strconv.Atoi(val); err != nil {
			t.Logf("Warning: invalid HIVE_PARALLELISM value %q", val)
		} else {
			parallelism = p
		}
	}

	_, testPattern := t.Sim.TestPattern()
	re := regexp.MustCompile(testPattern)

	var wg sync.WaitGroup
	testCh := deliverTests(t, "tests", &wg, re)
	for i := 0; i < parallelism; i++ {
		wg.Add(1)
		func() {
			defer wg.Done()
			for test := range testCh {
				url := "https://github.com/ethereum/execution-apis/blob/main/tests/graphql"
				t.Run(hivesim.TestSpec{
					Name:        fmt.Sprintf("%s (%s)", test.Name, c.Type),
					Description: fmt.Sprintf("Test case source: %s/%v.io", url, test.Name),
					Run: func(t *hivesim.T) {
						if err := runTest(t, c, test.Data); err != nil {
							t.Fatalf("failed in test: %s, error: %v\n", test.Name, err)
						}
					},
				})
			}
		}()
	}
	wg.Wait()
}

type test struct {
	Name string
	Data []byte
}

// deliverTests reads the test case files, sending them to the output channel.
func deliverTests(t *hivesim.T, root string, wg *sync.WaitGroup, re *regexp.Regexp) <-chan *test {
	out := make(chan *test)
	wg.Add(1)
	go func() {
		defer wg.Done()

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
				t.Log("skip", pathname)
				return nil // skip
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			name := strings.TrimLeft(strings.TrimLeft(pathname, root), "/")
			out <- &test{Name: name, Data: data}
			return nil
		})

		close(out)
	}()
	return out
}

type qlQuery struct {
	Query string `json:"query"`
}

type qlResponse struct {
	Errors *[]interface{} `json:"errors,omitempty"`
	Data   *interface{}   `json:"data,omitempty"`
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
		url  = fmt.Sprintf("http://%s:8545/graphql", c.IP.String())
		err  error
		code int
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
			code, resp, err = sendHTTP(client, url, line[3:])
			if err != nil {
				return err
			}
		case strings.HasPrefix(line, "<< "):
			// Read response. Unmarshal to interface{} to verify deep equality. Marshal
			// again to remove padding differences and to print each field in the same
			// order. This makes it easy to spot any discrepancies.
			if resp == nil {
				return errors.New("invalid test, response before request")
			}

			var have qlResponse
			if err := json.Unmarshal(resp, &have); err != nil {
				t.Fatal("can't decode response:", err)
			}

			// Check error with StatusCode only
			if have.Errors != nil {
				if code != 400 {
					return fmt.Errorf("invalid test stateCode, want 400, have %d", code)
				}
			} else {
				if code != 200 {
					return fmt.Errorf("invalid test stateCode, want 200, have %d", code)
				}
				want := []byte(strings.TrimSpace(line)[3:]) // trim leading "<< "

				// Now compare the response body.
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
					return fmt.Errorf("response differs from expected(-/have +/want):\n%s", diffString)
				}
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

// sendHttp sends an HTTP GraphQL POST with the provided data and reads the
// response into a byte slice and returns it.
// Example of working queries:
// curl 'http://127.0.0.1:8545/graphql' -H 'Content-Type: application/json' -d '{"query":"query blockNumber {block {number}}"}'
// curl 'http://127.0.0.1:8545/graphql' -H 'Content-Type: application/json' -d '{"query":"query blockNumber {block {number}}","variables":null,"operationName":"blockNumber"}'
func sendHTTP(c *http.Client, url string, query string) (int, []byte, error) {
	data, err := json.Marshal(qlQuery{Query: query})
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return 0, nil, fmt.Errorf("error building request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("write error: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("read error: %v", err)
	}
	resp.Body.Close()
	return resp.StatusCode, body, nil
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
