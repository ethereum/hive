package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/golang-jwt/jwt/v4"
)

func TestPostRPCRouting(t *testing.T) {
	for _, method := range []string{"eth_blockNumber", "engine_forkchoiceUpdatedV4"} {
		t.Run(method, func(t *testing.T) {
			request := `{"jsonrpc":"2.0","id":"fixture-id","method":"` + method + `","params":[]}`
			// Error envelopes must reach the fixture comparison unchanged too.
			response := `{"jsonrpc":"2.0","id":"fixture-id","error":{"code":-32602,"message":"Invalid params","data":"detail"}}`
			engine := strings.HasPrefix(method, "engine_")
			handler := func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
					t.Error("expected a JSON POST")
				}
				body, _ := io.ReadAll(r.Body)
				if string(body) != request {
					t.Errorf("request changed: %s", body)
				}
				auth := r.Header.Get("Authorization")
				if !engine && auth != "" {
					t.Error("JWT sent to public RPC")
				}
				if engine {
					token, err := jwt.Parse(strings.TrimPrefix(auth, "Bearer "), func(token *jwt.Token) (interface{}, error) {
						return hivesim.ENGINEAPI_JWT_SECRET[:], nil
					}, jwt.WithValidMethods([]string{"HS256"}))
					if err != nil || !token.Valid {
						t.Errorf("invalid Engine JWT: %v", err)
					} else {
						iat, ok := token.Claims.(jwt.MapClaims)["iat"].(float64)
						if !ok || time.Since(time.Unix(int64(iat), 0)).Abs() > time.Minute {
							t.Error("Engine JWT has no fresh iat")
						}
					}
				}
				io.WriteString(w, response)
			}
			server := httptest.NewServer(http.HandlerFunc(handler))
			defer server.Close()
			// A request routed to the wrong endpoint fails instead of being accepted.
			publicURL, engineURL := server.URL, "http://127.0.0.1:0"
			if engine {
				publicURL, engineURL = engineURL, publicURL
			}
			got, err := postRPC(server.Client(), publicURL, engineURL, request)
			if err != nil || string(got) != response {
				t.Fatalf("response = %s, error = %v", got, err)
			}
		})
	}
}

func TestPostRPCUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	_, err := postRPC(server.Client(), "", server.URL, `{"method":"engine_exchangeCapabilities"}`)
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected HTTP auth error, got %v", err)
	}
}

func TestForkchoiceFixtureComparison(t *testing.T) {
	const valid = `{"jsonrpc":"2.0","id":1,"result":{"payloadStatus":{"status":"VALID","latestValidHash":"0x1111","validationError":null},"payloadId":"0x0102030405060708"}}`
	tests := []struct {
		name, response string
		fail           bool
	}{
		{"same", valid, false},
		{"different-id", strings.ReplaceAll(valid, "0102030405060708", "aabbccddeeff0011"), false},
		{"null-id", strings.ReplaceAll(valid, `"0x0102030405060708"`, `null`), true},
		{"missing-id", strings.ReplaceAll(valid, `,"payloadId":"0x0102030405060708"`, ``), true},
		{"short-id", strings.ReplaceAll(valid, "0102030405060708", "0102"), true},
		{"long-id", strings.ReplaceAll(valid, "0102030405060708", "010203040506070809"), true},
		{"non-hex-id", strings.ReplaceAll(valid, "0102030405060708", "zzzzzzzzzzzzzzzz"), true},
		{"uppercase-id", strings.ReplaceAll(valid, "0102030405060708", "AABBCCDDEEFF0011"), true},
		{"numeric-id", strings.ReplaceAll(valid, `"0x0102030405060708"`, `123`), true},
		{"syncing", strings.ReplaceAll(valid, "VALID", "SYNCING"), true},
		{"invalid", strings.ReplaceAll(valid, "VALID", "INVALID"), true},
		{"wrong-ancestor", strings.ReplaceAll(valid, "0x1111", "0x2222"), true},
		{"wrong-envelope-id", strings.ReplaceAll(valid, `"id":1`, `"id":2`), true},
		{"rpc-error", `{"jsonrpc":"2.0","id":1,"error":{"code":-38003,"message":"Invalid payload attributes"}}`, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fixture, err := loadTestFile("fcu", strings.NewReader(">> {\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"engine_forkchoiceUpdatedV4\",\"params\":[]}\n<< "+valid))
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.WriteString(w, tc.response)
			}))
			defer server.Close()
			err = runRPCTest(t, server.Client(), "http://127.0.0.1:0", server.URL, &fixture)
			if (err != nil) != tc.fail {
				t.Fatalf("error = %v, want failure %v", err, tc.fail)
			}
		})
	}
}

func TestPayloadIDNormalizationScope(t *testing.T) {
	const response = `{"result":{"payloadId":"0xaabbccddeeff0011"}}`
	for _, tc := range []struct{ method, expected string }{
		{"eth_example", `{"result":{"payloadId":"0x0102030405060708"}}`},
		{"engine_forkchoiceUpdatedV4", `{"result":{"payloadId":null}}`},
		{"engine_forkchoiceUpdatedV4", `{"error":{"code":-38003}}`},
	} {
		got, err := normalizePayloadID(tc.method, response, tc.expected)
		if err != nil || got != response {
			t.Fatalf("unexpected normalization: %s, %v", got, err)
		}
	}
}
