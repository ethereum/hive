package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestServeFilesLeanLatestRedirect(t *testing.T) {
	handler := serveFiles{fsys: fstest.MapFS{
		"index.html": {Data: []byte("ok")},
	}}

	tests := []struct {
		path         string
		wantStatus   int
		wantLocation string
	}{
		{
			path:         "/lean-latest",
			wantStatus:   http.StatusMovedPermanently,
			wantLocation: "/",
		},
		{
			path:         "/lean-latest/",
			wantStatus:   http.StatusMovedPermanently,
			wantLocation: "/",
		},
		{
			path:         "/lean-latest.html?devnet=devnet4",
			wantStatus:   http.StatusMovedPermanently,
			wantLocation: "/?devnet=devnet4",
		},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != test.wantStatus {
				t.Fatalf("status mismatch: got %d, want %d", rec.Code, test.wantStatus)
			}
			if loc := rec.Header().Get("Location"); loc != test.wantLocation {
				t.Fatalf("location mismatch: got %q, want %q", loc, test.wantLocation)
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
