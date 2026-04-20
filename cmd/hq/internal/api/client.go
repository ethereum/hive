package api

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ethereum/hive/cmd/hq/internal/cache"
)

// Client queries a hive result server, caching responses locally.
type Client struct {
	BaseURL    string
	Suite      string
	Cache      *cache.Cache
	HTTPClient *http.Client
}

// NewClient returns a Client that talks to baseURL under the given suite and
// caches responses via c.
func NewClient(baseURL, suite string, c *cache.Cache) *Client {
	return &Client{
		BaseURL: baseURL,
		Suite:   suite,
		Cache:   c,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// fetch retrieves a URL with optional caching. ttl <= 0 means cache forever (immutable).
func (c *Client) fetch(url string, ttl time.Duration) ([]byte, error) {
	if data, ok := c.Cache.Get(url, ttl); ok {
		return data, nil
	}

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: status %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", url, err)
	}

	_ = c.Cache.Put(url, data)
	return data, nil
}

// fetchRange retrieves a byte range from a URL. Uses HTTP Range requests.
// Falls back to full fetch + slice if server returns 200 instead of 206.
func (c *Client) fetchRange(url string, begin, end int64) ([]byte, error) {
	cacheKey := fmt.Sprintf("%s#%d-%d", url, begin, end)
	if data, ok := c.Cache.Get(cacheKey, 0); ok {
		return data, nil
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", begin, end-1))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching range %s: %w", url, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading range %s: %w", url, err)
	}

	switch resp.StatusCode {
	case http.StatusPartialContent:
		// Got the exact range
	case http.StatusOK:
		// Server didn't support Range, got the full file, slice it
		if end <= int64(len(data)) {
			data = data[begin:end]
		}
	default:
		return nil, fmt.Errorf("fetching range %s: status %d", url, resp.StatusCode)
	}

	_ = c.Cache.Put(cacheKey, data)
	return data, nil
}
