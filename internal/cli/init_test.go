package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/powerumc/dclaude-config/internal/claude"
	"github.com/powerumc/dclaude-config/internal/envrc"
	"github.com/powerumc/dclaude-config/internal/profile"
)

func TestInitCreatesProfileEnvrcAndLogsIn(t *testing.T) {
	h := newHarness(t, true)
	cwd := h.chdir("work/a")
	cfg := filepath.Join(cwd, profile.DirName)

	h.mustRun("init", "--console")

	p, err := profile.Find(cwd)
	if err != nil || p.ConfigDir() != cfg {
		t.Fatalf("profile = %+v, %v", p, err)
	}
	if got := read(t, filepath.Join(cwd, ".envrc")); got != envrc.Block(cfg, claude.AuthOverrideVars) {
		t.Errorf(".envrc = %q", got)
	}
	calls := h.calls()
	for _, want := range []string{
		"direnv allow " + cwd,
		"claude auth login --console | CLAUDE_CONFIG_DIR=" + cfg + " | ANTHROPIC_API_KEY=<unset>",
	} {
		if !strings.Contains(calls, want) {
			t.Errorf("missing call %q in:\n%s", want, calls)
		}
	}
	if !strings.Contains(h.stdout.String(), "logged in: dev@example.com") {
		t.Errorf("stdout:\n%s", h.stdout.String())
	}
}

func TestInitRefusesExistingProfileAndHome(t *testing.T) {
	h := newHarness(t, true)
	h.chdir("work/a")
	h.mustRun("init")
	if code := h.run("init"); code == 0 || !strings.Contains(h.stderr.String(), "dclaude login") {
		t.Errorf("second init: code=%d stderr=%s", code, h.stderr.String())
	}
	h.cwd = h.home
	if code := h.run("init"); code == 0 || exists(filepath.Join(h.home, profile.DirName)) {
		t.Errorf("init in home: code=%d", code)
	}
}

func TestInitRefusesForeignClaudeConfig(t *testing.T) {
	h := newHarness(t, true)
	cwd := h.chdir("work/a")
	os.Mkdir(filepath.Join(cwd, profile.DirName), 0o700)
	if code := h.run("init"); code == 0 || !strings.Contains(h.stderr.String(), "not created by dclaude") {
		t.Errorf("code=%d stderr=%s", code, h.stderr.String())
	}
}

func TestInitLeavesForeignEnvrcForUserToApprove(t *testing.T) {
	h := newHarness(t, true)
	cwd := h.chdir("work/a")
	os.WriteFile(filepath.Join(cwd, ".envrc"), []byte("export FOO=1\n"), 0o644)
	h.setEnv("FAKE_DIRENV_ALLOWED=1") // direnv blocks the changed file

	h.mustRun("init")

	if strings.Contains(h.calls(), "direnv allow") {
		t.Error("must not approve content dclaude did not write")
	}
	if got := read(t, filepath.Join(cwd, ".envrc")); !strings.HasPrefix(got, "export FOO=1\n") {
		t.Errorf(".envrc = %q", got)
	}
	if !strings.Contains(h.stdout.String(), "approve it yourself") {
		t.Errorf("stdout:\n%s", h.stdout.String())
	}
}

func TestInitRequiresDirenv(t *testing.T) {
	h := newHarness(t, false)
	cwd := h.chdir("work/a")
	if code := h.run("init"); code == 0 {
		t.Fatal("init succeeded without direnv")
	}
	if stderr := h.stderr.String(); !strings.Contains(stderr, "direnv is not installed") || !strings.Contains(stderr, "https://direnv.net/docs/installation.html") {
		t.Errorf("stderr:\n%s", stderr)
	}
	if exists(filepath.Join(cwd, profile.DirName)) || exists(filepath.Join(cwd, ".envrc")) || strings.Contains(h.calls(), "auth login") {
		t.Errorf("init did work before stopping:\n%s", h.calls())
	}
}

func TestLoginRequiresDirenv(t *testing.T) {
	h := newHarness(t, true)
	h.chdir("work/a")
	h.mustRun("init")
	os.Remove(filepath.Join(h.tmp, "bin", "direnv"))
	if code := h.run("login"); code == 0 || !strings.Contains(h.stderr.String(), "direnv is not installed") {
		t.Errorf("code=%d stderr:\n%s", code, h.stderr.String())
	}
}
