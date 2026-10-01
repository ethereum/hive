package main

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/golang-jwt/jwt/v4"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// postRPC preserves the fixture's raw request and the client's raw response,
// including JSON-RPC IDs and errors. Engine calls use Hive's authenticated port
// and the same test secret as hivesim.Client.EngineAPI.
func postRPC(c *http.Client, publicURL, engineURL, data string) ([]byte, error) {
	isEngine := strings.HasPrefix(gjson.Get(data, "method").String(), "engine_")
	url := publicURL
	if isEngine {
		url = engineURL
	}
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if isEngine {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"iat": time.Now().Unix()})
		signed, err := token.SignedString(hivesim.ENGINEAPI_JWT_SECRET[:])
		if err != nil {
			return nil, fmt.Errorf("sign Engine API request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+signed)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if isEngine && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RPC HTTP status: %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

var (
	forkchoiceMethod = regexp.MustCompile(`^engine_forkchoiceUpdatedV[1-9][0-9]*$`)
	payloadIDFormat  = regexp.MustCompile(`^0x[0-9a-f]{16}$`)
)

// normalizePayloadID ignores only the client-specific bytes of a non-null FCU
// payload ID. Null, missing, or malformed IDs must not satisfy a fixture that
// expects a build to start. All other response fields remain exactly compared.
func normalizePayloadID(method, resp, expected string) (string, error) {
	if !forkchoiceMethod.MatchString(method) {
		return resp, nil
	}
	want := gjson.Get(expected, "result.payloadId")
	if want.Type == gjson.Null {
		return resp, nil
	}
	if want.Type != gjson.String || !payloadIDFormat.MatchString(want.Str) {
		return "", fmt.Errorf("fixture payloadId must be null or 8-byte DATA")
	}
	got := gjson.Get(resp, "result.payloadId")
	if got.Type != gjson.String || !payloadIDFormat.MatchString(got.Str) {
		return "", fmt.Errorf("expected a non-null 8-byte payloadId, got %s", got.Raw)
	}
	return sjson.Set(resp, "result.payloadId", want.Str)
}
