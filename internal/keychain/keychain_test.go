package keychain

import (
	"testing"

	"golang.org/x/text/unicode/norm"
)

func TestProfileHashMatchesClaudeCode(t *testing.T) {
	// Reference values computed outside Go (shasum, Python hashlib + NFC).
	accented := "/Users/test/café/a/.claude_config"
	cases := []struct{ dir, want string }{
		{"/Users/test/work/a/.claude_config", "affb3d47"},
		{norm.NFC.String(accented), "98b6d1c7"},
		{norm.NFD.String(accented), "98b6d1c7"}, // raw NFD bytes would hash to db3162de
	}
	for _, c := range cases {
		if got := ProfileHash(c.dir); got != c.want {
			t.Errorf("ProfileHash(%q) = %s, want %s", c.dir, got, c.want)
		}
	}
	if got := OAuthService("/Users/test/work/a/.claude_config"); got != "Claude Code-credentials-affb3d47" {
		t.Errorf("OAuthService = %s", got)
	}
	if got := APIKeyService("/Users/test/work/a/.claude_config"); got != "Claude Code-affb3d47" {
		t.Errorf("APIKeyService = %s", got)
	}
}

func TestAccount(t *testing.T) {
	user := func(name string) func(string) string {
		return func(k string) string {
			if k == "USER" {
				return name
			}
			return ""
		}
	}
	if got := Account(user("powerumc")); got != "powerumc" {
		t.Errorf("got %s, want powerumc", got)
	}
	if got := Account(user("josé")); got != "claude-code-user" {
		t.Errorf("non-ASCII user: got %s, want claude-code-user", got)
	}
}
