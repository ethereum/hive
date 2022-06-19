package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/ethereum/hive/hivesim"
)

var (
	chainID   = big.NewInt(1)
	networkID = big.NewInt(1)
)

var clientEnv = hivesim.Params{
	"HIVE_NODETYPE":       "full",
	"HIVE_NETWORK_ID":     "1",
	"HIVE_CHAIN_ID":       "1",
	"HIVE_FORK_HOMESTEAD": "0",
	//"HIVE_FORK_DAO_BLOCK":      2000,
	"HIVE_FORK_TANGERINE":      "0",
	"HIVE_FORK_SPURIOUS":       "0",
	"HIVE_FORK_BYZANTIUM":      "0",
	"HIVE_FORK_CONSTANTINOPLE": "0",
	"HIVE_FORK_PETERSBURG":     "0",
	"HIVE_FORK_ISTANBUL":       "0",
	"HIVE_FORK_BERLIN":         "0",
	"HIVE_FORK_LONDON":         "0",
	"HIVE_SKIP_POW":            "1",
	"HIVE_CHECK_LIVE_PORT":     "",
}

var files = map[string]string{
	"genesis.json": "./tests/genesis.json",
	"chain.rlp":    "./tests/chain.rlp",
}

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
		url     = fmt.Sprintf("http://%s", net.JoinHostPort(c.IP.String(), "8545"))
		readbuf = bytes.NewBuffer(nil)
	)

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case len(line) == 0 || strings.HasPrefix(line, "//"):
			// Skip comments, blank lines.
			continue
		case strings.HasPrefix(line, ">> "):
			// Write to connection.
			data := bytes.NewBuffer([]byte(line[3:]))
			req, err := http.NewRequest("POST", url, data)
			if err != nil {
				t.Fatalf("request error: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("write error: %v", err)
			}
			readbuf.ReadFrom(resp.Body)
		case strings.HasPrefix(line, "<< "):
			want := line[3:]
			// Read line from response buffer and compare.
			got, err := readbuf.ReadString('\n')
			if err != io.EOF && err != nil {
				t.Fatalf("read error: %v", err)
			}
			if eq, err := jsonEq(want, got); err != nil {
				t.Fatalf("json decoding error: %v", err)
			} else if !eq {
				t.Errorf("wrong line from server\ngot:  %s\nwant: %s", got, want)
			}
		default:
			t.Fatalf("invalid line in test script: %s", line)
		}
	}
	return nil
}

func jsonEq(a string, b string) (bool, error) {
	var x, y interface{}
	if err := json.Unmarshal([]byte(a), &x); err != nil {
		return false, err
	}
	if err := json.Unmarshal([]byte(b), &y); err != nil {
		return false, err
	}
	return reflect.DeepEqual(x, y), nil
}

func loadTests(t *hivesim.T, root string, re *regexp.Regexp) []test {
	tests := make([]test, 0, 0)
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
