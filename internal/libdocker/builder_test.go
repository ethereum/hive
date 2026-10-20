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
	tests := []struct {
		name, ext, context, wantDockerfile string
	}{
		{name: "default", wantDockerfile: "Dockerfile"},
		{name: "git", ext: "git", wantDockerfile: "Dockerfile.git"},
		{name: "default/context", context: "../..", wantDockerfile: "simulators/test/Dockerfile"},
		{name: "git/context", ext: "git", context: "../..", wantDockerfile: "simulators/test/Dockerfile.git"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := libhive.Inventory{BaseDir: t.TempDir()}
			sim := libhive.SimulatorDesignator{Simulator: "test", DockerfileExt: tt.ext, BuildArgs: map[string]string{"branch": "main"}}
			dir := inv.SimulatorDirectory(sim.Simulator)
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, sim.Dockerfile()), []byte("FROM scratch\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if tt.context != "" {
				if err := os.WriteFile(filepath.Join(dir, "hive_context.txt"), []byte(tt.context+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
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
				if got := q.Get("dockerfile"); got != tt.wantDockerfile {
					t.Errorf("Dockerfile = %q, want %q", got, tt.wantDockerfile)
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
					if hdr.Name == tt.wantDockerfile {
						found = true
					}
				}
				if !found {
					t.Errorf("build context does not contain %s", tt.wantDockerfile)
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
