// Package usage reads subscription quota from the endpoint Claude Code itself
// uses. It is not a public API and may change, so callers degrade gracefully.
package usage

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"
)

// DefaultURL is the usage endpoint Claude Code calls.
const DefaultURL = "https://api.anthropic.com/api/oauth/usage"

// Window is one quota window, such as five_hour or seven_day.
type Window struct {
	Utilization *float64 `json:"utilization"` // percent
	ResetsAt    string   `json:"resets_at"`
}

// Client fetches usage for an OAuth access token.
type Client struct {
	HTTP      *http.Client
	URL       string
	UserAgent string
}

// Fetch returns every window that reports a utilization; other keys are ignored.
func (c *Client) Fetch(accessToken string) (map[string]Window, error) {
	req, err := http.NewRequest(http.MethodGet, c.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	windows := map[string]Window{}
	for key, value := range raw {
		var w Window
		if json.Unmarshal(value, &w) == nil && w.Utilization != nil {
			windows[key] = w
		}
	}
	return windows, nil
}

var labels = map[string]string{
	"five_hour":        "5h",
	"seven_day":        "7d",
	"seven_day_opus":   "7d Opus",
	"seven_day_sonnet": "7d Sonnet",
}

// Lines formats windows for display: 5h and 7d first, then the rest by name.
// paint, when not nil, decorates each percentage, such as "23%".
func Lines(windows map[string]Window, now time.Time, paint func(utilization float64, percent string) string) []string {
	rank := func(k string) int {
		switch k {
		case "five_hour":
			return 0
		case "seven_day":
			return 1
		}
		return 2
	}
	keys := make([]string, 0, len(windows))
	for k := range windows {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if rank(keys[i]) != rank(keys[j]) {
			return rank(keys[i]) < rank(keys[j])
		}
		return keys[i] < keys[j]
	})
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		label := labels[k]
		if label == "" {
			label = k
		}
		u := *windows[k].Utilization
		percent := fmt.Sprintf("%.0f%%", u)
		if paint != nil {
			percent = paint(u, percent)
		}
		line := label + " " + percent
		if reset, err := time.Parse(time.RFC3339, windows[k].ResetsAt); err == nil {
			line += fmt.Sprintf(" (resets %s, in %s)", reset.Local().Format("1/2 15:04"), humanDuration(reset.Sub(now)))
		}
		lines = append(lines, line)
	}
	return lines
}

func humanDuration(d time.Duration) string {
	if d <= 0 {
		return "0m"
	}
	d = d.Round(time.Minute)
	days, hours, mins := int(d/(24*time.Hour)), int(d%(24*time.Hour)/time.Hour), int(d%time.Hour/time.Minute)
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}
