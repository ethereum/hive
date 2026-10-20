package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/golang-jwt/jwt/v4"
	"github.com/tidwall/gjson"
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
