package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/powerumc/dclaude-config/internal/envrc"
	"github.com/powerumc/dclaude-config/internal/keychain"
	"github.com/powerumc/dclaude-config/internal/profile"
)

func TestLoginWithoutProfileFails(t *testing.T) {
	h := newHarness(t, true)
	h.chdir("elsewhere")
	if code := h.run("login"); code == 0 || !strings.Contains(h.stderr.String(), "dclaude init") {
		t.Errorf("code=%d stderr=%s", code, h.stderr.String())
	}
}

func TestLoginRepointsMovedDirectory(t *testing.T) {
	h := newHarness(t, true)
	oldCfg := filepath.Join(h.chdir("work/a"), profile.DirName)
	h.mustRun("init")
	h.kc.items[keychain.OAuthService(oldCfg)+"|tester"] = "{}"

	newRoot := filepath.Join(h.tmp, "work/b")
	if err := os.Rename(filepath.Join(h.tmp, "work/a"), newRoot); err != nil {
		t.Fatal(err)
	}
	h.cwd = newRoot
	h.mustRun("login")

	newCfg := filepath.Join(newRoot, profile.DirName)
	if p, err := profile.Find(newRoot); err != nil || p.ConfigDir() != newCfg || p.Moved() {
		t.Errorf("profile not re-pointed: %+v %v", p, err)
	}
	if got, _ := envrc.ExportedConfigDir(filepath.Join(newRoot, ".envrc")); got != newCfg {
		t.Errorf(".envrc exports %q", got)
	}
	if len(h.kc.deleted) != 1 || h.kc.deleted[0] != keychain.OAuthService(oldCfg) {
		t.Errorf("old Keychain item not cleaned: %v", h.kc.deleted)
	}
	if !strings.Contains(h.calls(), "claude auth login | CLAUDE_CONFIG_DIR="+newCfg) {
		t.Errorf("login not run for the new path:\n%s", h.calls())
	}
}

func TestLoginAfterCopyKeepsOriginalLogin(t *testing.T) {
	h := newHarness(t, true)
	origCfg := filepath.Join(h.chdir("work/a"), profile.DirName)
	h.mustRun("init")
	h.kc.items[keychain.OAuthService(origCfg)+"|tester"] = "{}"

	copyRoot := h.chdir("work/c")
	os.Mkdir(filepath.Join(copyRoot, profile.DirName), 0o700)
	marker := read(t, filepath.Join(origCfg, profile.MarkerName))
	os.WriteFile(filepath.Join(copyRoot, profile.DirName, profile.MarkerName), []byte(marker), 0o600)
	h.mustRun("login")

	if len(h.kc.deleted) != 0 {
		t.Errorf("the original profile's Keychain item was deleted: %v", h.kc.deleted)
	}
	if p, _ := profile.Find(filepath.Join(h.tmp, "work/a")); p.ConfigDir() != origCfg {
		t.Errorf("original marker changed: %+v", p.Marker)
	}
}
