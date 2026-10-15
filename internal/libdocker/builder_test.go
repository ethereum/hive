package libdocker

import (
	"archive/tar"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/hive/internal/libhive"
	docker "github.com/fsouza/go-dockerclient"
)

func TestBuildSimulatorImage(t *testing.T) {
	for _, overrideContext := range []bool{false, true} {
		for _, ext := range []string{"", "git"} {
			name := ext
			if name == "" {
				name = "default"
			}
			if overrideContext {
				name += "/context"
			}
			t.Run(name, func(t *testing.T) {
				inv := libhive.Inventory{BaseDir: t.TempDir()}
				sim := libhive.SimulatorDesignator{Simulator: "test", DockerfileExt: ext, BuildArgs: map[string]string{"branch": "main"}}
				dir := inv.SimulatorDirectory(sim.Simulator)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, sim.Dockerfile()), []byte("FROM scratch\n"), 0600); err != nil {
					t.Fatal(err)
				}
				wantDockerfile := sim.Dockerfile()
				if overrideContext {
					if err := os.WriteFile(filepath.Join(dir, "hive_context.txt"), []byte("../..\n"), 0600); err != nil {
						t.Fatal(err)
					}
					wantDockerfile = "simulators/test/" + sim.Dockerfile()
				}
				called := false
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if !strings.HasSuffix(r.URL.Path, "/build") {
						w.Header().Set("Content-Type", "application/json")
						io.WriteString(w, `{"ApiVersion":"1.41"}`)
						return
					}
					called = true
					q := r.URL.Query()
					if got := q.Get("dockerfile"); got != wantDockerfile {
						t.Errorf("Dockerfile = %q, want %q", got, wantDockerfile)
					}
					if got := q.Get("t"); got != "hive/simulators/test:latest" {
						t.Errorf("image tag = %q", got)
					}
					var args map[string]string
					if err := json.Unmarshal([]byte(q.Get("buildargs")), &args); err != nil {
						t.Error(err)
					}
					if !reflect.DeepEqual(args, sim.BuildArgs) {
						t.Errorf("build args = %v", args)
					}
					archive := tar.NewReader(r.Body)
					found := false
					for {
						hdr, err := archive.Next()
						if err == io.EOF {
							break
						}
						if err != nil {
							t.Error(err)
							break
						}
						if hdr.Name == wantDockerfile {
							found = true
						}
					}
					if !found {
						t.Errorf("build context does not contain %s", wantDockerfile)
					}
					w.Header().Set("Content-Type", "application/json")
					io.WriteString(w, "{}\n")
				}))
				defer server.Close()
				client, err := docker.NewClient(server.URL)
				if err != nil {
					t.Fatal(err)
				}
				builder := NewBuilder(client, &Config{Inventory: inv}, nil)
				if _, err := builder.BuildSimulatorImage(context.Background(), sim); err != nil {
					t.Fatal(err)
				}
				if !called {
					t.Fatal("no Docker build request")
				}
			})
		}
	}
}
