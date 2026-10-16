//go:build unix

package claude

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// LiveSessions returns the PIDs of running claude processes that use
// configHome. Claude Code records each session as sessions/<pid>.json there.
func LiveSessions(configHome string) []int {
	files, _ := filepath.Glob(filepath.Join(configHome, "sessions", "*.json"))
	var pids []int
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var s struct {
			PID int `json:"pid"`
		}
		if json.Unmarshal(data, &s) != nil || s.PID <= 0 {
			continue
		}
		if err := syscall.Kill(s.PID, 0); err == nil || errors.Is(err, syscall.EPERM) {
			pids = append(pids, s.PID)
		}
	}
	return pids
}
