package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const zshHook = `eval "$(direnv hook zsh)"`

func (h *harness) hookShell() {
	h.t.Helper()
	if err := os.WriteFile(filepath.Join(h.home, ".zshrc"), []byte(zshHook+"\n"), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

// answer feeds the terminal input through a real file, as a terminal does: a
// child process that does not read it leaves it for dclaude's own prompt.
func (h *harness) answer(input string) {
	h.t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		h.t.Fatal(err)
	}
	w.WriteString(input)
	w.Close()
	h.t.Cleanup(func() { r.Close() })
	h.app.Stdin = r
}

func TestInitOffersToHookDirenvIntoTheShell(t *testing.T) {
	h := newHarness(t, true)
	h.setEnv("SHELL=/bin/zsh")
	h.answer("y\n")
	h.chdir("work/a")

	h.mustRun("init")

	out := h.stdout.String()
	for _, want := range []string{
		"direnv is not hooked into zsh yet",
		`  echo '` + zshHook + `' >> ~/.zshrc`,
		"  " + zshHook,
		"Add the hook to ~/.zshrc now? [y/N]",
		"added the direnv hook to ~/.zshrc; new terminals load it. To load it in this terminal now, run:\n  " + zshHook + "\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if got := read(t, filepath.Join(h.home, ".zshrc")); !strings.Contains(got, zshHook) {
		t.Errorf(".zshrc = %q", got)
	}
}

func TestInitLeavesShellAloneWhenHookDeclined(t *testing.T) {
	h := newHarness(t, true)
	h.setEnv("SHELL=/bin/zsh")
	h.chdir("work/a")

	h.mustRun("init") // empty stdin answers no

	if exists(filepath.Join(h.home, ".zshrc")) {
		t.Error(".zshrc written without consent")
	}
	if out := h.stdout.String(); !strings.Contains(out, "  "+zshHook) || strings.Contains(out, "added the direnv hook") {
		t.Errorf("stdout:\n%s", out)
	}
}

func TestInitWithHookedShellOnlySaysNextPrompt(t *testing.T) {
	h := newHarness(t, true)
	h.setEnv("SHELL=/bin/zsh")
	h.hookShell()
	h.chdir("work/a")

	h.mustRun("init")

	out := h.stdout.String()
	if !strings.Contains(out, "direnv will apply it to this shell at the next prompt") || strings.Contains(out, "Add the hook") {
		t.Errorf("stdout:\n%s", out)
	}
}

func TestLoginTellsToApproveTheEnvrc(t *testing.T) {
	h := newHarness(t, true)
	h.setEnv("SHELL=/bin/zsh")
	h.hookShell()
	cwd := h.chdir("work/a")
	h.mustRun("init")
	h.setEnv("FAKE_DIRENV_ALLOWED=1") // e.g. the user edited .envrc since

	h.chdir("work/a/src")
	h.mustRun("login")

	out := h.stdout.String()
	if !strings.Contains(out, "direnv allow "+cwd) || strings.Contains(out, "will apply it") {
		t.Errorf("stdout:\n%s", out)
	}
}

func TestInfoShowsHookCommandsWhenShellIsNotHooked(t *testing.T) {
	h := newHarness(t, true)
	h.setEnv("SHELL=/usr/bin/fish")
	h.chdir("work/a")
	h.mustRun("init")

	h.mustRun("info")

	out := h.stdout.String()
	for _, want := range []string{
		"shell     not active: direnv is not hooked into fish",
		"          echo 'direnv hook fish | source' >> ~/.config/fish/config.fish",
		"          direnv hook fish | source",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestInfoTellsToLoadTheHookIntoAShellStartedBeforeIt(t *testing.T) {
	h := newHarness(t, true)
	h.setEnv("SHELL=/bin/zsh")
	h.hookShell() // in ~/.zshrc, but this shell has no DIRENV_DIR
	h.chdir("work/a")
	h.mustRun("init")

	h.mustRun("info")

	out := h.stdout.String()
	for _, want := range []string{
		"direnv    allowed",
		"shell     not active: this shell has not loaded direnv's hook; open a new terminal, or load it here:",
		"          " + zshHook,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestInfoWaitsForApprovalWhenHookIsInPlace(t *testing.T) {
	h := newHarness(t, true)
	h.setEnv("SHELL=/bin/zsh")
	h.hookShell()
	h.chdir("work/a")
	h.mustRun("init")
	h.setEnv("FAKE_DIRENV_ALLOWED=1")

	h.mustRun("info")

	out := h.stdout.String()
	if !strings.Contains(out, "direnv    not allowed yet; run: direnv allow") || !strings.Contains(out, "shell     not active yet (direnv applies it at the next prompt once the .envrc is allowed)") {
		t.Errorf("stdout:\n%s", out)
	}
}
