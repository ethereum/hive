package hivesim

import (
	"fmt"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/hive/internal/fakes"
	"github.com/ethereum/hive/internal/libhive"
)

// This test checks that the API returns configured client names correctly.
func TestClientTypes(t *testing.T) {
	tm, srv := newFakeAPI(nil)
	defer srv.Close()
	defer tm.Terminate()

	sim := NewAt(srv.URL)
	ctypes, err := sim.ClientTypes()
	if err != nil {
		t.Fatal("can't get client types:", err)
	}
	if !reflect.DeepEqual(ctypes, []string{"client-1", "client-2"}) {
		t.Fatal("wrong client types:", ctypes)
	}
}

// This checks that the simulator replaces the IP in enode.sh output with the container IP.
func TestEnodeReplaceIP(t *testing.T) {
	// Set up the backend to return enode:// URL containing the
	// localhost IP.
	urlBase := "enode://a61215641fb8714a373c80edbfa0ea8878243193f57c96eeb44d0bc019ef295abd4e044fd619bfc4c59731a73fb79afe84e9ab6da0c743ceb479cbb6d263fa91@"
	hooks := &fakes.BackendHooks{
		RunEnodeSh: func(string) (string, error) {
			return urlBase + "127.0.0.1:8000", nil
		},
	}
	tm, srv := newFakeAPI(hooks)
	defer srv.Close()
	defer tm.Terminate()

	// Start the client.
	sim := NewAt(srv.URL)
	suiteID, err := sim.StartSuite("suite", "", "")
	if err != nil {
		t.Fatal("can't start suite:", err)
	}
	testID, err := sim.StartTest(suiteID, "test", "")
	if err != nil {
		t.Fatal("can't start test:", err)
	}
	params := map[string]string{"CLIENT": "client-1"}
	clientID, _, err := sim.StartClient(suiteID, testID, params, nil)
	if err != nil {
		t.Fatal("can't start client:", err)
	}

	// Ask for the enode URL. The IP should be corrected to the primary container IP.
	url, err := sim.ClientEnodeURL(suiteID, testID, clientID)
	if err != nil {
		t.Fatal("can't get enode URL:", err)
	}
	want := urlBase + "192.0.2.1:8000"
	if url != want {
		t.Fatalf("wrong enode URL %q\nwant %q", url, want)
	}
}

// This checks that the simulator can run a program
func TestRunProgram(t *testing.T) {
	// Set up the backend to return program execution. Simple debug program here.
	hooks := &fakes.BackendHooks{
		RunProgram: func(containerID string, opt libhive.ExecOptions) (*libhive.ExecInfo, error) {
			return &libhive.ExecInfo{
				StdOut:   fmt.Sprintf("user: %s, privileged: %v", opt.User, opt.Privileged),
				StdErr:   "cmd: " + strings.Join(opt.Cmd, ","),
				ExitCode: 42,
			}, nil
		},
	}
	tm, srv := newFakeAPI(hooks)
	defer srv.Close()
	defer tm.Terminate()

	// Start the client.
	sim := NewAt(srv.URL)
	suiteID, err := sim.StartSuite("suite", "", "")
	if err != nil {
		t.Fatal("can't start suite:", err)
	}
	testID, err := sim.StartTest(suiteID, "test", "")
	if err != nil {
		t.Fatal("can't start test:", err)
	}
	params := map[string]string{"CLIENT": "client-1"}
	clientID, _, err := sim.StartClient(suiteID, testID, params, nil)
	if err != nil {
		t.Fatal("can't start client:", err)
	}

	// Run a program
	stdOut, stdErr, code, err := sim.ClientRunProgram(
		suiteID, testID, clientID, true, "ether", "echo this")
	if err != nil {
		t.Fatal("failed to run program:", err)
	}

	if want := "user: ether, privileged: true"; stdOut != want {
		t.Fatalf("wrong std out %q\nwant %q", stdOut, want)
	}
	if want := "cmd: echo this"; stdErr != want {
		t.Fatalf("wrong std err %q\nwant %q", stdErr, want)
	}
	if want := 42; code != want {
		t.Fatalf("wrong code %q\nwant %q", code, want)
	}
}

// This test checks for some common errors returned by StartClient.
func TestStartClientErrors(t *testing.T) {
	tm, srv := newFakeAPI(nil)
	defer srv.Close()
	defer tm.Terminate()

	sim := NewAt(srv.URL)
	suiteID, err := sim.StartSuite("suite", "", "")
	if err != nil {
		t.Fatal("can't start suite:", err)
	}
	testID, err := sim.StartTest(suiteID, "test", "")
	if err != nil {
		t.Fatal("can't start test:", err)
	}

	// Need CLIENT to start the client.
	params := map[string]string{}
	clientID, _, err := sim.StartClient(suiteID, testID, params, nil)
	if err == nil {
		t.Fatalf("wanted error from GetNode without CLIENT parameter, got container ID %v", clientID)
	}
	if !strings.Contains(err.Error(), "missing 'CLIENT'") {
		t.Fatalf("wrong error for GetNode without CLIENT parameter: %q", err.Error())
	}

	// Unknown CLIENT.
	params = map[string]string{"CLIENT": "unknown"}
	clientID, _, err = sim.StartClient(suiteID, testID, params, nil)
	if err == nil {
		t.Fatalf("wanted error for unknown CLIENT parameter, got container ID %v", clientID)
	}
	if !strings.Contains(err.Error(), "unknown 'CLIENT'") {
		t.Fatalf("wrong error for GetNode with unknown CLIENT parameter: %q", err.Error())
	}
}

func newFakeAPI(hooks *fakes.BackendHooks) (*libhive.TestManager, *httptest.Server) {
	env := libhive.SimEnv{
		Images: map[string]string{
			"client-1": "client-1-image",
			"client-2": "client-2-image",
		},
	}
	backend := fakes.NewContainerBackend(hooks)
	tm := libhive.NewTestManager(env, backend, -1)
	srv := httptest.NewServer(tm.API())
	return tm, srv
}
