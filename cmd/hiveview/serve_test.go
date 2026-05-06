package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
