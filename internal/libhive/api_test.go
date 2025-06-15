package libhive

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

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