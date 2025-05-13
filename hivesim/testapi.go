package hivesim

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/internal/simapi"
)

// Suite is the description of a test suite.
type Suite struct {
	Name        string // Name is the unique identifier for the suite [Mandatory]
	DisplayName string // Display name for the suite (Name will be used if unset) [Optional]
	Location    string // Documentation output location for the test suite [Optional]
	Category    string // Category of the test suite [Optional]
	Description string // Description of the test suite (if empty, suite won't appear in documentation) [Optional]
	Tests       []AnyTest

	// SharedClients maps client IDs to client instances that are shared across tests
	SharedClients map[string]*Client

	// Internal tracking
	sharedClientOpts map[string][]StartOption // Stores options for starting shared clients
}

func (s *Suite) request() *simapi.TestRequest {
	return &simapi.TestRequest{
		Name:        s.Name,
		DisplayName: s.DisplayName,
		Location:    s.Location,
		Category:    s.Category,
		Description: s.Description,
	}
}

// Add adds a test to the suite.
func (s *Suite) Add(test AnyTest) *Suite {
	s.Tests = append(s.Tests, test)
	return s
}

// AddSharedClient registers a client to be shared across all tests in the suite.
// The client will be started when the suite begins and terminated when the suite ends.
// This is useful for maintaining state across tests for incremental testing or
// avoiding client initialization for every test.
func (s *Suite) AddSharedClient(clientID string, clientType string, options ...StartOption) *Suite {
	if s.SharedClients == nil {
		s.SharedClients = make(map[string]*Client)
	}
	if s.sharedClientOpts == nil {
		s.sharedClientOpts = make(map[string][]StartOption)
	}

	// Store options for later use when the suite is started
	s.sharedClientOpts[clientID] = append([]StartOption{}, options...)

	// Create a placeholder client that will be initialized when the suite runs
	s.SharedClients[clientID] = &Client{
		Type:     clientType,
		IsShared: true,
	}

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
		if host.ll > 3 { // hive log level > 3
			fmt.Fprintf(os.Stderr, "skipping suite %q because it doesn't match test pattern %s\n", suite.Name, host.m.pattern)
		}
		return nil
	}

	suiteID, err := host.StartSuite(suite.request(), "")
	if err != nil {
		return err
	}
	defer host.EndSuite(suiteID)

	// Start shared clients for the suite
	if len(suite.SharedClients) > 0 {
		fmt.Printf("Starting %d shared clients for suite %s...\n", len(suite.SharedClients), suite.Name)

		// Initialize any shared clients defined for this suite
		for clientID, client := range suite.SharedClients {
			// Retrieve stored options for this client
			options := suite.sharedClientOpts[clientID]

			// Start the shared client
			containerID, ip, err := host.StartSharedClient(suiteID, client.Type, options...)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error starting shared client %s: %v\n", clientID, err)
				return err
			}

			// Update the client object with actual container information
			client.Container = containerID
			client.IP = ip
			client.SuiteID = suiteID
			client.IsShared = true

			fmt.Printf("Started shared client %s (container: %s)\n", clientID, containerID)
		}
	}

	// Run all tests in the suite
	for _, test := range suite.Tests {
		if err := test.runTest(host, suiteID, &suite); err != nil {
			return err
		}
	}

	// Clean up any shared clients at the end of the suite
	// They are automatically stopped when the suite ends via defer host.EndSuite(suiteID) above
	// But we should output a message for clarity
	if len(suite.SharedClients) > 0 {
		fmt.Printf("Cleaning up %d shared clients for suite %s...\n", len(suite.SharedClients), suite.Name)
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
//	c := t.Client()
//	c.RPC().Call(...)
//
// or run a subtest using t.RunClientTest():
//
//	t.RunClientTest(hivesim.ClientTestSpec{...})
type TestSpec struct {
	// These fields are displayed in the UI. Be sure to add
	// a meaningful description here.
	Name        string // Name is the unique identifier for the test [Mandatory]
	DisplayName string // Display name for the test (Name will be used if unset) [Optional]
	Description string // Description of the test (if empty, test won't appear in documentation) [Optional]
	Category    string // Category of the test [Optional]

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
	Name        string // Name is the unique identifier for the test [Mandatory]
	DisplayName string // Display name for the test (Name will be used if unset) [Optional]
	Description string // Description of the test (if empty, test won't appear in documentation) [Optional]
	Category    string // Category of the test [Optional]

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

	mu        sync.Mutex
	rpc       *rpc.Client
	enginerpc *rpc.Client
	test      *T

	// Fields for shared client support
	IsShared    bool      // Whether this client is shared across tests
	LogPosition int64     // Current position in the log file (for shared clients)
	SuiteID     SuiteID   // The suite this client belongs to (for shared clients)
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

// EngineAPI returns an RPC client connected to an execution-layer client's engine API server.
func (c *Client) EngineAPI() *rpc.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.enginerpc != nil {
		return c.enginerpc
	}
	auth := rpc.WithHTTPAuth(jwtAuth(ENGINEAPI_JWT_SECRET))
	url := fmt.Sprintf("http://%v:8551", c.IP)
	c.enginerpc, _ = rpc.DialOptions(context.Background(), url, auth)
	return c.enginerpc
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

	// Fields for tracking client logs
	clientLogOffsets map[string]*LogOffset // Tracks log offsets for clients used in this test
}

// StartClient starts a client instance. If the client cannot by started, the test fails immediately.
func (t *T) StartClient(clientType string, option ...StartOption) *Client {
	container, ip, err := t.Sim.StartClientWithOptions(t.SuiteID, t.TestID, clientType, option...)
	if err != nil {
		t.Fatalf("can't launch node (type %s): %v", clientType, err)
	}

	// Initialize log tracking for this client
	if t.clientLogOffsets == nil {
		t.clientLogOffsets = make(map[string]*LogOffset)
	}

	return &Client{
		Type:      clientType,
		Container: container,
		IP:        ip,
		test:      t,
		IsShared:  false,
	}
}

// GetSharedClient retrieves a shared client by ID and prepares it for use in this test.
// The client can be used like a normal Client object, but maintains its state across tests.
// Returns nil if the client doesn't exist.
func (t *T) GetSharedClient(clientID string) *Client {
	if t.suite == nil || t.suite.SharedClients == nil {
		t.Logf("No shared clients available in this suite")
		return nil
	}

	sharedClient, exists := t.suite.SharedClients[clientID]
	if !exists {
		t.Logf("Shared client %q not found", clientID)
		return nil
	}

	// Get the current log position before we use the client
	// This will be the starting position for this test's log segment
	currentLogPosition, err := t.Sim.GetClientLogOffset(t.SuiteID, clientID)
	if err != nil {
		t.Logf("Warning: Failed to get log position for shared client %s: %v", clientID, err)
		// Use the position from the client object as a fallback
		currentLogPosition = sharedClient.LogPosition
	}
	
	// Store the test context in the client so it can be used for this test
	// Create a new Client instance that points to the same container
	client := &Client{
		Type:        sharedClient.Type,
		Container:   sharedClient.Container,
		IP:          sharedClient.IP,
		test:        t,
		IsShared:    true,
		SuiteID:     t.SuiteID,
		LogPosition: currentLogPosition, // Use the current log position as a starting point
	}

	// Initialize log tracking for this client
	if t.clientLogOffsets == nil {
		t.clientLogOffsets = make(map[string]*LogOffset)
	}

	// Record the current log position for this client
	t.clientLogOffsets[clientID] = &LogOffset{
		Start: currentLogPosition,
		End:   0, // Will be set when the test completes
	}

	t.Logf("Using shared client %s with log position %d", clientID, currentLogPosition)

	// Register this shared client with the test using the startClient endpoint with a special parameter
	// This makes it appear in the test's ClientInfo so the UI can display the logs correctly
	var (
		config = simapi.NodeConfig{
			Client:        sharedClient.Type,
			SharedClientID: clientID,
		}
	)

	// Set up a client setup object to post with files (even though we don't have any files)
	setup := &clientSetup{
		files: make(map[string]func() (io.ReadCloser, error)),
		config: config,
	}

	url := fmt.Sprintf("%s/testsuite/%d/test/%d/node", t.Sim.url, t.SuiteID, t.TestID)
	var resp simapi.StartNodeResponse
	err = setup.postWithFiles(url, &resp)
	if err != nil {
		t.Logf("Warning: Failed to register shared client %s with test: %v", clientID, err)
	} else {
		t.Logf("Successfully registered shared client %s with test %d", clientID, t.TestID)
	}

	return client
}

// RunClient runs the given client test against a single client type.
// It waits for the subtest to complete.
func (t *T) RunClient(clientType string, spec ClientTestSpec) {
	test := testSpec{
		suiteID:     t.SuiteID,
		suite:       t.suite,
		name:        clientTestName(spec.Name, clientType),
		displayName: spec.DisplayName,
		category:    spec.Category,
		desc:        spec.Description,
		alwaysRun:   spec.AlwaysRun,
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
	suiteID     SuiteID
	suite       *Suite
	name        string
	displayName string
	category    string
	desc        string
	alwaysRun   bool
}

func (spec testSpec) request() TestStartInfo {
	return TestStartInfo{
		Name:        spec.name,
		DisplayName: spec.displayName,
		Category:    spec.category,
		Description: spec.desc,
	}
}

func runTest(host *Simulation, test testSpec, runit func(t *T)) error {
	if !test.alwaysRun && !host.m.match(test.suite.Name, test.name) {
		if host.ll > 3 { // hive log level > 3
			fmt.Fprintf(os.Stderr, "skipping test %q because it doesn't match test pattern %s\n", test.name, host.m.pattern)
		}
		return nil
	}

	// Register test on simulation server and initialize the T.
	t := &T{
		Sim:     host,
		SuiteID: test.suiteID,
		suite:   test.suite,
		clientLogOffsets: make(map[string]*LogOffset), // Initialize log offset tracking
	}
	testID, err := host.StartTest(test.suiteID, test.request())
	if err != nil {
		return err
	}
	t.TestID = testID
	t.result.Pass = true

	// Capture current log positions for all shared clients before running the test
	if test.suite != nil && test.suite.SharedClients != nil {
		for clientID, client := range test.suite.SharedClients {
			// Get the current log position for each shared client
			logPosition, err := host.GetClientLogOffset(test.suiteID, client.Container)
			if err == nil {
				t.clientLogOffsets[clientID] = &LogOffset{
					Start: logPosition,
					End:   0, // Will be set when test completes
				}
			}
		}
	}

	defer func() {
		t.mu.Lock()

		// After test is complete, update ending log positions for all shared clients
		if test.suite != nil && test.suite.SharedClients != nil {
			for clientID, client := range test.suite.SharedClients {
				if offset, exists := t.clientLogOffsets[clientID]; exists {
					// Get the current log position after test execution
					logPosition, err := host.GetClientLogOffset(test.suiteID, client.Container)
					if err == nil {
						offset.End = logPosition

						// Update the shared client's log position for the next test
						client.LogPosition = logPosition
					}
				}
			}
		}

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
		if host.CollectTestsOnly() && !test.alwaysRun {
			// Don't run the test if we're just generating docs.
			return
		}
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
			suiteID:     suiteID,
			suite:       suite,
			name:        clientTestName(spec.Name, clientDef.Name),
			displayName: spec.DisplayName,
			category:    spec.Category,
			desc:        spec.Description,
			alwaysRun:   spec.AlwaysRun,
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
		suiteID:     suiteID,
		suite:       suite,
		name:        spec.Name,
		displayName: spec.DisplayName,
		category:    spec.Category,
		desc:        spec.Description,
		alwaysRun:   spec.AlwaysRun,
	}
	return runTest(host, test, spec.Run)
}
