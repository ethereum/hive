package hivesim

import (
	"errors"
	"io"
	"net"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
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
	wantClients := []*ClientDefinition{
		{
			Name:    "client-1",
			Version: "client-1-version",
			Meta:    ClientMetadata{Roles: []string{"eth1"}},
		},
		{
			Name:    "client-2",
			Version: "client-2-version",
			Meta:    ClientMetadata{Roles: []string{"beacon"}},
		},
	}
	if !reflect.DeepEqual(ctypes, wantClients) {
		t.Fatalf("wrong client types: %s", spew.Sdump(ctypes))
	}
}

// This checks that the simulator replaces the IP in enode.sh output with the container IP.
func TestEnodeReplaceIP(t *testing.T) {
	// Set up the backend to return enode:// URL containing the
	// localhost IP.
	urlBase := "enode://a61215641fb8714a373c80edbfa0ea8878243193f57c96eeb44d0bc019ef295abd4e044fd619bfc4c59731a73fb79afe84e9ab6da0c743ceb479cbb6d263fa91@"
	hooks := &fakes.BackendHooks{
		RunProgram: func(containerID string, script []string) (*libhive.ExecInfo, error) {
			if len(script) != 1 || script[0] != "/hive-bin/enode.sh" {
				t.Error("hive called wrong client script", script)
				return nil, errors.New("bad script")
			}
			info := &libhive.ExecInfo{Stdout: urlBase + "127.0.0.1:8000"}
			return info, nil
		},
		NetworkNameToID: func(s string) (string, error) {
			return "bridgeID", nil
		},
		ContainerIP: func(string, networkID string) (net.IP, error) {
			if networkID == "bridgeID" {
				return net.ParseIP("192.0.1.1"), nil
			}
			return net.ParseIP("192.0.2.1"), nil
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
	err = sim.CreateNetwork(suiteID, "network1")
	if err != nil {
		t.Fatal("can't create network:", err)
	}

	// Ask for the enode URL on network1. The IP should be corrected to the network1 container IP.
	url, err := sim.ClientEnodeURLNetwork(suiteID, testID, clientID, "network1")
	if err != nil {
		t.Fatal("can't get enode URL:", err)
	}
	want := urlBase + "192.0.2.1:8000"
	if url != want {
		t.Fatalf("wrong enode URL %q\nwant %q", url, want)
	}
	// Ask for the enode URL. The IP should be corrected to the primary container IP.
	url, err = sim.ClientEnodeURL(suiteID, testID, clientID)
	if err != nil {
		t.Fatal("can't get enode URL:", err)
	}
	want = urlBase + "192.0.1.1:8000"
	if url != want {
		t.Fatalf("wrong enode URL %q\nwant %q", url, want)
	}
}

// This test checks the usage of common client start options.
func TestStartClientStartOptions(t *testing.T) {
	var lastOptions libhive.ContainerOptions
	tm, srv := newFakeAPI(&fakes.BackendHooks{
		StartContainer: func(image, containerID string, opt libhive.ContainerOptions) (*libhive.ContainerInfo, error) {
			lastOptions = opt
			return &libhive.ContainerInfo{}, nil
		},
	})
	defer srv.Close()
	defer tm.Terminate()

	// Start the suite and test.
	sim := NewAt(srv.URL)
	suiteID, err := sim.StartSuite("suite", "", "")
	if err != nil {
		t.Fatal("can't start suite:", err)
	}
	testID, err := sim.StartTest(suiteID, "test", "")
	if err != nil {
		t.Fatal("can't start test:", err)
	}

	t.Run("empty_options", func(t *testing.T) {
		// Empty options
		_, _, err = sim.StartClientWithOptions(suiteID, testID, "client-2")
		if err != nil {
			t.Fatalf("failed to start client without any options: %v", err)
		}
	})

	t.Run("bundle_options", func(t *testing.T) {
		// Params with overrides
		_, _, err = sim.StartClientWithOptions(suiteID, testID, "client-1",
			Bundle(Params{"HIVE_FOO": "1"}, Params{"HIVE_BAR": "2"}))
		if err != nil {
			t.Fatalf("failed to start client: %v", err)
		}
		if got := lastOptions.Env["HIVE_FOO"]; got != "1" {
			t.Fatalf("wrong HIVE_FOO, got: %s", got)
		}
		if got := lastOptions.Env["HIVE_BAR"]; got != "2" {
			t.Fatalf("wrong HIVE_BAR, got: %s", got)
		}
	})

	t.Run("params_options", func(t *testing.T) {
		// Params with overrides
		_, _, err = sim.StartClientWithOptions(suiteID, testID, "client-1",
			Params{"HIVE_FOO": "1", "HIVE_BAR": "2"}, Params{"HIVE_FOO": "3"})
		if err != nil {
			t.Fatalf("failed to start client: %v", err)
		}
		if got := lastOptions.Env["HIVE_FOO"]; got != "3" {
			t.Fatalf("2nd option failed to overwrite param of 1st option, got: %s", got)
		}
		if got := lastOptions.Env["HIVE_BAR"]; got != "2" {
			t.Fatalf("non-overwritten option changed or went missing, got: %s", got)
		}
	})

	t.Run("files_options", func(t *testing.T) {
		file1, err := os.CreateTemp("", "hivesim_test")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file1.WriteString("aaa"); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(file1.Name())

		file2, err := os.CreateTemp("", "hivesim_test")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file2.WriteString("bb"); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(file2.Name())

		t.Run("static", func(t *testing.T) {
			// Static files with override of /data/foo
			_, _, err = sim.StartClientWithOptions(suiteID, testID, "client-1",
				WithStaticFiles(map[string]string{"/data/foo": "/tmp/bad", "foo": file1.Name()}),
				WithStaticFiles(map[string]string{"/data/foo": file2.Name()}))
			if err != nil {
				t.Fatalf("failed to start client: %v", err)
			}
			got, ok := lastOptions.Files["/data/foo"]
			if !ok {
				t.Fatal("missing /data/foo")
			}
			if got.Size != 2 {
				t.Fatalf("expected 2 bytes for '/data/foo', got: %d", got.Size)
			}
			got, ok = lastOptions.Files["foo"]
			if !ok {
				t.Fatal("missing foo") // same file name as /data/foo, but no path, thus different.
			}
			if got.Size != 3 {
				t.Fatalf("expected 3 bytes for 'foo', got: %d", got.Size)
			}
		})

		mockSrc := func(content string) func() (io.ReadCloser, error) {
			return func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(content)), nil }
		}

		t.Run("dynamic", func(t *testing.T) {
			// Dynamic files with override of /data/bar, and override static file too.
			_, _, err = sim.StartClientWithOptions(suiteID, testID, "client-1",
				WithDynamicFile("/data/bar", func() (io.ReadCloser, error) {
					t.Fatal("this should have been overridden")
					return nil, nil
				}),
				WithStaticFiles(map[string]string{"/data/bar": file1.Name()}),
				WithDynamicFile("/data/bar", mockSrc("dddddd")))
			if err != nil {
				t.Fatalf("failed to start client: %v", err)
			}
			got, ok := lastOptions.Files["/data/bar"]
			if !ok {
				t.Fatal("missing /data/bar")
			}
			if got.Size != 6 {
				t.Fatalf("expected 6 bytes for '/data/bar', got %d", got.Size)
			}
		})
	})
}

// This checks running scripts in a client container.
func TestRunProgram(t *testing.T) {
	hooks := &fakes.BackendHooks{
		RunProgram: func(containerID string, cmd []string) (*libhive.ExecInfo, error) {
			if len(cmd) > 0 && cmd[0] == "/hive-bin/echo" {
				info := &libhive.ExecInfo{
					Stdout:   "script: " + strings.Join(cmd, " "),
					Stderr:   "error output",
					ExitCode: 42,
				}
				return info, nil
			}
			return nil, errors.New("invalid script")
		},
	}
	tm, srv := newFakeAPI(hooks)
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

	params := map[string]string{"CLIENT": "client-1"}
	clientID, _, err := sim.StartClient(suiteID, testID, params, nil)
	if err != nil {
		t.Fatal("can't start client:", err)
	}

	// Run the echo script.
	res, err := sim.ClientExec(suiteID, testID, clientID, []string{"echo", "this"})
	if err != nil {
		t.Fatal("failed to run program:", err)
	}
	if want := "script: /hive-bin/echo this"; res.Stdout != want {
		t.Fatalf("wrong std out %q\nwant %q", res.Stdout, want)
	}
	if want := "error output"; res.Stderr != want {
		t.Fatalf("wrong std err %q\nwant %q", res.Stderr, want)
	}
	if want := 42; res.ExitCode != want {
		t.Fatalf("wrong code %q\nwant %q", res.ExitCode, want)
	}

	// Run a script that doesn't exist.
	_, err = sim.ClientExec(suiteID, testID, clientID, []string{"a-script"})
	if err == nil {
		t.Fatal("no error from ClientExec for non-existent script")
	}
	if err.Error() != "invalid script" {
		t.Fatalf("wrong error message for non-existent script: %q", err)
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
		t.Fatalf("wanted error for unknown client parameter, got container ID %v", clientID)
	}
	if !strings.Contains(err.Error(), "unknown client type") {
		t.Fatalf("wrong error for GetNode with unknown client parameter: %q", err.Error())
	}
}

func TestStartClientInitialNetworks(t *testing.T) {
	var (
		connections = make(map[string]net.IP)
		ipcounter   byte
	)
	tm, srv := newFakeAPI(&fakes.BackendHooks{
		StartContainer: func(image, containerID string, opt libhive.ContainerOptions) (*libhive.ContainerInfo, error) {
			return &libhive.ContainerInfo{}, nil
		},
		ConnectContainer: func(containerID string, networkID string) error {
			ipcounter++
			connections[containerID+networkID] = net.IP{203, 0, 113, ipcounter}
			return nil
		},
		ContainerIP: func(containerID string, networkID string) (net.IP, error) {
			ip, ok := connections[containerID+networkID]
			if !ok {
				return nil, errors.New("container not connected")
			}
			return ip, nil
		},
	})
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

	// Create networks.
	sim.CreateNetwork(suiteID, "Init Network 1")
	sim.CreateNetwork(suiteID, "Init Network 2")
	sim.CreateNetwork(suiteID, "Init Network 3")
	defer sim.RemoveNetwork(suiteID, "Init Network 1")
	defer sim.RemoveNetwork(suiteID, "Init Network 2")
	defer sim.RemoveNetwork(suiteID, "Init Network 3")

	// Start the client.
	opt := WithInitialNetworks([]string{"Init Network 1", "Init Network 3"})
	containerID, _, err := sim.StartClientWithOptions(suiteID, testID, "client-1", opt)
	if err != nil {
		t.Fatalf("failed to start client: %v", err)
	}
	if ip, _ := sim.ContainerNetworkIP(suiteID, "Init Network 1", containerID); ip != "203.0.113.1" {
		t.Fatalf("network 1 was not connected at start: %v", ip)
	}
	if ip, _ := sim.ContainerNetworkIP(suiteID, "Init Network 2", containerID); ip != "" {
		t.Fatalf("network 2 was incorrectly connected at start: %v", ip)
	}
	if ip, _ := sim.ContainerNetworkIP(suiteID, "Init Network 3", containerID); ip != "203.0.113.2" {
		t.Fatalf("network 3 was not connected at start: %v", ip)
	}
}

func newFakeAPI(hooks *fakes.BackendHooks) (*libhive.TestManager, *httptest.Server) {
	defs := map[string]*libhive.ClientDefinition{
		"client-1": {Name: "client-1", Image: "/ignored/in/api", Version: "client-1-version", Meta: libhive.ClientMetadata{Roles: []string{"eth1"}}},
		"client-2": {Name: "client-2", Image: "/not/exposed/", Version: "client-2-version", Meta: libhive.ClientMetadata{Roles: []string{"beacon"}}},
	}
	env := libhive.SimEnv{}
	backend := fakes.NewContainerBackend(hooks)
	tm := libhive.NewTestManager(env, backend, defs)
	srv := httptest.NewServer(tm.API())
	return tm, srv
}
