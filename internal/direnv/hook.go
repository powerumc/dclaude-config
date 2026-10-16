package direnv

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// HookDocsURL documents the hook for shells without a one-line setup.
const HookDocsURL = "https://direnv.net/docs/hook.html"

// InstallDocsURL documents how to install direnv.
const InstallDocsURL = "https://direnv.net/docs/installation.html"

// Hook is how direnv hooks into one shell: a line in its startup file.
type Hook struct {
	Shell  string // shell name, such as "zsh"
	RCFile string // startup file the line goes into
	Line   string
	home   string
	checks []string // startup files that may already load direnv
}

// HookFor returns the hook for the user's login shell ($SHELL). ok is false
// for shells without a one-line setup; HookDocsURL covers those.
func HookFor(getenv func(string) string, home string) (h *Hook, ok bool) {
	h = &Hook{Shell: filepath.Base(getenv("SHELL")), home: home}
	in := func(dir string, names ...string) []string {
		paths := make([]string, len(names))
		for i, n := range names {
			paths[i] = filepath.Join(dir, n)
		}
		return paths
	}
	switch h.Shell {
	case "zsh":
		dir := home
		if z := getenv("ZDOTDIR"); z != "" {
			dir = z
		}
		h.Line, h.checks = `eval "$(direnv hook zsh)"`, in(dir, ".zshrc", ".zprofile", ".zshenv")
	case "bash":
		h.Line, h.checks = `eval "$(direnv hook bash)"`, in(home, ".bashrc", ".bash_profile", ".profile")
	case "fish":
		dir := filepath.Join(home, ".config")
		if x := getenv("XDG_CONFIG_HOME"); x != "" {
			dir = x
		}
		h.Line, h.checks = "direnv hook fish | source", in(filepath.Join(dir, "fish"), "config.fish")
		if extra, err := filepath.Glob(filepath.Join(dir, "fish", "conf.d", "*.fish")); err == nil {
			h.checks = append(h.checks, extra...)
		}
	case "tcsh":
		h.Line, h.checks = "eval `direnv hook tcsh`", in(home, ".cshrc", ".tcshrc")
	default:
		return nil, false
	}
	h.RCFile = h.checks[0]
	return h, true
}

// Installed reports whether a startup file of the shell mentions direnv,
// which also covers plugin managers such as oh-my-zsh's direnv plugin.
func (h *Hook) Installed() bool {
	for _, path := range h.checks {
		if data, err := os.ReadFile(path); err == nil && strings.Contains(string(data), "direnv") {
			return true
		}
	}
	return false
}

// RCDisplay is RCFile as a user would type it, with ~ for the home directory.
func (h *Hook) RCDisplay() string {
	if rel, err := filepath.Rel(h.home, h.RCFile); err == nil && !strings.HasPrefix(rel, "..") && safeWord.MatchString(rel) {
		return "~/" + rel
	}
	return Quote(h.RCFile)
}

// Commands are the shell commands that add the hook to the startup file, for
// new terminals, and load it into the running shell, without restarting it.
func (h *Hook) Commands() []string {
	return []string{"echo '" + h.Line + "' >> " + h.RCDisplay(), h.Line}
}

// Install appends the hook line to RCFile, creating the file if needed.
func (h *Hook) Install() error {
	if err := os.MkdirAll(filepath.Dir(h.RCFile), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(h.RCFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	text := "# direnv hook, added by dclaude\n" + h.Line + "\n"
	if len(data) > 0 {
		text = "\n" + text
		if data[len(data)-1] != '\n' {
			text = "\n" + text
		}
	}
	f, err := os.OpenFile(h.RCFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(text); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

var safeWord = regexp.MustCompile(`^[A-Za-z0-9._/@%+=:,-]+$`)

// Quote returns s ready to paste into a POSIX shell.
func Quote(s string) string {
	if safeWord.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
