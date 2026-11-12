package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// FetchListing streams the listing.jsonl file and applies filters.
// It stops early once limit results are collected.
func (c *Client) FetchListing(sim, client string, limit int) ([]ListingEntry, error) {
	url := fmt.Sprintf("%s/%s/listing.jsonl", c.BaseURL, c.Suite)
	data, err := c.fetch(url, volatileTTL)
	if err != nil {
		return nil, err
	}

	var results []ListingEntry
	scanner := bufio.NewScanner(bytes.NewReader(data))
	// Some listing lines embed full error blobs; bump the scan buffer past the
	// default 64 KiB to avoid bufio.ErrTooLong.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry ListingEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // skip malformed lines
		}
		if sim != "" && !strings.Contains(strings.ToLower(entry.Name), strings.ToLower(sim)) {
			continue
		}
		if client != "" && !ContainsClient(entry.Clients, client) {
			continue
		}
		results = append(results, entry)
		if limit > 0 && len(results) >= limit {
			break
		}
	}

	return results, scanner.Err()
}

// ContainsClient reports whether any element of clients contains target as a
// case-insensitive substring.
func ContainsClient(clients []string, target string) bool {
	target = strings.ToLower(target)
	for _, cl := range clients {
		if strings.Contains(strings.ToLower(cl), target) {
			return true
		}
	}
	return false
}

// SortByTime sorts entries newest-first by Start time.
func SortByTime(entries []ListingEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Start.After(entries[j].Start)
	})
}

// FormatTime returns a human-friendly relative time string.
func FormatTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}

// FormatDuration returns the run time between start and end in at most two
// units, e.g. "42s", "17m 5s", "4h 12m" or "2d 3h". It returns "-" when the
// end time is unknown, as in listings written before hiveview recorded it.
func FormatDuration(start, end time.Time) string {
	if end.IsZero() || end.Before(start) {
		return "-"
	}
	d := end.Sub(start).Round(time.Second)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dd %dh", int(d.Hours())/24, int(d.Hours())%24)
	}
}
