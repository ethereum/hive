package hive

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/simulators/ethereum/rpc/eth/common"
)

// SingleClientTest is a test against a single client.
type SingleClientTest struct {
	Name        string
	Description string
	Parameters  map[string]string
	Files       map[string]string
	Run         func(host *ClientTest)
}

// ClientTest is a test against a running client.
type ClientTest struct {
	Type      string
	Container string
	IP        net.IP

	host    common.TestSuiteHost
	testID  common.TestID
	suiteID common.TestSuiteID

	mu     sync.Mutex
	rpc    *rpc.Client
	result common.TestResult
}

// EnodeURL returns the peer-to-peer endpoint of the client.
func (t *ClientTest) EnodeURL() (string, error) {
	enodePtr, err := t.host.GetClientEnode(t.suiteID, t.testID, t.Container)
	if err != nil {
		return "", err
	}
	return *enodePtr, nil
}

// RPC returns an RPC client connected to the client's RPC server.
func (t *ClientTest) RPC() *rpc.Client {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.rpc == nil {
		t.rpc, _ = rpc.DialHTTP(fmt.Sprintf("http://%v:8545", t.IP))
	}
	return t.rpc
}

// Error is like testing.T.Error.
func (t *ClientTest) Error(values ...interface{}) {
	t.Log(values...)
	t.Fail()
}

// Errorf is like testing.T.Errorf.
func (t *ClientTest) Errorf(format string, values ...interface{}) {
	t.Logf(format, values...)
	t.Fail()
}

// Fatal is like testing.T.Fatal. It fails the test immediately.
func (t *ClientTest) Fatal(values ...interface{}) {
	t.Log(values...)
	t.FailNow()
}

// Fatalf is like testing.T.Fatalf. It fails the test immediately.
func (t *ClientTest) Fatalf(format string, values ...interface{}) {
	t.Logf(format, values...)
	t.FailNow()
}

// Logf prints to standard output, which goes to the simulation log file.
func (t *ClientTest) Logf(format string, values ...interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !strings.HasSuffix(format, "\n") {
		format = format + "\n"
	}
	fmt.Printf(format, values...)
	t.result.Details += fmt.Sprintf(format, values...)
}

// Log prints to standard output, which goes to the simulation log file.
func (t *ClientTest) Log(values ...interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fmt.Println(values...)
	t.result.Details += fmt.Sprintln(values...)
}

// Failed reports whether the test has already failed.
func (t *ClientTest) Failed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.result.Pass
}

// Fail signals that the test has failed.
func (t *ClientTest) Fail() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.result.Pass = false
}

// FailNow signals that the test has failed and exits the test immediately.
// As with testing.T.FailNow(), this should only be called from the main test goroutine.
func (t *ClientTest) FailNow() {
	t.Fail()
	runtime.Goexit()
}

// RunAllClients runs the given tests against all available client types.
func RunAllClients(host common.TestSuiteHost, name string, tests []SingleClientTest) error {
	clientTypes, err := host.GetClientTypes()
	if err != nil {
		return err
	}
	suiteID, err := host.StartTestSuite(name, "", os.Getenv("HIVE_SIMLOG"))
	if err != nil {
		return err
	}
	defer host.EndTestSuite(suiteID)

	for _, clientType := range clientTypes {
		for _, test := range tests {
			testID, err := host.StartTest(suiteID, test.Name, test.Description)
			if err != nil {
				return err
			}
			params := clientParams(test.Parameters, clientType)
			container, ip, _, err := host.GetNode(suiteID, testID, params, test.Files)
			if err != nil {
				host.EndTest(suiteID, testID, errorResult(err), nil)
				return err
			}
			t := &ClientTest{
				Type:      clientType,
				Container: container,
				IP:        ip,
				host:      host,
				testID:    testID,
				suiteID:   suiteID,
			}
			t.result.Pass = true
			runTest(t, test.Run)
			host.EndTest(suiteID, testID, &t.result, nil)
		}
	}
	return nil
}

func runTest(t *ClientTest, run func(*ClientTest)) {
	done := make(chan struct{})
	go func() {
		defer func() {
			if err := recover(); err != nil {
				buf := make([]byte, 4096)
				i := runtime.Stack(buf, false)
				t.Logf("panic: %v\n\n%s", err, buf[:i])
				t.Fail()
			}
			close(done)
		}()
		run(t)
	}()
	<-done
}

func errorResult(err error) *common.TestResult {
	if err == nil {
		return &common.TestResult{Pass: true}
	}
	return &common.TestResult{Details: err.Error()}
}

// clientParams adds the CLIENT key to a copy of 'params'.
func clientParams(params map[string]string, client string) map[string]string {
	pm := make(map[string]string, len(params))
	for k, v := range params {
		pm[k] = v
	}
	pm["CLIENT"] = client
	return pm
}
