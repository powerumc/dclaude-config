package claude

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnvDropsAuthOverrides(t *testing.T) {
	env := Env([]string{
		"PATH=/bin", "ANTHROPIC_API_KEY=k", "CLAUDE_CODE_OAUTH_TOKEN=t",
		"CLAUDE_SECURESTORAGE_CONFIG_DIR=/s", "CLAUDE_CONFIG_DIR=/other",
	}, "/p/.claude_config")
	if got := strings.Join(env, " "); got != "PATH=/bin CLAUDE_CONFIG_DIR=/p/.claude_config" {
		t.Fatalf("env = %s", got)
	}
	if got := strings.Join(Env([]string{"PATH=/bin", "CLAUDE_CONFIG_DIR=/x"}, ""), " "); got != "PATH=/bin" {
		t.Fatalf("global env = %s", got)
	}
}

type mapStore map[string]string

func (m mapStore) Read(service, account string) (string, bool, error) {
	v, ok := m[service+"|"+account]
	return v, ok, nil
}

func (m mapStore) Delete(string, string) (bool, error) { return false, nil }

func TestReadCredentialPrefersKeychainOverFile(t *testing.T) {
	home := t.TempDir()
	os.WriteFile(filepath.Join(home, ".credentials.json"), []byte(`{"claudeAiOauth":{"accessToken":"from-file"}}`), 0o600)

	store := mapStore{"svc|me": `{"claudeAiOauth":{"accessToken":"from-keychain","expiresAt":1000}}`}
	cred, src, err := ReadCredential(store, "me", "svc", home)
	if err != nil || src != FromKeychain || cred.AccessToken != "from-keychain" {
		t.Fatalf("keychain: %+v %s %v", cred, src, err)
	}
	if !cred.Expired(time.UnixMilli(1000)) || cred.Expired(time.UnixMilli(999)) {
		t.Error("Expired boundary is wrong")
	}

	cred, src, err = ReadCredential(mapStore{}, "me", "svc", home)
	if err != nil || src != FromFile || cred.AccessToken != "from-file" {
		t.Fatalf("file fallback: %+v %s %v", cred, src, err)
	}

	cred, _, err = ReadCredential(mapStore{}, "me", "svc", t.TempDir())
	if err != nil || cred != nil {
		t.Fatalf("nothing stored: %+v %v", cred, err)
	}
}

func TestLiveSessions(t *testing.T) {
	home := t.TempDir()
	sessions := filepath.Join(home, "sessions")
	os.Mkdir(sessions, 0o700)
	os.WriteFile(filepath.Join(sessions, "a.json"), []byte(fmt.Sprintf(`{"pid":%d}`, os.Getpid())), 0o600)
	os.WriteFile(filepath.Join(sessions, "b.json"), []byte(`{"pid":999999}`), 0o600) // above any pid_max
	os.WriteFile(filepath.Join(sessions, "c.json"), []byte(`not json`), 0o600)

	if got := LiveSessions(home); len(got) != 1 || got[0] != os.Getpid() {
		t.Fatalf("LiveSessions = %v, want [%d]", got, os.Getpid())
	}
}
