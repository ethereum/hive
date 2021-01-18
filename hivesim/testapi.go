package hivesim

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/rpc"
)

// Suite is the description of a test suite.
type Suite struct {
	Name        string
	Description string
	Tests       []AnyTest
}

// Add adds a test to the suite.
func (s *Suite) Add(test AnyTest) *Suite {
	s.Tests = append(s.Tests, test)
	return s
}

// AnyTest is either Test or SingleClientTest.
type AnyTest interface {
	runTest(*Simulation, SuiteID) error
}

// RunSuite runs all tests in a suite.
func RunSuite(host *Simulation, suite Suite) error {
	logfile := os.Getenv("HIVE_SIMLOG") // TODO: remove this
	suiteID, err := host.StartSuite(suite.Name, suite.Description, logfile)
	if err != nil {
		return err
	}
	defer host.EndSuite(suiteID)

	for _, test := range suite.Tests {
		if err := test.runTest(host, suiteID); err != nil {
			return err
		}
	}
	return nil
}

// MustRunSuite runs the given suite, exiting the process if there is a problem reaching
// the simulation API.
func MustRunSuite(host *Simulation, suite Suite) {
	if err := RunSuite(host, suite); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// TestSpec is the description of a test.
//
// Using this test type doesn't launch any clients by default. To interact with clients,
// you can launch them using the t.Client method:
//
//    c := t.Client()
//    c.RPC().Call(...)
//
// or run a subtest using t.RunClientTest():
//
//    t.RunClientTest(hivesim.ClientTestSpec{...})
//
type TestSpec struct {
	Name        string
	Description string
	Run         func(*T)
}

// ClientTestSpec is a test against a single client. You can either put this in your suite
// directly, or launch it using RunClient or RunAllClients from another test.
//
// When used as a test in a suite, the test runs against all available client types.
//
// If the Name of the test includes "CLIENT", it is replaced by the client name being tested.
type ClientTestSpec struct {
	Name        string
	Description string
	Parameters  Params
	Files       map[string]string
	Run         func(*T, *Client)
}

// Client represents a running client.
type Client struct {
	Type      string
	Container string
	IP        net.IP

	mu   sync.Mutex
	rpc  *rpc.Client
	test *T
}

// EnodeURL returns the peer-to-peer endpoint of the client.
func (c *Client) EnodeURL() (string, error) {
	return c.test.Sim.ClientEnodeURL(c.test.SuiteID, c.test.TestID, c.Container)
}

// RPC returns an RPC client connected to the client's RPC server.
func (c *Client) RPC() *rpc.Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rpc == nil {
		c.rpc, _ = rpc.DialHTTP(fmt.Sprintf("http://%v:8545", c.IP))
	}
	return c.rpc
}

// T is a running test. This is a lot like testing.T, but has some additional methods for
// launching clients.
//
// All test log output (via t.Log, t.Logf) goes to the 'details' section of the test report.
type T struct {
	// Test case info.
	Sim     *Simulation
	TestID  TestID
	SuiteID SuiteID
	mu      sync.Mutex
	result  TestResult
}

// StartClient starts a client. If the client cannot by started, the test fails immediately.
func (t *T) StartClient(clientType string, parameters Params, files map[string]string) *Client {
	params := parameters.Set("CLIENT", clientType)
	container, ip, err := t.Sim.StartClient(t.SuiteID, t.TestID, params, files)
	if err != nil {
		t.Fatalf("can't launch node (type %s): %v", clientType, err)
	}
	return &Client{Type: clientType, Container: container, IP: ip, test: t}
}

// RunClient runs the given client test against a single client type.
// It waits for the subtest to complete.
func (t *T) RunClient(clientType string, spec ClientTestSpec) {
	runTest(t.Sim, t.SuiteID, spec.Name, spec.Description, func(t *T) {
		client := t.StartClient(clientType, spec.Parameters, spec.Files)
		spec.Run(t, client)
	})
}

// RunAllClients runs the given client test against all available client type.
// It waits for all subtests to complete.
func (t *T) RunAllClients(spec ClientTestSpec) {
	spec.runTest(t.Sim, t.SuiteID)
}

// Run runs a subtest of this test. It waits for the subtest to complete before continuing.
// It is safe to call this from multiple goroutines concurrently, just be sure to wait for
// all your tests to finish until returning from the parent test.
func (t *T) Run(spec TestSpec) {
	runTest(t.Sim, t.SuiteID, spec.Name, spec.Description, spec.Run)
}

// Error is like testing.T.Error.
func (t *T) Error(values ...interface{}) {
	t.Log(values...)
	t.Fail()
}

// Errorf is like testing.T.Errorf.
func (t *T) Errorf(format string, values ...interface{}) {
	t.Logf(format, values...)
	t.Fail()
}

// Fatal is like testing.T.Fatal. It fails the test immediately.
func (t *T) Fatal(values ...interface{}) {
	t.Log(values...)
	t.FailNow()
}

// Fatalf is like testing.T.Fatalf. It fails the test immediately.
func (t *T) Fatalf(format string, values ...interface{}) {
	t.Logf(format, values...)
	t.FailNow()
}

// Logf prints to standard output, which goes to the simulation log file.
func (t *T) Logf(format string, values ...interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !strings.HasSuffix(format, "\n") {
		format = format + "\n"
	}
	fmt.Printf(format, values...)
	t.result.Details += fmt.Sprintf(format, values...)
}

// Log prints to standard output, which goes to the simulation log file.
func (t *T) Log(values ...interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fmt.Println(values...)
	t.result.Details += fmt.Sprintln(values...)
}

// Failed reports whether the test has already failed.
func (t *T) Failed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.result.Pass
}

// Fail signals that the test has failed.
func (t *T) Fail() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.result.Pass = false
}

// FailNow signals that the test has failed and exits the test immediately.
// As with testing.T.FailNow(), this should only be called from the main test goroutine.
func (t *T) FailNow() {
	t.Fail()
	runtime.Goexit()
}

func runTest(host *Simulation, s SuiteID, name, desc string, runit func(t *T)) error {
	// Register test on simulation server and initialize the T.
	t := &T{
		Sim:     host,
		SuiteID: s,
	}
	testID, err := host.StartTest(s, name, desc)
	if err != nil {
		return err
	}
	t.TestID = testID
	t.result.Pass = true
	defer func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		host.EndTest(s, testID, t.result)
	}()

	// Run the test function.
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
		runit(t)
	}()
	<-done
	return nil
}

func (spec ClientTestSpec) runTest(host *Simulation, suite SuiteID) error {
	clients, err := host.ClientTypes()
	if err != nil {
		return err
	}
	for _, clientType := range clients {
		name := clientTestName(spec.Name, clientType)
		err := runTest(host, suite, name, spec.Description, func(t *T) {
			client := t.StartClient(clientType, spec.Parameters, spec.Files)
			spec.Run(t, client)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// clientTestName ensures that 'name' contains the client type.
func clientTestName(name, clientType string) string {
	if name == "" {
		return clientType
	}
	if strings.Contains(name, "CLIENT") {
		return strings.ReplaceAll(name, "CLIENT", clientType)
	}
	return name + " (" + clientType + ")"
}

func (spec TestSpec) runTest(host *Simulation, suite SuiteID) error {
	return runTest(host, suite, spec.Name, spec.Description, spec.Run)
}
