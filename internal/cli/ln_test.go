package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/powerumc/dclaude-config/internal/envrc"
	"github.com/powerumc/dclaude-config/internal/profile"
)

// withGit lets dclaude find the real git, and returns a runner for setting
// up repositories.
func (h *harness) withGit() func(dir string, args ...string) {
	h.t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		h.t.Skip("git is not installed")
	}
	writeExec(h.t, filepath.Join(h.tmp, "bin", "git"), "#!/bin/sh\nexec "+git+" \"$@\"\n")
	return func(dir string, args ...string) {
		h.t.Helper()
		cmd := exec.Command(git, append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "init.defaultBranch=main"}, args...)...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			h.t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestLnLinksAWorktreeToTheMainCheckout(t *testing.T) {
	h := newHarness(t, true)
	git := h.withGit()
	repo := h.chdir("work/repo")
	git(repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(repo, "add", "README")
	git(repo, "commit", "-q", "-m", "init")
	h.mustRun("init")
	cfg := filepath.Join(repo, profile.DirName)
	git(repo, "worktree", "add", "-q", "../wt")
	logins := strings.Count(h.calls(), "auth login")

	wt := h.chdir("work/wt")
	h.mustRun("ln")

	out := h.stdout.String()
	if target, _ := os.Readlink(filepath.Join(wt, profile.DirName)); target != cfg {
		t.Errorf("link -> %q, want %q", target, cfg)
	}
	if got, _ := envrc.ExportedConfigDir(filepath.Join(wt, ".envrc")); got != cfg {
		t.Errorf(".envrc exports %q", got)
	}
	if !strings.Contains(out, "account: dev@example.com") || strings.Count(h.calls(), "auth login") != logins {
		t.Errorf("ln must reuse the login\nstdout:\n%s\ncalls:\n%s", out, h.calls())
	}
	if !strings.Contains(h.calls(), "direnv allow "+wt) {
		t.Errorf("worktree .envrc not approved:\n%s", h.calls())
	}
	if !strings.Contains(read(t, filepath.Join(repo, ".git", "info", "exclude")), profile.DirName) {
		t.Error(".claude_config not excluded from git")
	}
	if src := h.profileAt(repo); len(src.Marker.Links) != 1 || src.Marker.Links[0] != wt {
		t.Errorf("source links = %q", src.Marker.Links)
	}

	h.mustRun("info")
	if !strings.Contains(h.stdout.String(), "profile   "+cfg+" (linked with dclaude ln)") {
		t.Errorf("info:\n%s", h.stdout.String())
	}
}

func (h *harness) profileAt(dir string) *profile.Profile {
	h.t.Helper()
	p, err := profile.Find(dir)
	if err != nil {
		h.t.Fatal(err)
	}
	return p
}

func TestLnToANamedDirectory(t *testing.T) {
	h := newHarness(t, true)
	h.chdir("work/a")
	h.mustRun("init")
	h.chdir("work/b")

	h.mustRun("ln", "../a")

	if p := h.profileAt(h.cwd); !p.Linked() || p.ConfigDir() != filepath.Join(h.tmp, "work/a", profile.DirName) {
		t.Errorf("linked = %v, configDir = %q", p.Linked(), p.ConfigDir())
	}

	// Linking to a linked directory reaches the same profile.
	h.chdir("work/c")
	h.mustRun("ln", "../b")
	if target, _ := os.Readlink(filepath.Join(h.cwd, profile.DirName)); target != filepath.Join(h.tmp, "work/a", profile.DirName) {
		t.Errorf("c -> %q", target)
	}
}

func TestLnAgainRepairsTheEnvrc(t *testing.T) {
	h := newHarness(t, true)
	h.chdir("work/a")
	h.mustRun("init")
	b := h.chdir("work/b")
	h.mustRun("ln", "../a")
	if err := os.Remove(filepath.Join(b, ".envrc")); err != nil { // e.g. ln was cut short
		t.Fatal(err)
	}

	h.mustRun("ln", "../a")

	if got, _ := envrc.ExportedConfigDir(filepath.Join(b, ".envrc")); got != filepath.Join(h.tmp, "work/a", profile.DirName) || !strings.Contains(h.stdout.String(), "already linked") {
		t.Errorf(".envrc exports %q\nstdout:\n%s", got, h.stdout.String())
	}
}

func TestLnRefusals(t *testing.T) {
	h := newHarness(t, true)
	h.withGit()
	h.chdir("work/a")
	h.mustRun("init")

	for _, tc := range []struct {
		dir  string
		args []string
		want string
	}{
		{"work/a", []string{"ln", "../x"}, "no dclaude profile at or above"},
		{"work/b", []string{"ln"}, "not in a git worktree"},
		{"work/b", []string{"ln", "a", "b"}, "usage: dclaude ln [dir]"},
	} {
		h.chdir(tc.dir)
		if code := h.run(tc.args...); code == 0 || !strings.Contains(h.stderr.String(), tc.want) {
			t.Errorf("%s: %v: exit %d, stderr: %s", tc.dir, tc.args, code, h.stderr.String())
		}
	}

	h.chdir("work/b")
	h.mustRun("init")
	if code := h.run("ln", "../a"); code == 0 || !strings.Contains(h.stderr.String(), "already has its own profile") {
		t.Errorf("exit %d, stderr: %s", code, h.stderr.String())
	}

	h.chdir("work/a/sub")
	h.mustRun("ln", "..")
	if !strings.Contains(h.stdout.String(), "already uses the profile at") || isLink(filepath.Join(h.cwd, profile.DirName)) {
		t.Errorf("stdout:\n%s", h.stdout.String())
	}
}

func TestDeinitInALinkedDirectoryKeepsTheProfile(t *testing.T) {
	h := newHarness(t, true)
	a := h.chdir("work/a")
	h.mustRun("init")
	b := h.chdir("work/b")
	h.mustRun("ln", "../a")
	before := h.calls()

	h.mustRun("deinit", "-y")

	if exists(filepath.Join(b, profile.DirName)) || exists(filepath.Join(b, ".envrc")) {
		t.Error("link or .envrc left behind")
	}
	if !profile.IsProfileDir(filepath.Join(a, profile.DirName)) {
		t.Error("deinit through a link removed the profile")
	}
	if after := strings.TrimPrefix(h.calls(), before); strings.Contains(after, "auth logout") || len(h.kc.deleted) > 0 {
		t.Errorf("deinit through a link logged out or deleted Keychain items:\n%s", after)
	}
	if links := h.profileAt(a).Marker.Links; len(links) != 0 {
		t.Errorf("source still lists %q", links)
	}
}

func TestDeinitOfTheProfileRemovesItsLinks(t *testing.T) {
	h := newHarness(t, true)
	h.chdir("work/a")
	h.mustRun("init")
	b := h.chdir("work/b")
	h.mustRun("ln", "../a")
	h.chdir("work/a")

	h.mustRun("deinit", "-y")

	out := h.stdout.String()
	if !strings.Contains(out, "It also removes the links to this profile in: "+b) || !strings.Contains(out, "removed link: "+filepath.Join(b, profile.DirName)) {
		t.Errorf("stdout:\n%s", out)
	}
	if exists(filepath.Join(b, profile.DirName)) || exists(filepath.Join(b, ".envrc")) {
		t.Error("link or .envrc left behind")
	}
}
