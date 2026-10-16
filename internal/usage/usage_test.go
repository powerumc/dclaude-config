package usage

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchKeepsOnlyWindows(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" || r.Header.Get("anthropic-beta") != "oauth-2025-04-20" {
			http.Error(w, "bad headers", http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"five_hour":{"utilization":23,"resets_at":"2026-10-15T14:00:00Z"},
			"seven_day":{"utilization":41.4,"resets_at":"2026-10-18T09:00:00.123Z"},
			"seven_day_opus":null,"extra_usage":{"is_enabled":false}}`))
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), URL: srv.URL}

	windows, err := c.Fetch("tok")
	if err != nil {
		t.Fatal(err)
	}
	if len(windows) != 2 {
		t.Fatalf("windows = %v", windows)
	}
	lines := Lines(windows, time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC), nil)
	if !strings.HasPrefix(lines[0], "5h 23%") || !strings.Contains(lines[0], "in 4h 0m") {
		t.Errorf("line 0 = %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "7d 41%") || !strings.Contains(lines[1], "in 2d 23h") {
		t.Errorf("line 1 = %q", lines[1])
	}

	painted := Lines(windows, time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC), func(u float64, percent string) string {
		return fmt.Sprintf("<%s@%.0f>", percent, u)
	})
	if !strings.HasPrefix(painted[0], "5h <23%@23> (resets ") {
		t.Errorf("painted line 0 = %q", painted[0])
	}

	if _, err := c.Fetch("wrong"); err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("want HTTP 401 error, got %v", err)
	}
}

func TestHumanDuration(t *testing.T) {
	cases := map[time.Duration]string{
		-time.Minute:               "0m",
		12 * time.Minute:           "12m",
		90 * time.Minute:           "1h 30m",
		50*time.Hour + time.Minute: "2d 2h",
	}
	for d, want := range cases {
		if got := humanDuration(d); got != want {
			t.Errorf("humanDuration(%v) = %s, want %s", d, got, want)
		}
	}
}
