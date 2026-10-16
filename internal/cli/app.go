// Package cli implements the dclaude commands on top of the profile, envrc,
// claude, direnv, keychain and usage packages.
package cli

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/powerumc/dclaude-config/internal/claude"
	"github.com/powerumc/dclaude-config/internal/direnv"
	"github.com/powerumc/dclaude-config/internal/keychain"
	"github.com/powerumc/dclaude-config/internal/profile"
	"github.com/powerumc/dclaude-config/internal/usage"
)

// App carries every side effect, so tests can swap them out.
type App struct {
	Stdin    io.Reader
	Stdout   io.Writer
	Stderr   io.Writer
	Env      []string
	Home     string
	Getwd    func() (string, error)
	LookPath func(string) (string, error)
	Keychain keychain.Store
	Usage    *usage.Client
	Now      func() time.Time
	Version  string
}

// New returns an App wired to the real process environment.
func New(version string) *App {
	home, _ := os.UserHomeDir()
	return &App{
		Stdin:    os.Stdin,
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
		Env:      os.Environ(),
		Home:     home,
		Getwd:    os.Getwd,
		LookPath: exec.LookPath,
		Keychain: keychain.Default(),
		Usage: &usage.Client{
			HTTP:      &http.Client{Timeout: 10 * time.Second},
			URL:       usage.DefaultURL,
			UserAgent: "dclaude/" + version,
		},
		Now:     time.Now,
		Version: version,
	}
}

const usageText = `dclaude - per-directory Claude Code login (via direnv)

Usage:
  dclaude init [share option] [login options]
                                  create a .claude_config profile and .envrc here, then log in
  dclaude login [login options]   log in again (re-points the profile if the directory moved)
  dclaude ln [dir]                use the profile of dir here, login included; in a git
                                  worktree, dir defaults to the same place in the main checkout
  dclaude deinit [-y] [--force]   log out and remove the profile, its Keychain items and the .envrc block
                                  (in a linked directory: remove only the link)
  dclaude info                    show the account, usage and activation state for this directory

Share options: --isolated (share nothing from ~/.claude), --share-history (share the
conversation history too). Without one, init asks whether to share ~/.claude.
Login options are passed to "claude auth login": --console, --sso, --email <address>

For apps that start claude without a shell, use the separate dclaude-code command in
place of claude: it runs claude with the direnv environment of the current directory.
`

// Run executes one dclaude command and returns the process exit code.
func (a *App) Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(a.Stderr, usageText)
		return 2
	}
	var err error
	switch args[0] {
	case "init":
		err = a.cmdInit(args[1:])
	case "login":
		err = a.cmdLogin(args[1:])
	case "ln":
		err = a.cmdLn(args[1:])
	case "deinit":
		err = a.cmdDeinit(args[1:])
	case "info":
		err = a.cmdInfo(args[1:])
	case "version", "--version":
		fmt.Fprintln(a.Stdout, a.Version)
	case "help", "-h", "--help":
		fmt.Fprint(a.Stdout, usageText)
	default:
		fmt.Fprintf(a.Stderr, "unknown command: %s\n\n%s", args[0], usageText)
		return 2
	}
	if err != nil {
		fmt.Fprintln(a.Stderr, a.errs().bad("dclaude:")+" "+err.Error())
		return 1
	}
	return 0
}

func (a *App) claudeCLI() *claude.CLI {
	return &claude.CLI{LookPath: a.LookPath, Env: a.Env, Stdin: a.Stdin, Stdout: a.Stdout, Stderr: a.Stderr}
}

func (a *App) direnvCLI() *direnv.Client {
	return &direnv.Client{LookPath: a.LookPath, Env: a.Env, Stdout: a.Stdout, Stderr: a.Stderr}
}

func (a *App) getenv(key string) string {
	for _, kv := range a.Env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == key {
			return v
		}
	}
	return ""
}

func (a *App) warnf(format string, args ...any) {
	fmt.Fprintf(a.Stderr, a.errs().warn("warning:")+" "+format+"\n", args...)
}

// confirm asks a yes/no question; anything but y/yes is a no.
func (a *App) confirm(question string) bool { return a.ask(question, false) }

// ask asks a yes/no question; an empty answer, or none, picks def.
func (a *App) ask(question string, def bool) bool {
	hint := "[y/N]"
	if def {
		hint = "[Y/n]"
	}
	fmt.Fprintf(a.Stdout, "%s %s ", a.out().bold(question), hint)
	switch strings.ToLower(strings.TrimSpace(a.readLine())) {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	case "":
		return def
	}
	return false
}

// readLine reads one line a byte at a time, so a later question still finds
// the answers typed ahead for it.
func (a *App) readLine() string {
	var line []byte
	b := make([]byte, 1)
	for {
		n, err := a.Stdin.Read(b)
		if n == 1 {
			if b[0] == '\n' {
				break
			}
			line = append(line, b[0])
		}
		if err != nil {
			break
		}
	}
	return string(line)
}

// currentProfile finds the profile covering the cwd, or profile.ErrNotFound.
func (a *App) currentProfile() (*profile.Profile, string, error) {
	cwd, err := a.Getwd()
	if err != nil {
		return nil, "", err
	}
	p, err := profile.Find(cwd)
	return p, cwd, err
}

// deleteKeychainItems removes both items Claude Code may have written for configDir.
func (a *App) deleteKeychainItems(configDir string) []string {
	account := keychain.Account(a.getenv)
	var deleted []string
	for _, svc := range []string{keychain.OAuthService(configDir), keychain.APIKeyService(configDir)} {
		ok, err := a.Keychain.Delete(svc, account)
		if err != nil {
			a.warnf("could not delete Keychain item %s: %v", svc, err)
			continue
		}
		if ok {
			deleted = append(deleted, svc)
		}
	}
	return deleted
}

func describeAccount(st *claude.Status, s style) string {
	var parts []string
	if st.Email != "" {
		parts = append(parts, s.bold(st.Email))
	}
	for _, v := range []string{st.OrgName, st.SubscriptionType} {
		if v != "" {
			parts = append(parts, v)
		}
	}
	desc := strings.Join(parts, ", ")
	if st.AuthMethod != "" {
		desc += " (" + st.AuthMethod + ")"
	}
	return desc
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

var errNoProfile = errors.New("no dclaude profile here; create one with: dclaude init")
