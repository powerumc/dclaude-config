package direnv

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const fakeDirenv = `#!/bin/sh
echo "$* PWD=$PWD" >> "$FAKE_LOG"
if [ "$1" = "status" ]; then
  printf '{"state":{"foundRC":{"path":"%s/.envrc","allowed":1}}}\n' "$PWD"
fi
`

func newClient(t *testing.T) (*Client, string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "direnv")
	if err := os.WriteFile(bin, []byte(fakeDirenv), 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(dir, "log")
	c := &Client{
		LookPath: func(string) (string, error) { return bin, nil },
		Env:      []string{"PATH=/usr/bin:/bin", "PWD=/somewhere/else", "FAKE_LOG=" + log},
	}
	return c, log
}

func TestFoundRCUsesLogicalPWD(t *testing.T) {
	c, log := newClient(t)
	dir := t.TempDir()
	rc, err := c.FoundRC(dir)
	if err != nil {
		t.Fatal(err)
	}
	if rc.Path != dir+"/.envrc" || rc.Allowed != NotAllowed {
		t.Errorf("rc = %+v", rc)
	}
	if data, _ := os.ReadFile(log); !strings.Contains(string(data), "status --json PWD="+dir) {
		t.Errorf("direnv did not see PWD=%s: %s", dir, data)
	}
}

func TestAllowAndMissingBinary(t *testing.T) {
	c, log := newClient(t)
	if err := c.Allow("/p"); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(log); !strings.HasPrefix(string(data), "allow /p") {
		t.Errorf("log = %s", data)
	}

	missing := &Client{LookPath: func(string) (string, error) { return "", exec.ErrNotFound }}
	if missing.Installed() || missing.Allow("/p") == nil {
		t.Error("missing direnv not reported")
	}
	if _, err := missing.FoundRC("/p"); err == nil {
		t.Error("FoundRC without direnv returned no error")
	}
}
