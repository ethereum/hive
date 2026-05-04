package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestServeFilesLeanLatestFlag(t *testing.T) {
	fsys := fstest.MapFS{
		"lean-latest.html": {Data: []byte("ok")},
	}

	tests := []struct {
		name              string
		path              string
		enableLeanLatest  bool
		wantStatus        int
		wantLocation      string
		wantCacheDisabled bool
	}{
		{
			name:       "disabled route",
			path:       "/lean-latest",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "disabled trailing slash",
			path:       "/lean-latest/",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "disabled html file",
			path:       "/lean-latest.html",
			wantStatus: http.StatusNotFound,
		},
		{
			name:              "enabled route",
			path:              "/lean-latest",
			enableLeanLatest:  true,
			wantStatus:        http.StatusOK,
			wantCacheDisabled: true,
		},
		{
			name:              "enabled html file",
			path:              "/lean-latest.html",
			enableLeanLatest:  true,
			wantStatus:        http.StatusOK,
			wantCacheDisabled: true,
		},
		{
			name:             "enabled trailing slash redirects",
			path:             "/lean-latest/",
			enableLeanLatest: true,
			wantStatus:       http.StatusMovedPermanently,
			wantLocation:     "/lean-latest",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := serveFiles{
				fsys:             fsys,
				enableLeanLatest: test.enableLeanLatest,
			}
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != test.wantStatus {
				t.Fatalf("status mismatch: got %d, want %d", rec.Code, test.wantStatus)
			}
			if loc := rec.Header().Get("Location"); loc != test.wantLocation {
				t.Fatalf("location mismatch: got %q, want %q", loc, test.wantLocation)
			}
			if cache := rec.Header().Get("cache-control"); test.wantCacheDisabled && cache != "no-cache" {
				t.Fatalf("cache-control mismatch: got %q, want no-cache", cache)
			}
		})
	}
}

func TestServeFeatures(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/features.json", nil)

			serveFeatures{enableLeanLatest: enabled}.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status mismatch: got %d, want %d", rec.Code, http.StatusOK)
			}
			if cache := rec.Header().Get("cache-control"); cache != "no-cache" {
				t.Fatalf("cache-control mismatch: got %q, want no-cache", cache)
			}

			var features map[string]bool
			if err := json.NewDecoder(rec.Body).Decode(&features); err != nil {
				t.Fatal(err)
			}
			if features["leanLatest"] != enabled {
				t.Fatalf("leanLatest mismatch: got %v, want %v", features["leanLatest"], enabled)
			}
		})
	}
}
