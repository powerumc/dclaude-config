// Package direnv runs the direnv commands dclaude needs.
package direnv

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strings"
	"time"
)

// Allow states reported by `direnv status --json`.
const (
	Allowed = iota
	NotAllowed
	Denied
)

var errNotInstalled = errors.New("direnv is not installed")

// Client runs the direnv binary that LookPath resolves.
type Client struct {
	LookPath func(string) (string, error)
	Env      []string
	Stdout   io.Writer
	Stderr   io.Writer
}

func (c *Client) bin() string {
	bin, err := c.LookPath("direnv")
	if err != nil {
		return ""
	}
	return bin
}

// Installed reports whether direnv can be found.
func (c *Client) Installed() bool { return c.bin() != "" }

// Allow approves the .envrc in dir.
func (c *Client) Allow(dir string) error {
	bin := c.bin()
	if bin == "" {
		return errNotInstalled
	}
	cmd := exec.Command(bin, "allow", dir)
	cmd.Env = c.Env
	cmd.Stdout, cmd.Stderr = c.Stdout, c.Stderr
	return cmd.Run()
}

// RC is an .envrc as direnv sees it.
type RC struct {
	Path    string `json:"path"`
	Allowed int    `json:"allowed"`
}

// FoundRC reports the .envrc direnv would load in dir, or nil when there is none.
func (c *Client) FoundRC(dir string) (*RC, error) {
	bin := c.bin()
	if bin == "" {
		return nil, errNotInstalled
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "status", "--json")
	cmd.Dir = dir
	// direnv walks up from $PWD; keep the logical path the user's shell sees.
	cmd.Env = append(withoutVar(c.Env, "PWD"), "PWD="+dir)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var st struct {
		State struct {
			FoundRC *RC `json:"foundRC"`
		} `json:"state"`
	}
	if err := json.Unmarshal(out, &st); err != nil {
		return nil, err
	}
	return st.State.FoundRC, nil
}

func withoutVar(env []string, key string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !strings.HasPrefix(kv, key+"=") {
			out = append(out, kv)
		}
	}
	return out
}
