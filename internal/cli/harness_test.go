package cli

import (
	"bytes"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/powerumc/dclaude-config/internal/usage"
)

// fakeClaude records every call and keeps a per-profile "logged in" flag file.
const fakeClaude = `#!/bin/sh
echo "claude $* | CLAUDE_CONFIG_DIR=${CLAUDE_CONFIG_DIR-<unset>} | ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY-<unset>}" >> "$FAKE_LOG"
case "$1 $2" in
"auth login") touch "$CLAUDE_CONFIG_DIR/.fake-logged-in" ;;
"auth logout") rm -f "$CLAUDE_CONFIG_DIR/.fake-logged-in" ;;
"auth status")
  if [ -n "$CLAUDE_CONFIG_DIR" ] && [ -f "$CLAUDE_CONFIG_DIR/.fake-logged-in" ]; then
    echo '{"loggedIn":true,"authMethod":"claude.ai","email":"dev@example.com","orgName":"Example","subscriptionType":"max"}'
  elif [ -z "$CLAUDE_CONFIG_DIR" ] && [ -n "$FAKE_GLOBAL_LOGGED_IN" ]; then
    echo '{"loggedIn":true,"authMethod":"claude.ai","email":"global@example.com","subscriptionType":"pro"}'
  else
    echo '{"loggedIn":false,"authMethod":"none"}'
  fi ;;
esac
`

// fakeDirenv records calls; status reports FAKE_FOUND_RC (default $PWD/.envrc) as allowed.
const fakeDirenv = `#!/bin/sh
echo "direnv $*" >> "$FAKE_LOG"
if [ "$1" = "status" ]; then
  printf '{"state":{"foundRC":{"path":"%s","allowed":%s}}}\n' "${FAKE_FOUND_RC:-$PWD/.envrc}" "${FAKE_DIRENV_ALLOWED:-0}"
fi
`

type fakeKeychain struct {
	items   map[string]string // "service|account" -> secret
	deleted []string
}

func (k *fakeKeychain) Read(service, account string) (string, bool, error) {
	v, ok := k.items[service+"|"+account]
	return v, ok, nil
}

func (k *fakeKeychain) Delete(service, account string) (bool, error) {
	if _, ok := k.items[service+"|"+account]; !ok {
		return false, nil
	}
	delete(k.items, service+"|"+account)
	k.deleted = append(k.deleted, service)
	return true, nil
}

var testNow = time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)

type harness struct {
	t              *testing.T
	app            *App
	tmp, home, cwd string
	log            string
	kc             *fakeKeychain
	stdout, stderr bytes.Buffer
}

func newHarness(t *testing.T, withDirenv bool) *harness {
	t.Helper()
	tmp, err := filepath.EvalSymlinks(t.TempDir()) // macOS: /var -> /private/var
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{
		t:    t,
		tmp:  tmp,
		home: filepath.Join(tmp, "home"),
		log:  filepath.Join(tmp, "calls.log"),
		kc:   &fakeKeychain{items: map[string]string{}},
	}
	bin := filepath.Join(tmp, "bin")
	for _, d := range []string{h.home, bin} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeExec(t, filepath.Join(bin, "claude"), fakeClaude)
	if withDirenv {
		writeExec(t, filepath.Join(bin, "direnv"), fakeDirenv)
	}
	h.app = &App{
		Stdin:  strings.NewReader(""),
		Stdout: &h.stdout,
		Stderr: &h.stderr,
		Env: []string{
			"PATH=" + bin + ":/usr/bin:/bin", "USER=tester", "HOME=" + h.home,
			"ANTHROPIC_API_KEY=sk-must-not-leak", "FAKE_LOG=" + h.log,
		},
		Home:  h.home,
		Getwd: func() (string, error) { return h.cwd, nil },
		LookPath: func(name string) (string, error) {
			p := filepath.Join(bin, name)
			if _, err := os.Stat(p); err != nil {
				return "", exec.ErrNotFound
			}
			return p, nil
		},
		Keychain: h.kc,
		Usage:    &usage.Client{HTTP: http.DefaultClient, URL: "http://127.0.0.1:1/unused"},
		Now:      func() time.Time { return testNow },
		Version:  "test",
	}
	return h
}

func writeExec(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// chdir creates a directory under the temp root and makes it the cwd.
func (h *harness) chdir(rel string) string {
	h.t.Helper()
	h.cwd = filepath.Join(h.tmp, rel)
	if err := os.MkdirAll(h.cwd, 0o755); err != nil {
		h.t.Fatal(err)
	}
	return h.cwd
}

func (h *harness) run(args ...string) int {
	h.stdout.Reset()
	h.stderr.Reset()
	return h.app.Run(args)
}

func (h *harness) mustRun(args ...string) {
	h.t.Helper()
	if code := h.run(args...); code != 0 {
		h.t.Fatalf("dclaude %v exited %d\nstdout:\n%s\nstderr:\n%s", args, code, h.stdout.String(), h.stderr.String())
	}
}

func (h *harness) calls() string {
	data, _ := os.ReadFile(h.log)
	return string(data)
}

func (h *harness) setEnv(kv string) { h.app.Env = append(h.app.Env, kv) }

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
