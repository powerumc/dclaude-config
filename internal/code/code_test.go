package code

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeDirenv's status reports $PWD/.envrc with the allowed state in
// FAKE_ALLOWED, no .envrc with FAKE_NO_RC, and fails with FAKE_STATUS_FAILS.
const fakeDirenv = `#!/bin/sh
if [ "$1" = "status" ]; then
  [ -n "$FAKE_STATUS_FAILS" ] && exit 1
  if [ -n "$FAKE_NO_RC" ]; then
    echo '{"state":{"foundRC":null}}'
  else
    printf '{"state":{"foundRC":{"path":"%s/.envrc","allowed":%s}}}\n' "$PWD" "${FAKE_ALLOWED:-0}"
  fi
fi
`

type harness struct {
	t      *testing.T
	r      *Runner
	bin    string
	cwd    string
	stderr bytes.Buffer
	argv   []string
	env    []string
}

func newHarness(t *testing.T, withDirenv bool) *harness {
	t.Helper()
	tmp, err := filepath.EvalSymlinks(t.TempDir()) // macOS: /var -> /private/var
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, bin: filepath.Join(tmp, "bin"), cwd: filepath.Join(tmp, "work")}
	for _, d := range []string{h.bin, h.cwd} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeExec(t, filepath.Join(h.bin, "claude"), "#!/bin/sh\n")
	if withDirenv {
		writeExec(t, filepath.Join(h.bin, "direnv"), fakeDirenv)
	}
	h.r = &Runner{
		Stderr: &h.stderr,
		Env:    []string{"PATH=" + h.bin + ":/usr/bin:/bin", "HOME=" + tmp, "ANTHROPIC_API_KEY=sk-test"},
		Getwd:  func() (string, error) { return h.cwd, nil },
		LookPath: func(name string) (string, error) {
			p := filepath.Join(h.bin, name)
			if _, err := os.Stat(p); err != nil {
				return "", exec.ErrNotFound
			}
			return p, nil
		},
		Exec: func(argv0 string, argv, env []string) error {
			if argv0 != argv[0] {
				t.Errorf("argv0 %q differs from argv[0] %q", argv0, argv[0])
			}
			h.argv, h.env = argv, env
			return nil
		},
	}
	return h
}

func writeExec(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func (h *harness) run(args ...string) int {
	h.stderr.Reset()
	return h.r.Run(args)
}

// The arguments an agent app passes, plus words that are dclaude commands: all go to claude.
var claudeArgs = []string{"--version", "init", "--output-format", "stream-json", "-h"}

func TestRunsClaudeInTheDirectorysDirenvEnvironment(t *testing.T) {
	h := newHarness(t, true)

	if code := h.run(claudeArgs...); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, h.stderr.String())
	}

	want := append([]string{filepath.Join(h.bin, "direnv"), "exec", h.cwd, filepath.Join(h.bin, "claude")}, claudeArgs...)
	if !slices.Equal(h.argv, want) {
		t.Errorf("argv = %q\nwant   %q", h.argv, want)
	}
	if !slices.Contains(h.env, "DIRENV_LOG_FORMAT=") {
		t.Errorf("direnv's log is not silenced: %q", h.env)
	}
	if h.stderr.Len() > 0 {
		t.Errorf("stderr: %q", h.stderr.String())
	}
}

func TestRunsClaudeAsIsWithoutAnApprovedEnvrc(t *testing.T) {
	for _, tc := range []struct {
		name       string
		withDirenv bool
		env        string
		warns      bool
	}{
		{"direnv missing", false, "", false},
		{"no .envrc", true, "FAKE_NO_RC=1", false},
		{".envrc not allowed", true, "FAKE_ALLOWED=1", false},
		{".envrc denied", true, "FAKE_ALLOWED=2", false},
		{"direnv status fails", true, "FAKE_STATUS_FAILS=1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.withDirenv)
			if tc.env != "" {
				h.r.Env = append(h.r.Env, tc.env)
			}
			before := slices.Clone(h.r.Env)

			if code := h.run(claudeArgs...); code != 0 {
				t.Fatalf("exit %d, stderr: %s", code, h.stderr.String())
			}

			want := append([]string{filepath.Join(h.bin, "claude")}, claudeArgs...)
			if !slices.Equal(h.argv, want) {
				t.Errorf("argv = %q\nwant   %q", h.argv, want)
			}
			if !slices.Equal(h.env, before) {
				t.Errorf("env changed:\n%q\nwant\n%q", h.env, before)
			}
			if warned := strings.Contains(h.stderr.String(), "could not check direnv"); warned != tc.warns {
				t.Errorf("stderr: %q", h.stderr.String())
			}
		})
	}
}

func TestFailures(t *testing.T) {
	h := newHarness(t, true)
	h.r.Exec = func(string, []string, []string) error { return errors.New("exec format error") }
	if code := h.run(); code != 126 || !strings.Contains(h.stderr.String(), "exec format error") {
		t.Errorf("exit %d, stderr: %s", code, h.stderr.String())
	}

	h.r.LookPath = func(string) (string, error) { return "", errors.New("not on PATH") }
	if code := h.run(); code != 127 || !strings.Contains(h.stderr.String(), "dclaude-code: claude not found") {
		t.Errorf("exit %d, stderr: %s", code, h.stderr.String())
	}
}
