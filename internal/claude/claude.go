// Package claude drives the Claude Code CLI and reads the files Claude Code
// keeps in a config directory. It never writes or refreshes credentials.
package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// AuthOverrideVars outrank a profile's /login credential (or relocate its
// credential store), so profile shells and dclaude's own claude runs drop them.
var AuthOverrideVars = []string{
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_AUTH_TOKEN",
	"CLAUDE_CODE_OAUTH_TOKEN",
	"CLAUDE_CODE_OAUTH_TOKEN_FILE_DESCRIPTOR",
	"CLAUDE_CODE_API_KEY_FILE_DESCRIPTOR",
	"CLAUDE_SECURESTORAGE_CONFIG_DIR",
}

// Env returns base without CLAUDE_CONFIG_DIR and AuthOverrideVars, then
// points CLAUDE_CONFIG_DIR at configDir; "" keeps the default ~/.claude profile.
func Env(base []string, configDir string) []string {
	drop := map[string]bool{"CLAUDE_CONFIG_DIR": true}
	for _, k := range AuthOverrideVars {
		drop[k] = true
	}
	env := make([]string, 0, len(base)+1)
	for _, kv := range base {
		if k, _, _ := strings.Cut(kv, "="); !drop[k] {
			env = append(env, kv)
		}
	}
	if configDir != "" {
		env = append(env, "CLAUDE_CONFIG_DIR="+configDir)
	}
	return env
}

// CLI runs the claude binary that LookPath resolves.
type CLI struct {
	LookPath func(string) (string, error)
	Env      []string
	Stdin    io.Reader
	Stdout   io.Writer
	Stderr   io.Writer
}

func (c *CLI) command(ctx context.Context, configDir string, args ...string) (*exec.Cmd, error) {
	bin, err := c.LookPath("claude")
	if err != nil {
		return nil, fmt.Errorf("claude not found: %w", err)
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = Env(c.Env, configDir)
	return cmd, nil
}

// Run runs claude against configDir, attached to the terminal; login and
// logout are interactive.
func (c *CLI) Run(configDir string, args ...string) error {
	cmd, err := c.command(context.Background(), configDir, args...)
	if err != nil {
		return err
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = c.Stdin, c.Stdout, c.Stderr
	return cmd.Run()
}

// Status is the output of `claude auth status --json`.
type Status struct {
	LoggedIn         bool   `json:"loggedIn"`
	AuthMethod       string `json:"authMethod"`
	APIProvider      string `json:"apiProvider"`
	Email            string `json:"email"`
	OrgName          string `json:"orgName"`
	SubscriptionType string `json:"subscriptionType"`
}

// AuthStatus asks claude who configDir is logged in as. It is a local check;
// claude makes no API call for it.
func (c *CLI) AuthStatus(configDir string) (*Status, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd, err := c.command(ctx, configDir, "auth", "status", "--json")
	if err != nil {
		return nil, err
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	var st Status
	if err := json.Unmarshal(stdout.Bytes(), &st); err != nil {
		if runErr != nil {
			return nil, fmt.Errorf("claude auth status failed: %v %s", runErr, strings.TrimSpace(stderr.String()))
		}
		return nil, fmt.Errorf("cannot parse claude auth status output: %w", err)
	}
	return &st, nil
}
