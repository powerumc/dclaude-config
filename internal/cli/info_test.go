package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/powerumc/dclaude-config/internal/keychain"
	"github.com/powerumc/dclaude-config/internal/profile"
)

func TestInfoShowsProfileAccountAndUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok-123" {
			http.Error(w, "nope", http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"five_hour":{"utilization":23,"resets_at":"2026-10-15T14:00:00Z"},"seven_day":{"utilization":41,"resets_at":"2026-10-18T09:00:00Z"}}`))
	}))
	defer srv.Close()

	h := newHarness(t, true)
	cwd := h.chdir("work/a")
	cfg := filepath.Join(cwd, profile.DirName)
	h.mustRun("init")
	h.app.Usage.URL, h.app.Usage.HTTP = srv.URL, srv.Client()
	expires := testNow.Add(time.Hour).UnixMilli()
	h.kc.items[keychain.OAuthService(cfg)+"|tester"] = fmt.Sprintf(`{"claudeAiOauth":{"accessToken":"tok-123","expiresAt":%d}}`, expires)
	h.setEnv("CLAUDE_CONFIG_DIR=" + cfg) // as if direnv had loaded the block
	h.setEnv("FAKE_FOUND_RC=" + filepath.Join(cwd, ".envrc"))

	h.chdir("work/a/src/pkg")
	h.mustRun("info")

	out := h.stdout.String()
	for _, want := range []string{
		"profile   " + cfg,
		"direnv    allowed",
		"shell     active",
		"account   dev@example.com, Example, max (claude.ai)",
		"Keychain: " + keychain.OAuthService(cfg),
		"5h 23%",
		"7d 41%",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestInfoExpiredTokenSkipsUsageCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("usage endpoint called with an expired token")
	}))
	defer srv.Close()

	h := newHarness(t, true)
	cfg := filepath.Join(h.chdir("work/a"), profile.DirName)
	h.mustRun("init")
	h.app.Usage.URL, h.app.Usage.HTTP = srv.URL, srv.Client()
	expired := testNow.Add(-time.Minute).UnixMilli()
	h.kc.items[keychain.OAuthService(cfg)+"|tester"] = fmt.Sprintf(`{"claudeAiOauth":{"accessToken":"t","expiresAt":%d}}`, expired)

	h.mustRun("info")
	if !strings.Contains(h.stdout.String(), "token expired") {
		t.Errorf("stdout:\n%s", h.stdout.String())
	}
}

func TestInfoWithoutProfileReportsGlobalAccount(t *testing.T) {
	h := newHarness(t, true)
	h.chdir("elsewhere")
	h.setEnv("FAKE_GLOBAL_LOGGED_IN=1")

	h.mustRun("info")

	if out := h.stdout.String(); !strings.Contains(out, "profile   none") || !strings.Contains(out, "global@example.com") {
		t.Errorf("stdout:\n%s", out)
	}
	if !strings.Contains(h.stderr.String(), "ANTHROPIC_API_KEY") {
		t.Errorf("auth override not flagged:\n%s", h.stderr.String())
	}
	if !strings.Contains(h.calls(), "claude auth status --json | CLAUDE_CONFIG_DIR=<unset>") {
		t.Errorf("global status not queried without CLAUDE_CONFIG_DIR:\n%s", h.calls())
	}
}

func TestInfoFlagsShadowingEnvrc(t *testing.T) {
	h := newHarness(t, true)
	h.chdir("work/a")
	h.mustRun("init")
	sub := h.chdir("work/a/sub")
	os.WriteFile(filepath.Join(sub, ".envrc"), []byte("export X=1\n"), 0o644)
	h.setEnv("FAKE_FOUND_RC=" + filepath.Join(sub, ".envrc"))

	h.mustRun("info")
	if !strings.Contains(h.stdout.String(), "hides this profile") {
		t.Errorf("stdout:\n%s", h.stdout.String())
	}
}

func TestInfoAcceptsCloserEnvrcChainingUp(t *testing.T) {
	h := newHarness(t, true)
	h.chdir("work/a")
	h.mustRun("init")
	sub := h.chdir("work/a/sub")
	os.WriteFile(filepath.Join(sub, ".envrc"), []byte("source_up\nexport X=1\n"), 0o644)
	deep := h.chdir("work/a/sub/deep")
	os.WriteFile(filepath.Join(deep, ".envrc"), []byte("source_up_if_exists\n"), 0o644)
	h.setEnv("FAKE_FOUND_RC=" + filepath.Join(deep, ".envrc"))

	h.mustRun("info")
	if out := h.stdout.String(); strings.Contains(out, "hides this profile") || !strings.Contains(out, "direnv    allowed") {
		t.Errorf("stdout:\n%s", out)
	}
}

func TestInfoNamesTheEnvrcThatBreaksTheChain(t *testing.T) {
	h := newHarness(t, true)
	h.chdir("work/a")
	h.mustRun("init")
	sub := h.chdir("work/a/sub")
	os.WriteFile(filepath.Join(sub, ".envrc"), []byte("export X=1\n"), 0o644)
	deep := h.chdir("work/a/sub/deep")
	os.WriteFile(filepath.Join(deep, ".envrc"), []byte("source_up\n"), 0o644)
	h.setEnv("FAKE_FOUND_RC=" + filepath.Join(deep, ".envrc"))

	h.mustRun("info")
	if want := "direnv    " + filepath.Join(sub, ".envrc") + " is closer and hides this profile"; !strings.Contains(h.stdout.String(), want) {
		t.Errorf("missing %q in:\n%s", want, h.stdout.String())
	}
}
