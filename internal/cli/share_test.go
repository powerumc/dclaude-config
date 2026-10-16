package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/powerumc/dclaude-config/internal/profile"
)

const shareQuestion = "Share ~/.claude with this profile? [Y/n]"

// global creates ~/.claude with a few entries, and returns its path.
func (h *harness) global(entries ...string) string {
	h.t.Helper()
	g := filepath.Join(h.home, ".claude")
	if err := os.MkdirAll(filepath.Join(g, "projects"), 0o755); err != nil {
		h.t.Fatal(err)
	}
	for _, e := range append([]string{"settings.json", "CLAUDE.md", ".credentials.json"}, entries...) {
		if err := os.WriteFile(filepath.Join(g, e), []byte("global "+e), 0o644); err != nil {
			h.t.Fatal(err)
		}
	}
	return g
}

func (h *harness) profile() *profile.Profile {
	h.t.Helper()
	p, err := profile.Find(h.cwd)
	if err != nil {
		h.t.Fatal(err)
	}
	return p
}

func isLink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

func TestInitSharesGlobalSetupByDefault(t *testing.T) {
	h := newHarness(t, true)
	g := h.global()
	h.chdir("work/a")

	h.mustRun("init") // no answer picks the default

	out := h.stdout.String()
	if !strings.Contains(out, shareQuestion) || !strings.Contains(out, "linked 2 entries of ~/.claude into this profile") {
		t.Errorf("stdout:\n%s", out)
	}
	p := h.profile()
	if p.Marker.Sharing != profile.Shared {
		t.Errorf("sharing = %q", p.Marker.Sharing)
	}
	if target, _ := os.Readlink(filepath.Join(p.Dir(), "settings.json")); target != filepath.Join(g, "settings.json") {
		t.Errorf("settings.json -> %q", target)
	}
	for _, n := range []string{"projects", ".credentials.json"} {
		if isLink(filepath.Join(p.Dir(), n)) {
			t.Errorf("%s must stay with the profile", n)
		}
	}
}

func TestInitShareChoicesWithoutAsking(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		want    profile.Sharing
		history bool
	}{
		{[]string{"init", "--isolated", "--sso"}, profile.Isolated, false},
		{[]string{"init", "--share-history"}, profile.SharedHistory, true},
	} {
		h := newHarness(t, true)
		h.global()
		h.chdir("work/a")

		h.mustRun(tc.args...)

		p := h.profile()
		if strings.Contains(h.stdout.String(), shareQuestion) || p.Marker.Sharing != tc.want {
			t.Errorf("%v: sharing %q, stdout:\n%s", tc.args, p.Marker.Sharing, h.stdout.String())
		}
		if got := isLink(filepath.Join(p.Dir(), "projects")); got != tc.history {
			t.Errorf("%v: projects linked = %v", tc.args, got)
		}
		if strings.Contains(h.calls(), "--isolated") || strings.Contains(h.calls(), "--share-history") {
			t.Errorf("%v: share flag passed to claude:\n%s", tc.args, h.calls())
		}
	}
}

func TestInitRejectsConflictingShareFlags(t *testing.T) {
	h := newHarness(t, true)
	h.chdir("work/a")
	if code := h.run("init", "--isolated", "--share-history"); code == 0 || !strings.Contains(h.stderr.String(), "cannot be used together") {
		t.Errorf("exit %d, stderr: %s", code, h.stderr.String())
	}
}

func TestInitAsksNothingWithoutGlobalDir(t *testing.T) {
	h := newHarness(t, true)
	h.chdir("work/a")
	h.mustRun("init")

	if strings.Contains(h.stdout.String(), shareQuestion) || h.profile().Marker.Sharing != profile.SharingUnset {
		t.Errorf("stdout:\n%s", h.stdout.String())
	}
	h.mustRun("info")
	if !strings.Contains(h.stdout.String(), "shared    nothing; ~/.claude does not exist") {
		t.Errorf("info:\n%s", h.stdout.String())
	}
}

// Answers typed ahead in one go must reach each question in turn.
func TestAnswersReachEachQuestion(t *testing.T) {
	h := newHarness(t, true)
	h.global()
	h.setEnv("SHELL=/bin/zsh")
	h.answer("n\ny\n")
	h.chdir("work/a")

	h.mustRun("init")

	if p := h.profile(); p.Marker.Sharing != profile.Isolated || isLink(filepath.Join(p.Dir(), "settings.json")) {
		t.Errorf("answered n to sharing, got %q", p.Marker.Sharing)
	}
	if got := read(t, filepath.Join(h.home, ".zshrc")); !strings.Contains(got, zshHook) {
		t.Errorf("answered y to the hook, .zshrc = %q", got)
	}
}

func TestLoginAsksOldProfilesOnceAndKeepsTheirOwnFiles(t *testing.T) {
	h := newHarness(t, true)
	h.chdir("work/a")
	h.mustRun("init") // before ~/.claude existed: nothing chosen
	p := h.profile()
	if err := os.WriteFile(filepath.Join(p.Dir(), "settings.json"), []byte("own"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := h.global()

	h.mustRun("info")
	if !strings.Contains(h.stdout.String(), "shared    not chosen yet; dclaude login asks whether to share ~/.claude") {
		t.Errorf("info:\n%s", h.stdout.String())
	}

	h.mustRun("login")
	out := h.stdout.String()
	if !strings.Contains(out, shareQuestion) || !strings.Contains(out, "moved this profile's own settings.json to "+filepath.Join(p.Dir(), profile.BackupDirName)) {
		t.Errorf("stdout:\n%s", out)
	}
	if target, _ := os.Readlink(filepath.Join(p.Dir(), "settings.json")); target != filepath.Join(g, "settings.json") {
		t.Errorf("settings.json -> %q", target)
	}
	backups, _ := filepath.Glob(filepath.Join(p.Dir(), profile.BackupDirName, "*", "settings.json"))
	if len(backups) != 1 || read(t, backups[0]) != "own" {
		t.Errorf("backups = %q", backups)
	}

	h.mustRun("login")
	if strings.Contains(h.stdout.String(), shareQuestion) {
		t.Error("asked again after choosing")
	}
}

func TestLoginLinksEntriesAddedToGlobalLater(t *testing.T) {
	h := newHarness(t, true)
	g := h.global()
	h.chdir("work/a")
	h.mustRun("init")
	if err := os.Mkdir(filepath.Join(g, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}

	h.mustRun("info")
	if !strings.Contains(h.stdout.String(), "1 newer entries not linked yet (dclaude login links them)") {
		t.Errorf("info:\n%s", h.stdout.String())
	}
	h.mustRun("login")
	if !strings.Contains(h.stdout.String(), "linked 1 entries") || !isLink(filepath.Join(h.profile().Dir(), "agents")) {
		t.Errorf("stdout:\n%s", h.stdout.String())
	}
	if code := h.run("login", "--isolated"); code == 0 || !strings.Contains(h.stderr.String(), "changing it is not supported yet") {
		t.Errorf("exit %d, stderr: %s", code, h.stderr.String())
	}
}

func TestDeinitLeavesTheSharedGlobalAlone(t *testing.T) {
	h := newHarness(t, true)
	g := h.global()
	if err := os.WriteFile(filepath.Join(g, "projects", "session.jsonl"), []byte("talk"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.chdir("work/a")
	h.mustRun("init", "--share-history")

	h.mustRun("deinit", "-y")

	if exists(filepath.Join(h.cwd, profile.DirName)) {
		t.Error("profile directory still exists")
	}
	if read(t, filepath.Join(g, "settings.json")) != "global settings.json" || read(t, filepath.Join(g, "projects", "session.jsonl")) != "talk" {
		t.Error("deinit touched ~/.claude")
	}
}
