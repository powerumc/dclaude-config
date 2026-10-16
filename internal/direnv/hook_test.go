package direnv

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestHookForShells(t *testing.T) {
	home := "/home/u"
	for _, tc := range []struct {
		vars      map[string]string
		shell, rc string
		line      string
	}{
		{map[string]string{"SHELL": "/bin/zsh"}, "zsh", home + "/.zshrc", `eval "$(direnv hook zsh)"`},
		{map[string]string{"SHELL": "/bin/zsh", "ZDOTDIR": "/z"}, "zsh", "/z/.zshrc", `eval "$(direnv hook zsh)"`},
		{map[string]string{"SHELL": "/opt/homebrew/bin/bash"}, "bash", home + "/.bashrc", `eval "$(direnv hook bash)"`},
		{map[string]string{"SHELL": "/usr/bin/fish"}, "fish", home + "/.config/fish/config.fish", "direnv hook fish | source"},
		{map[string]string{"SHELL": "/usr/bin/fish", "XDG_CONFIG_HOME": "/x"}, "fish", "/x/fish/config.fish", "direnv hook fish | source"},
		{map[string]string{"SHELL": "/bin/tcsh"}, "tcsh", home + "/.cshrc", "eval `direnv hook tcsh`"},
	} {
		h, ok := HookFor(env(tc.vars), home)
		if !ok {
			t.Fatalf("%v: no hook", tc.vars)
		}
		if h.Shell != tc.shell || h.RCFile != tc.rc || h.Line != tc.line {
			t.Errorf("%v: got %q %q %q", tc.vars, h.Shell, h.RCFile, h.Line)
		}
		if got := h.Commands(); got[len(got)-1] != h.Line {
			t.Errorf("%v: the last command should load the hook into this shell: %q", tc.vars, got)
		}
	}
	for _, shell := range []string{"", "/bin/sh", "/usr/bin/nu"} {
		if _, ok := HookFor(env(map[string]string{"SHELL": shell}), home); ok {
			t.Errorf("SHELL=%q: want no one-line hook", shell)
		}
	}
}

func TestHookCommandsQuoteTheStartupFile(t *testing.T) {
	h, _ := HookFor(env(map[string]string{"SHELL": "/bin/zsh"}), "/home/u")
	want := []string{`echo 'eval "$(direnv hook zsh)"' >> ~/.zshrc`, `eval "$(direnv hook zsh)"`}
	if got := h.Commands(); !reflect.DeepEqual(got, want) {
		t.Errorf("Commands = %q, want %q", got, want)
	}
	h, _ = HookFor(env(map[string]string{"SHELL": "/bin/zsh", "ZDOTDIR": "/my dots"}), "/home/u")
	if got := h.Commands()[0]; got != `echo 'eval "$(direnv hook zsh)"' >> '/my dots/.zshrc'` {
		t.Errorf("Commands[0] = %q", got)
	}
}

func TestHookInstalledLooksAtEveryStartupFile(t *testing.T) {
	home := t.TempDir()
	h, _ := HookFor(env(map[string]string{"SHELL": "/bin/bash"}), home)
	if h.Installed() {
		t.Fatal("Installed with no startup files")
	}
	os.WriteFile(filepath.Join(home, ".bashrc"), []byte("alias ll='ls -l'\n"), 0o644)
	if h.Installed() {
		t.Fatal("Installed without a direnv line")
	}
	os.WriteFile(filepath.Join(home, ".bash_profile"), []byte(`eval "$(direnv hook bash)"`+"\n"), 0o644)
	if !h.Installed() {
		t.Fatal("hook in .bash_profile not found")
	}
}

func TestHookInstallAppendsAndKeepsContent(t *testing.T) {
	home := t.TempDir()
	rc := filepath.Join(home, ".zshrc")
	os.WriteFile(rc, []byte("export A=1"), 0o600) // no trailing newline
	h, _ := HookFor(env(map[string]string{"SHELL": "/bin/zsh"}), home)
	if err := h.Install(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(rc)
	if !strings.HasPrefix(string(data), "export A=1\n") || !strings.HasSuffix(string(data), `eval "$(direnv hook zsh)"`+"\n") {
		t.Fatalf(".zshrc:\n%s", data)
	}
	if info, _ := os.Stat(rc); info.Mode().Perm() != 0o600 {
		t.Errorf("mode changed to %v", info.Mode().Perm())
	}
	if !h.Installed() {
		t.Error("Installed after Install")
	}
}

func TestHookInstallCreatesFishConfig(t *testing.T) {
	home := t.TempDir()
	h, _ := HookFor(env(map[string]string{"SHELL": "/usr/bin/fish"}), home)
	if err := h.Install(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".config", "fish", "config.fish"))
	if err != nil || !strings.Contains(string(data), "direnv hook fish | source") {
		t.Fatalf("config.fish: %q %v", data, err)
	}
}
