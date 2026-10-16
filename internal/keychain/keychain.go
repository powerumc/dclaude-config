// Package keychain names and removes the macOS Keychain items that Claude Code
// keeps per config directory. It never writes: logging in and refreshing
// tokens stay Claude Code's job.
package keychain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os/exec"
	"os/user"
	"regexp"
	"runtime"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"
)

// DefaultOAuthService is the unsuffixed item Claude Code uses when
// CLAUDE_CONFIG_DIR is unset.
const DefaultOAuthService = "Claude Code-credentials"

// ProfileHash is the suffix Claude Code derives from the raw CLAUDE_CONFIG_DIR
// string: NFC-normalized, never resolved, sha256 hex truncated to 8 chars.
func ProfileHash(configDir string) string {
	sum := sha256.Sum256([]byte(norm.NFC.String(configDir)))
	return hex.EncodeToString(sum[:])[:8]
}

// OAuthService names the item holding a profile's claude.ai login.
func OAuthService(configDir string) string {
	return DefaultOAuthService + "-" + ProfileHash(configDir)
}

// APIKeyService names the item holding a profile's Console API key login.
func APIKeyService(configDir string) string {
	return "Claude Code-" + ProfileHash(configDir)
}

var accountRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// Account mirrors the account name Claude Code files its items under:
// $USER, else the OS user name, else a fixed fallback.
func Account(getenv func(string) string) string {
	name := getenv("USER")
	if name == "" {
		if u, err := user.Current(); err == nil {
			name = u.Username
		}
	}
	if !accountRe.MatchString(name) {
		return "claude-code-user"
	}
	return name
}

// Store is the part of the Keychain dclaude touches.
type Store interface {
	Read(service, account string) (secret string, found bool, err error)
	Delete(service, account string) (deleted bool, err error)
}

// Default returns the platform store: Apple's security CLI on macOS and a
// no-op elsewhere.
func Default() Store {
	if runtime.GOOS == "darwin" {
		return Security{Path: "/usr/bin/security"}
	}
	return None{}
}

// errSecItemNotFound is the security exit code for a missing item.
const errSecItemNotFound = 44

// Security runs Apple's security binary by absolute path, so nothing earlier
// on PATH can intercept secrets.
type Security struct{ Path string }

func (s Security) run(args ...string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, s.Path, args...).Output()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return out, exitErr.ExitCode(), nil
	}
	return out, 0, err
}

// Read returns the item's secret; found is false when the item does not exist.
func (s Security) Read(service, account string) (string, bool, error) {
	out, code, err := s.run("find-generic-password", "-a", account, "-s", service, "-w")
	switch {
	case err != nil:
		return "", false, err
	case code == errSecItemNotFound:
		return "", false, nil
	case code != 0:
		return "", false, errors.New("security find-generic-password failed")
	}
	return strings.TrimSuffix(string(out), "\n"), true, nil
}

// Delete removes the item and any duplicates; deleted is false when there was none.
func (s Security) Delete(service, account string) (bool, error) {
	deleted := false
	for range 3 { // one call removes one item
		_, code, err := s.run("delete-generic-password", "-a", account, "-s", service)
		switch {
		case err != nil:
			return deleted, err
		case code == errSecItemNotFound:
			return deleted, nil
		case code != 0:
			return deleted, errors.New("security delete-generic-password failed")
		}
		deleted = true
	}
	return deleted, nil
}

// None is the store off macOS, where Claude Code keeps credentials in the
// config directory itself.
type None struct{}

// Read never finds anything.
func (None) Read(string, string) (string, bool, error) { return "", false, nil }

// Delete never deletes anything.
func (None) Delete(string, string) (bool, error) { return false, nil }
