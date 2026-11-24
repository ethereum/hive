package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/hive/cmd/hq/internal/cache"
)

func TestSuitePaths(t *testing.T) {
	tests := []struct {
		suite string
		want  []string
	}{
		{"generic", []string{"/generic/listing.jsonl", "/generic/results/run.json", "/generic/results/details.log"}},
		// A local hiveview serves one suite at the root.
		{"", []string{"/listing.jsonl", "/results/run.json", "/results/details.log"}},
	}
	for _, tc := range tests {
		t.Run("suite="+tc.suite, func(t *testing.T) {
			var got []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = append(got, r.URL.Path)
				w.Write([]byte("{}"))
			}))
			defer srv.Close()
			c := NewClient(srv.URL, tc.suite, &cache.Cache{Dir: t.TempDir()})

			if _, err := c.FetchListing("", "", 0); err != nil {
				t.Fatal(err)
			}
			if _, err := c.FetchResult("run.json"); err != nil {
				t.Fatal(err)
			}
			if _, err := c.FetchTestLog("details.log", 0, 2); err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("requested paths %q, want %q", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("request %d: path %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}
