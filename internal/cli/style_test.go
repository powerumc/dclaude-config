package cli

import (
	"os"
	"strings"
	"testing"
)

func TestOutputIsPlainWhenNotATerminal(t *testing.T) {
	h := newHarness(t, true)
	h.chdir("work/a")
	h.mustRun("init")
	h.mustRun("info")

	if out := h.stdout.String() + h.stderr.String(); strings.Contains(out, "\x1b[") {
		t.Errorf("escape codes in output to a buffer:\n%q", out)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if h.app.styleFor(w).on {
		t.Error("a pipe is not a terminal")
	}
}

func TestColorsWhenForced(t *testing.T) {
	h := newHarness(t, true)
	h.setEnv("CLICOLOR_FORCE=1")
	h.chdir("work/a")
	h.mustRun("init")

	h.mustRun("info")

	out := h.stdout.String()
	for _, want := range []string{
		"\x1b[1mdirenv    \x1b[0m\x1b[32mallowed\x1b[0m",
		"\x1b[1mshell     \x1b[0m\x1b[33mnot active: ",
		"\x1b[1mdev@example.com\x1b[0m",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%q", want, out)
		}
	}
	if !strings.Contains(h.stderr.String(), "\x1b[33mwarning:\x1b[0m ") {
		t.Errorf("stderr: %q", h.stderr.String())
	}
}

func TestNoColorWinsOverForce(t *testing.T) {
	h := newHarness(t, true)
	h.setEnv("CLICOLOR_FORCE=1")
	h.setEnv("NO_COLOR=1")
	h.chdir("work/a")
	h.mustRun("init")
	h.mustRun("info")

	if out := h.stdout.String() + h.stderr.String(); strings.Contains(out, "\x1b[") {
		t.Errorf("NO_COLOR ignored:\n%q", out)
	}
}

func TestUsageLevels(t *testing.T) {
	s := style{on: true}
	for _, tc := range []struct {
		u    float64
		code string
	}{{0, "32"}, {49, "32"}, {50, "33"}, {79, "33"}, {80, "31"}, {100, "31"}} {
		if got := s.level(tc.u, "x"); got != "\x1b["+tc.code+"mx\x1b[0m" {
			t.Errorf("level(%v) = %q", tc.u, got)
		}
	}
}
