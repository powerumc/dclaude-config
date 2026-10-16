package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/powerumc/dclaude-config/internal/keychain"
	"github.com/powerumc/dclaude-config/internal/profile"
)

func TestDeinitRemovesProfileKeychainAndEnvrc(t *testing.T) {
	h := newHarness(t, true)
	cwd := h.chdir("work/a")
	cfg := filepath.Join(cwd, profile.DirName)
	h.mustRun("init")
	h.kc.items[keychain.OAuthService(cfg)+"|tester"] = "{}"
	h.kc.items[keychain.APIKeyService(cfg)+"|tester"] = "key"
	h.kc.items["Claude Code-device-keys|tester"] = "shared" // machine-wide, must survive

	h.mustRun("deinit", "--yes")

	if exists(cfg) || exists(filepath.Join(cwd, ".envrc")) {
		t.Error("profile or .envrc still present")
	}
	if got := strings.Join(h.kc.deleted, ","); got != keychain.OAuthService(cfg)+","+keychain.APIKeyService(cfg) {
		t.Errorf("deleted = %s", got)
	}
	if _, ok := h.kc.items["Claude Code-device-keys|tester"]; !ok {
		t.Error("shared device-key item was deleted")
	}
	if !strings.Contains(h.calls(), "claude auth logout | CLAUDE_CONFIG_DIR="+cfg) {
		t.Errorf("logout not run for the profile:\n%s", h.calls())
	}
}

func TestDeinitKeepsForeignEnvrcContent(t *testing.T) {
	h := newHarness(t, true)
	cwd := h.chdir("work/a")
	os.WriteFile(filepath.Join(cwd, ".envrc"), []byte("export FOO=1\n"), 0o644)
	h.mustRun("init")
	h.mustRun("deinit", "-y")
	if got := read(t, filepath.Join(cwd, ".envrc")); got != "export FOO=1\n" {
		t.Errorf(".envrc = %q", got)
	}
}

func TestDeinitAsksAndRefusesWhileSessionRuns(t *testing.T) {
	h := newHarness(t, true)
	cfg := filepath.Join(h.chdir("work/a"), profile.DirName)
	h.mustRun("init")

	h.app.Stdin = strings.NewReader("n\n")
	if code := h.run("deinit"); code == 0 || !exists(cfg) {
		t.Fatalf("declined deinit removed the profile (code=%d)", code)
	}

	os.MkdirAll(filepath.Join(cfg, "sessions"), 0o700)
	session := fmt.Sprintf(`{"pid":%d}`, os.Getpid())
	os.WriteFile(filepath.Join(cfg, "sessions", "1.json"), []byte(session), 0o600)
	if code := h.run("deinit", "-y"); code == 0 || !strings.Contains(h.stderr.String(), fmt.Sprint(os.Getpid())) {
		t.Fatalf("live session not detected: code=%d stderr=%s", code, h.stderr.String())
	}
	h.mustRun("deinit", "-y", "--force")
	if exists(cfg) {
		t.Error("--force did not remove the profile")
	}
}

// Real claude creates a missing CLAUDE_CONFIG_DIR, so a stale path must never reach it.
func TestMovedProfileIsNeverHandedToClaude(t *testing.T) {
	h := newHarness(t, true)
	oldRoot := h.chdir("work/a")
	oldCfg := filepath.Join(oldRoot, profile.DirName)
	h.mustRun("init")
	h.kc.items[keychain.OAuthService(oldCfg)+"|tester"] = "{}"
	newRoot := filepath.Join(h.tmp, "work/b")
	if err := os.Rename(oldRoot, newRoot); err != nil {
		t.Fatal(err)
	}
	os.Remove(h.log) // only calls after the move matter
	h.cwd = newRoot

	h.mustRun("info")
	if !strings.Contains(h.stdout.String(), "the directory moved") {
		t.Errorf("info stdout:\n%s", h.stdout.String())
	}
	h.mustRun("deinit", "-y")

	if strings.Contains(h.calls(), "CLAUDE_CONFIG_DIR="+oldCfg) {
		t.Errorf("claude ran with the stale path:\n%s", h.calls())
	}
	if exists(filepath.Join(newRoot, profile.DirName)) || exists(filepath.Join(newRoot, ".envrc")) {
		t.Error("moved profile not removed")
	}
	if len(h.kc.deleted) != 1 || h.kc.deleted[0] != keychain.OAuthService(oldCfg) {
		t.Errorf("orphaned Keychain item not cleaned: %v", h.kc.deleted)
	}
}
