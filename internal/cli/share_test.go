package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/powerumc/dclaude-config/internal/profile"
)

const (
	shareQuestion      = "Share ~/.claude with this profile? [Y/n]"
	historyQuestion    = "Share the conversation history too? [Y/n]"
	ownHistoryQuestion = "Move it aside and use the history in ~/.claude? [y/N]"
)

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

// ownHistory gives the profile a conversation of its own, replacing any link.
func (h *harness) ownHistory(p *profile.Profile) string {
	h.t.Helper()
	dir := filepath.Join(p.Dir(), "projects")
	if err := os.RemoveAll(dir); err != nil {
		h.t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "x"), 0o700); err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x", "s.jsonl"), []byte("mine"), 0o600); err != nil {
		h.t.Fatal(err)
	}
	return dir
}

func isLink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

func TestInitSharesGlobalSetupAndHistoryByDefault(t *testing.T) {
	h := newHarness(t, true)
	g := h.global()
	h.chdir("work/a")

	h.mustRun("init") // no answers pick the defaults

	out := h.stdout.String()
	for _, want := range []string{shareQuestion, "anthropics/claude-code#98044", historyQuestion, "linked 4 entries of ~/.claude into this profile"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in stdout:\n%s", want, out)
		}
	}
	p := h.profile()
	if p.Marker.Sharing != profile.SharedHistory {
		t.Errorf("sharing = %q", p.Marker.Sharing)
	}
	for _, n := range []string{"settings.json", "projects"} {
		if target, _ := os.Readlink(filepath.Join(p.Dir(), n)); target != filepath.Join(g, n) {
			t.Errorf("%s -> %q", n, target)
		}
	}
	if isLink(filepath.Join(p.Dir(), ".credentials.json")) {
		t.Error("the login must stay with the profile")
	}
}

// A profile linked from the start never writes a history of its own, which a
// later link would have to move aside.
func TestInitStartsTheSharedHistoryWhenGlobalHasNone(t *testing.T) {
	h := newHarness(t, true)
	g := filepath.Join(h.home, ".claude")
	if err := os.MkdirAll(g, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g, "settings.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.chdir("work/a")

	h.mustRun("init")

	p := h.profile()
	for _, n := range []string{"projects", "history.jsonl"} {
		if target, _ := os.Readlink(filepath.Join(p.Dir(), n)); target != filepath.Join(g, n) {
			t.Errorf("%s -> %q", n, target)
		}
	}
	if info, err := os.Stat(filepath.Join(g, "projects")); err != nil || !info.IsDir() {
		t.Errorf("~/.claude/projects: %v", err)
	}
	if info, err := os.Stat(filepath.Join(g, "history.jsonl")); err != nil || info.Size() != 0 || info.Mode().Perm() != 0o600 {
		t.Errorf("~/.claude/history.jsonl: %v %v", info, err)
	}
}

func TestInitKeepsHistorySeparateWhenAnsweredNo(t *testing.T) {
	h := newHarness(t, true)
	g := h.global()
	h.answer("y\nn\n")
	h.chdir("work/a")

	h.mustRun("init")

	p := h.profile()
	if p.Marker.Sharing != profile.Shared || !strings.Contains(h.stdout.String(), historyQuestion) {
		t.Errorf("sharing = %q, stdout:\n%s", p.Marker.Sharing, h.stdout.String())
	}
	if target, _ := os.Readlink(filepath.Join(p.Dir(), "settings.json")); target != filepath.Join(g, "settings.json") {
		t.Errorf("settings.json -> %q", target)
	}
	if isLink(filepath.Join(p.Dir(), "projects")) {
		t.Error("projects must stay with the profile")
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
		{[]string{"init", "--no-share-history"}, profile.Shared, false},
	} {
		h := newHarness(t, true)
		h.global()
		h.chdir("work/a")

		h.mustRun(tc.args...)

		p := h.profile()
		if out := h.stdout.String(); strings.Contains(out, shareQuestion) || strings.Contains(out, historyQuestion) || p.Marker.Sharing != tc.want {
			t.Errorf("%v: sharing %q, stdout:\n%s", tc.args, p.Marker.Sharing, h.stdout.String())
		}
		if got := isLink(filepath.Join(p.Dir(), "projects")); got != tc.history {
			t.Errorf("%v: projects linked = %v", tc.args, got)
		}
		if got, want := isLink(filepath.Join(p.Dir(), "settings.json")), tc.want != profile.Isolated; got != want {
			t.Errorf("%v: settings.json linked = %v", tc.args, got)
		}
		if strings.Contains(h.calls(), tc.args[1]) {
			t.Errorf("%v: share flag passed to claude:\n%s", tc.args, h.calls())
		}
	}
}

func TestInitRejectsConflictingShareFlags(t *testing.T) {
	h := newHarness(t, true)
	h.chdir("work/a")
	for _, flags := range [][2]string{
		{"--isolated", "--share-history"},
		{"--isolated", "--no-share-history"},
		{"--share-history", "--no-share-history"},
	} {
		want := flags[0] + " and " + flags[1] + " cannot be used together"
		if code := h.run("init", flags[0], flags[1]); code == 0 || !strings.Contains(h.stderr.String(), want) {
			t.Errorf("%v: exit %d, stderr: %s", flags, code, h.stderr.String())
		}
	}
	if exists(filepath.Join(h.cwd, profile.DirName)) {
		t.Error("a rejected init created the profile")
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

	if p := h.profile(); p.Marker.Sharing != profile.Isolated || isLink(filepath.Join(p.Dir(), "settings.json")) || strings.Contains(h.stdout.String(), historyQuestion) {
		t.Errorf("answered n to sharing, got %q, stdout:\n%s", p.Marker.Sharing, h.stdout.String())
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
	projects := h.ownHistory(p)
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
	// A profile with a history of its own is not offered ~/.claude's: linking
	// it would move that history aside.
	if strings.Contains(out, historyQuestion) || h.profile().Marker.Sharing != profile.Shared || isLink(projects) || read(t, filepath.Join(projects, "x", "s.jsonl")) != "mine" {
		t.Errorf("login offered to share the history; stdout:\n%s", out)
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

func TestLoginOffersTheHistoryToAnUnchosenProfileWithoutOne(t *testing.T) {
	h := newHarness(t, true)
	h.chdir("work/a")
	h.mustRun("init") // before ~/.claude existed: nothing chosen
	g := h.global()

	h.mustRun("login")

	out := h.stdout.String()
	if !strings.Contains(out, shareQuestion) || !strings.Contains(out, historyQuestion) {
		t.Errorf("stdout:\n%s", out)
	}
	p := h.profile()
	if target, _ := os.Readlink(filepath.Join(p.Dir(), "projects")); p.Marker.Sharing != profile.SharedHistory || target != filepath.Join(g, "projects") {
		t.Errorf("sharing %q, projects -> %q", p.Marker.Sharing, target)
	}
}

func TestLoginAsksBeforeMovingTheProfilesOwnHistory(t *testing.T) {
	h := newHarness(t, true)
	g := h.global()
	h.chdir("work/a")
	h.mustRun("init", "--share-history")
	p := h.profile()
	projects := h.ownHistory(p) // e.g. written while ~/.claude had no projects/

	h.mustRun("login") // no answer keeps it
	if out := h.stdout.String(); !strings.Contains(out, ownHistoryQuestion) || isLink(projects) || read(t, filepath.Join(projects, "x", "s.jsonl")) != "mine" {
		t.Errorf("moved the profile's own history without asking; stdout:\n%s", out)
	}
	h.mustRun("info")
	if out := h.stdout.String(); !strings.Contains(out, "keeps its own projects (dclaude login asks to move it aside)") || strings.Contains(out, "newer entries") {
		t.Errorf("info:\n%s", out)
	}

	h.answer("y\n")
	h.mustRun("login")
	if target, _ := os.Readlink(projects); target != filepath.Join(g, "projects") {
		t.Errorf("projects -> %q", target)
	}
	backups, _ := filepath.Glob(filepath.Join(p.Dir(), profile.BackupDirName, "*", "projects", "x", "s.jsonl"))
	if len(backups) != 1 || read(t, backups[0]) != "mine" {
		t.Errorf("backups = %q", backups)
	}
}

func TestLoginShareHistoryFlagAsksBeforeMovingAnOldProfilesHistory(t *testing.T) {
	h := newHarness(t, true)
	h.chdir("work/a")
	h.mustRun("init") // before ~/.claude existed: nothing chosen
	projects := h.ownHistory(h.profile())
	h.global()

	h.mustRun("login", "--share-history")

	if out := h.stdout.String(); !strings.Contains(out, ownHistoryQuestion) || isLink(projects) || read(t, filepath.Join(projects, "x", "s.jsonl")) != "mine" {
		t.Errorf("moved the profile's own history without asking; stdout:\n%s", out)
	}
}

func TestLoginShareFlags(t *testing.T) {
	h := newHarness(t, true)
	h.global()
	h.chdir("work/a")
	h.mustRun("init") // shares the history by default
	if code := h.run("login", "--no-share-history"); code == 0 || !strings.Contains(h.stderr.String(), `already chose "shared-history"`) {
		t.Errorf("exit %d, stderr: %s", code, h.stderr.String())
	}

	h = newHarness(t, true)
	h.chdir("work/a")
	h.mustRun("init") // before ~/.claude existed: nothing chosen
	h.global()
	h.mustRun("login", "--no-share-history")
	p := h.profile()
	if out := h.stdout.String(); strings.Contains(out, shareQuestion) || p.Marker.Sharing != profile.Shared || !isLink(filepath.Join(p.Dir(), "settings.json")) || isLink(filepath.Join(p.Dir(), "projects")) {
		t.Errorf("sharing %q, stdout:\n%s", p.Marker.Sharing, out)
	}
	if strings.Contains(h.calls(), "--no-share-history") {
		t.Errorf("share flag passed to claude:\n%s", h.calls())
	}
}
