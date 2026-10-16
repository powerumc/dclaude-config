package claude

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/powerumc/dclaude-config/internal/keychain"
)

// Credential is the claude.ai login Claude Code stores in the Keychain or in
// .credentials.json.
type Credential struct {
	AccessToken      string   `json:"accessToken"`
	ExpiresAt        int64    `json:"expiresAt"` // unix milliseconds
	Scopes           []string `json:"scopes"`
	SubscriptionType string   `json:"subscriptionType"`
}

// Expired reports whether the access token has passed its expiry.
func (c *Credential) Expired(now time.Time) bool {
	return c.ExpiresAt > 0 && now.UnixMilli() >= c.ExpiresAt
}

// Source tells where a credential was read from.
type Source string

// Credential sources, in the order Claude Code consults them.
const (
	FromKeychain Source = "keychain"
	FromFile     Source = "file"
)

// ReadCredential reads the stored login without refreshing it: the Keychain
// item first, as Claude Code does, then configHome/.credentials.json. It
// returns a nil Credential when neither holds a claude.ai login.
func ReadCredential(store keychain.Store, account, service, configHome string) (*Credential, Source, error) {
	secret, found, err := store.Read(service, account)
	if err != nil {
		return nil, "", fmt.Errorf("keychain read failed: %w", err)
	}
	source := FromKeychain
	if !found {
		data, err := os.ReadFile(filepath.Join(configHome, ".credentials.json"))
		if errors.Is(err, fs.ErrNotExist) {
			return nil, "", nil
		}
		if err != nil {
			return nil, "", err
		}
		secret, source = string(data), FromFile
	}
	var doc struct {
		ClaudeAiOauth *Credential `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal([]byte(secret), &doc); err != nil {
		return nil, "", fmt.Errorf("cannot parse stored credential (%s): %w", source, err)
	}
	return doc.ClaudeAiOauth, source, nil
}
