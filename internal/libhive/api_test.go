package libhive

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/hive/internal/simapi"
	"github.com/gorilla/mux"
)

type mockBackend struct{}

func (m mockBackend) Build(ctx context.Context, b Builder) error { return nil }
func (m mockBackend) SetHiveInstanceInfo(instanceID, version string) {}
func (m mockBackend) GetDockerClient() interface{} { return nil }
func (m mockBackend) ServeAPI(ctx context.Context, h http.Handler) (APIServer, error) { return nil, nil }
func (m mockBackend) CreateContainer(ctx context.Context, image string, opt ContainerOptions) (string, error) { return "", nil }
func (m mockBackend) StartContainer(ctx context.Context, containerID string, opt ContainerOptions) (*ContainerInfo, error) { return nil, nil }
func (m mockBackend) DeleteContainer(containerID string) error { return nil }
func (m mockBackend) PauseContainer(containerID string) error { return nil }
func (m mockBackend) UnpauseContainer(containerID string) error { return nil }
func (m mockBackend) RunProgram(ctx context.Context, containerID string, cmdline []string) (*ExecInfo, error) { return nil, nil }
func (m mockBackend) NetworkNameToID(name string) (string, error) { return "", nil }
func (m mockBackend) CreateNetwork(name string) (string, error) { return "", nil }
func (m mockBackend) RemoveNetwork(id string) error { return nil }
func (m mockBackend) ContainerIP(containerID, networkID string) (net.IP, error) { return nil, nil }
func (m mockBackend) ConnectContainer(containerID, networkID string) error { return nil }
func (m mockBackend) DisconnectContainer(containerID, networkID string) error { return nil }

func TestRegisterRoutes(t *testing.T) {
	api := &simAPI{
		backend: mockBackend{},
		env:     SimEnv{},
		tm:      &TestManager{},
		hive:    HiveInfo{},
	}
	router := mux.NewRouter()
	api.registerRoutes(router)

	// Test that all expected routes are registered
	routes := []struct {
		path   string
		method string
	}{
		{"/hive", "GET"},
		{"/clients", "GET"},
		{"/testsuite", "POST"},
		{"/testsuite/{suite}", "DELETE"},
		{"/testsuite/{suite}/test", "POST"},
		{"/testsuite/{suite}/test/{test}", "POST"},
		{"/testsuite/{suite}/test/{test}/node", "POST"},
		{"/testsuite/{suite}/test/{test}/node/{node}", "DELETE"},
		{"/testsuite/{suite}/test/{test}/node/{node}", "GET"},
		{"/testsuite/{suite}/test/{test}/node/{node}/pause", "POST"},
		{"/testsuite/{suite}/test/{test}/node/{node}/pause", "DELETE"},
		{"/testsuite/{suite}/test/{test}/node/{node}/exec", "POST"},
		{"/testsuite/{suite}/network/{network}", "POST"},
		{"/testsuite/{suite}/network/{network}", "DELETE"},
		{"/testsuite/{suite}/network/{network}/{node}", "GET"},
		{"/testsuite/{suite}/network/{network}/{node}", "POST"},
		{"/testsuite/{suite}/network/{network}/{node}", "DELETE"},
	}

	for _, route := range routes {
		if !router.Match(&http.Request{Method: route.method, URL: &url.URL{Path: route.path}}, &mux.RouteMatch{}) {
			t.Errorf("Route %s %s not registered", route.method, route.path)
		}
	}
}

func TestErrorHandling(t *testing.T) {
	api := &simAPI{
		backend: mockBackend{},
		env:     SimEnv{},
		tm:      &TestManager{},
		hive:    HiveInfo{},
	}

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		request    *http.Request
		wantStatus int
		wantError  string
	}{
		{
			name:       "Invalid JSON in startSuite",
			handler:    api.startSuite,
			request:    httptest.NewRequest("POST", "/testsuite", strings.NewReader("invalid json")),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid JSON in request body",
		},
		{
			name:       "Empty suite name",
			handler:    api.startSuite,
			request:    httptest.NewRequest("POST", "/testsuite", strings.NewReader(`{"name": "", "description": "test"}`)),
			wantStatus: http.StatusBadRequest,
			wantError:  "suite name is empty",
		},
		{
			name:       "Invalid test suite ID",
			handler:    api.endSuite,
			request:    httptest.NewRequest("DELETE", "/testsuite/invalid", nil),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid test suite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tt.handler(w, tt.request)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			var response simapi.Error
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatal(err)
			}

			if !strings.Contains(response.Error, tt.wantError) {
				t.Errorf("got error %q, want %q", response.Error, tt.wantError)
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	api := &simAPI{
		backend: mockBackend{},
		env: SimEnv{
			SimLogLevel: 3,
		},
		tm:   &TestManager{},
		hive: HiveInfo{},
	}

	tests := []struct {
		name     string
		config   *simapi.NodeConfig
		wantEnv  map[string]string
	}{
		{
			name: "Basic environment",
			config: &simapi.NodeConfig{
				Environment: map[string]string{
					"HIVE_LOGLEVEL": "5",
					"OTHER_VAR":     "value",
				},
			},
			wantEnv: map[string]string{
				"HIVE_LOGLEVEL": "5",
			},
		},
		{
			name: "Default loglevel",
			config: &simapi.NodeConfig{
				Environment: map[string]string{
					"OTHER_VAR": "value",
				},
			},
			wantEnv: map[string]string{
				"HIVE_LOGLEVEL": "3",
			},
		},
		{
			name: "Multiple HIVE variables",
			config: &simapi.NodeConfig{
				Environment: map[string]string{
					"HIVE_LOGLEVEL": "5",
					"HIVE_TEST":     "value",
					"OTHER_VAR":     "value",
				},
			},
			wantEnv: map[string]string{
				"HIVE_LOGLEVEL": "5",
				"HIVE_TEST":     "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := api.prepareClientEnvironment(tt.config)
			if !reflect.DeepEqual(got, tt.wantEnv) {
				t.Errorf("prepareClientEnvironment() = %v, want %v", got, tt.wantEnv)
			}
		})
	}
}

func TestClientStartTimeout(t *testing.T) {
	tests := []struct {
		name     string
		env      SimEnv
		want     time.Duration
	}{
		{
			name: "Default timeout",
			env:  SimEnv{},
			want: defaultStartTimeout,
		},
		{
			name: "Custom timeout",
			env: SimEnv{
				ClientStartTimeout: 30 * time.Second,
			},
			want: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &simAPI{
				backend: mockBackend{},
				env:     tt.env,
				tm:      &TestManager{},
				hive:    HiveInfo{},
			}
			got := api.getClientStartTimeout()
			if got != tt.want {
				t.Errorf("getClientStartTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
} 