// Package code is the dclaude-code command: it runs claude with the
// environment direnv gives the current directory, so an app that starts claude
// without a shell, such as an agent app's chat UI, uses the directory's
// profile like a terminal there would.
package code

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/powerumc/dclaude-config/internal/direnv"
)

// Name is the command name used in messages.
const Name = "dclaude-code"

// Runner carries every side effect, so tests can swap them out.
type Runner struct {
	Stderr   io.Writer
	Env      []string
	Getwd    func() (string, error)
	LookPath func(string) (string, error)
	// Exec replaces the process with argv; it returns only on failure.
	Exec func(argv0 string, argv, env []string) error
}

// New returns a Runner wired to the real process.
func New() *Runner {
	return &Runner{
		Stderr:   os.Stderr,
		Env:      os.Environ(),
		Getwd:    os.Getwd,
		LookPath: exec.LookPath,
		Exec:     syscall.Exec,
	}
}

// Run starts claude with args, none of which it reads, and returns an exit
// code only if claude could not be started.
func (r *Runner) Run(args []string) int {
	claudeBin, err := r.LookPath("claude")
	if err != nil {
		fmt.Fprintf(r.Stderr, "%s: claude not found: %v\n", Name, err)
		return 127
	}
	argv := append([]string{claudeBin}, args...)
	env := r.Env
	if dir, direnvBin, ok := r.allowedEnvrc(); ok {
		// direnv exec loads the .envrc the way the shell hook would. Its log
		// goes to stderr, which an app reading claude's output may show.
		argv = append([]string{direnvBin, "exec", dir}, argv...)
		env = append(withoutVar(env, "DIRENV_LOG_FORMAT"), "DIRENV_LOG_FORMAT=")
	}
	if err := r.Exec(argv[0], argv, env); err != nil {
		fmt.Fprintf(r.Stderr, "%s: running %s: %v\n", Name, argv[0], err)
		return 126
	}
	return 0
}

// allowedEnvrc finds the .envrc direnv would load in the current directory.
// Only one approved with direnv allow counts: running any other would let a
// repository pick the profile, and with it the hooks, claude runs.
func (r *Runner) allowedEnvrc() (dir, direnvBin string, ok bool) {
	direnvBin, err := r.LookPath("direnv")
	if err != nil {
		return "", "", false
	}
	dir, err = r.Getwd()
	if err != nil {
		return "", "", false
	}
	client := &direnv.Client{LookPath: r.LookPath, Env: r.Env, Stdout: io.Discard, Stderr: r.Stderr}
	rc, err := client.FoundRC(dir)
	if err != nil {
		fmt.Fprintf(r.Stderr, "%s: could not check direnv, so claude runs without it: %v\n", Name, err)
		return "", "", false
	}
	if rc == nil || rc.Allowed != direnv.Allowed {
		return "", "", false
	}
	return dir, direnvBin, true
}

func withoutVar(env []string, key string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if k, _, _ := strings.Cut(kv, "="); k != key {
			out = append(out, kv)
		}
	}
	return out
}
