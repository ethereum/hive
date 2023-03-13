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

// AnyTest is a TestSpec or ClientTestSpec.
type AnyTest interface {
	runTest(*Simulation, SuiteID, *Suite) error
}

// Run executes all given test suites.
func Run(host *Simulation, suites ...Suite) error {
	for _, s := range suites {
		if err := RunSuite(host, s); err != nil {
			return err
		}
	}
	return nil
}

// MustRun executes all given test suites, exiting the process if there is a problem
// reaching the simulation API.
func MustRun(host *Simulation, suites ...Suite) {
	for _, s := range suites {
		MustRunSuite(host, s)
	}
}

// RunSuite runs all tests in a suite.
func RunSuite(host *Simulation, suite Suite) error {
	if !host.m.match(suite.Name, "") {
		fmt.Fprintf(os.Stderr, "skipping suite %q because it doesn't match test pattern %s\n", suite.Name, host.m.pattern)
		return nil
	}

	suiteID, err := host.StartSuite(suite.Name, suite.Description, "")
	if err != nil {
		return err
	}
	defer host.EndSuite(suiteID)

	for _, test := range suite.Tests {
		if err := test.runTest(host, suiteID, &suite); err != nil {
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
	// These fields are displayed in the UI. Be sure to add
	// a meaningful description here.
	Name        string
	Description string

	// If AlwaysRun is true, the test will run even if Name does not match the test
	// pattern. This option is useful for tests that launch a client instance and
	// then perform further tests against it.
	AlwaysRun bool

	// The Run function is invoked when the test executes.
	Run func(*T)
}

// ClientTestSpec is a test against a single client. You can either put this in your suite
// directly, or launch it using RunClient or RunAllClients from another test.
//
// When used as a test in a suite, the test runs against all available client types,
// with the specified Role. If no Role is specified, the test runs with all available clients.
//
// If the Name of the test includes "CLIENT", it is replaced by the client name being tested.
type ClientTestSpec struct {
	// These fields are displayed in the UI. Be sure to add
	// a meaningful description here.
	Name        string
	Description string

	// If AlwaysRun is true, the test will run even if Name does not match the test
	// pattern. This option is useful for tests that launch a client instance and
	// then perform further tests against it.
	AlwaysRun bool

	// This filters client types by role.
	// If no role is specified, the test runs for all available client types.
	Role string

	// Parameters and Files are launch options for client instances.
	Parameters Params
	Files      map[string]string

	// The Run function is invoked when the test executes.
	Run func(*T, *Client)
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

// EnodeURL returns the default peer-to-peer endpoint of the client.
func (c *Client) EnodeURL() (string, error) {
	return c.test.Sim.ClientEnodeURL(c.test.SuiteID, c.test.TestID, c.Container)
}

// EnodeURL returns the peer-to-peer endpoint of the client on a specific network.
func (c *Client) EnodeURLNetwork(network string) (string, error) {
	return c.test.Sim.ClientEnodeURLNetwork(c.test.SuiteID, c.test.TestID, c.Container, network)
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

// Exec runs a script in the client container.
func (c *Client) Exec(command ...string) (*ExecInfo, error) {
	return c.test.Sim.ClientExec(c.test.SuiteID, c.test.TestID, c.Container, command)
}

// Pauses the client container.
func (c *Client) Pause() error {
	return c.test.Sim.PauseClient(c.test.SuiteID, c.test.TestID, c.Container)
}

// Unpauses the client container.
func (c *Client) Unpause() error {
	return c.test.Sim.UnpauseClient(c.test.SuiteID, c.test.TestID, c.Container)
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
	suite   *Suite
	mu      sync.Mutex
	result  TestResult
}

// StartClient starts a client instance. If the client cannot by started, the test fails immediately.
func (t *T) StartClient(clientType string, option ...StartOption) *Client {
	container, ip, err := t.Sim.StartClientWithOptions(t.SuiteID, t.TestID, clientType, option...)
	if err != nil {
		t.Fatalf("can't launch node (type %s): %v", clientType, err)
	}
	return &Client{Type: clientType, Container: container, IP: ip, test: t}
}

// RunClient runs the given client test against a single client type.
// It waits for the subtest to complete.
func (t *T) RunClient(clientType string, spec ClientTestSpec) {
	test := testSpec{
		suiteID:   t.SuiteID,
		suite:     t.suite,
		name:      clientTestName(spec.Name, clientType),
		desc:      spec.Description,
		alwaysRun: spec.AlwaysRun,
	}
	runTest(t.Sim, test, func(t *T) {
		client := t.StartClient(clientType, spec.Parameters, WithStaticFiles(spec.Files))
		spec.Run(t, client)
	})
}

// RunAllClients runs the given client test against all available client types.
// It waits for all subtests to complete.
func (t *T) RunAllClients(spec ClientTestSpec) {
	spec.runTest(t.Sim, t.SuiteID, t.suite)
}

// Run runs a subtest of this test. It waits for the subtest to complete before continuing.
// It is safe to call this from multiple goroutines concurrently, just be sure to wait for
// all your tests to finish until returning from the parent test.
func (t *T) Run(spec TestSpec) {
	spec.runTest(t.Sim, t.SuiteID, t.suite)
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

type testSpec struct {
	suiteID   SuiteID
	suite     *Suite
	name      string
	desc      string
	alwaysRun bool
}

func runTest(host *Simulation, test testSpec, runit func(t *T)) error {
	if !test.alwaysRun && !host.m.match(test.suite.Name, test.name) {
		fmt.Fprintf(os.Stderr, "skipping test %q because it doesn't match test pattern %s\n", test.name, host.m.pattern)
		return nil
	}

	// Register test on simulation server and initialize the T.
	t := &T{
		Sim:     host,
		SuiteID: test.suiteID,
		suite:   test.suite,
	}
	testID, err := host.StartTest(test.suiteID, test.name, test.desc)
	if err != nil {
		return err
	}
	t.TestID = testID
	t.result.Pass = true
	defer func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		host.EndTest(test.suiteID, testID, t.result)
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

func (spec ClientTestSpec) runTest(host *Simulation, suiteID SuiteID, suite *Suite) error {
	clients, err := host.ClientTypes()
	if err != nil {
		return err
	}
	for _, clientDef := range clients {
		// 'role' is an optional filter, so eth1 tests, beacon node tests,
		// validator tests, etc. can all live in harmony.
		if spec.Role != "" && !clientDef.HasRole(spec.Role) {
			continue
		}
		test := testSpec{
			suiteID:   suiteID,
			suite:     suite,
			name:      clientTestName(spec.Name, clientDef.Name),
			desc:      spec.Description,
			alwaysRun: spec.AlwaysRun,
		}
		err := runTest(host, test, func(t *T) {
			client := t.StartClient(clientDef.Name, spec.Parameters, WithStaticFiles(spec.Files))
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

func (spec TestSpec) runTest(host *Simulation, suiteID SuiteID, suite *Suite) error {
	test := testSpec{
		suiteID:   suiteID,
		suite:     suite,
		name:      spec.Name,
		desc:      spec.Description,
		alwaysRun: spec.AlwaysRun,
	}
	return runTest(host, test, spec.Run)
}
