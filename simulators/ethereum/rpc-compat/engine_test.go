package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/execution-apis/tools/iofile"
	"github.com/ethereum/execution-apis/tools/spec"
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

func TestEngineFixtureValidation(t *testing.T) {
	schema, err := spec.LoadString("engine-test", `{"openrpc":"1.4.1","info":{"title":"test","version":"1"},"methods":[{"name":"engine_exchangeCapabilities","params":[],"result":{"name":"capabilities","schema":{"type":"array","items":{"type":"string"}}}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, result   string
		specOnly, fail bool
	}{
		{"script-pass", `["actual"]`, true, false},
		{"script-fail", `["unexpected"]`, true, true},
		{"schema-fail", `[42]`, true, true},
		{"literal-comparison", `["actual"]`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture, err := iofile.Load("engine.io", strings.NewReader(`
>> {"jsonrpc":"2.0","id":1,"method":"engine_exchangeCapabilities","params":[]}
<< {"jsonrpc":"2.0","id":1,"result":["recorded"]}
--
if (messages[1].response.result[0] !== "actual") throw new Error("script rejected actual response");
`))
			if err != nil {
				t.Fatal(err)
			}
			fixture.SpecOnly = tc.specOnly
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
					t.Error("missing Engine JWT")
				}
				io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":`+tc.result+`}`)
			}))
			defer server.Close()
			err = runRPCTest(t, server.Client(), "http://127.0.0.1:0", server.URL, &fixture, iofile.ScriptConfig{Log: t, OpenRPC: schema})
			if (err != nil) != tc.fail {
				t.Fatalf("error = %v, want failure %v", err, tc.fail)
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
