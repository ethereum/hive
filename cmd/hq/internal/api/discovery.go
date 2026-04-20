package api

import (
	"encoding/json"
	"fmt"
	"time"
)

const volatileTTL = 5 * time.Minute

// FetchDiscovery returns the list of suites advertised by the server.
func (c *Client) FetchDiscovery() ([]Discovery, error) {
	url := fmt.Sprintf("%s/discovery.json", c.BaseURL)
	data, err := c.fetch(url, volatileTTL)
	if err != nil {
		return nil, err
	}
	var result []Discovery
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing discovery.json: %w", err)
	}
	return result, nil
}
