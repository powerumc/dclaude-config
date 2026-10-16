package cli

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/powerumc/dclaude-config/internal/profile"
)

func (a *App) cmdInit(args []string) error {
	share, args, err := splitShareFlags(args)
	if err != nil {
		return err
	}
	if err := a.requireDirenv(); err != nil {
		return err
	}
	cwd, err := a.Getwd()
	if err != nil {
		return err
	}
	root, err := profile.Canonical(cwd)
	if err != nil {
		return err
	}
	home, err := profile.Canonical(a.Home)
	if err != nil {
		return err
	}
	if err := profile.CheckInitTarget(root, home); err != nil {
		return err
	}
	if dir := filepath.Join(cwd, profile.DirName); exists(dir) {
		if !profile.IsProfileDir(dir) {
			return fmt.Errorf("%s already exists but was not created by dclaude", dir)
		}
		return fmt.Errorf("%s already exists; to log in again run: dclaude login", dir)
	}

	parent, err := profile.Find(filepath.Dir(cwd))
	if err != nil && !errors.Is(err, profile.ErrNotFound) {
		return err
	}
	if parent != nil {
		a.warnf("this profile takes precedence over the parent profile at %s", parent.Root)
	}
	if v := a.getenv("CLAUDE_CONFIG_DIR"); v != "" && (parent == nil || v != parent.ConfigDir()) {
		a.warnf("CLAUDE_CONFIG_DIR=%s is set in this shell; inside this directory the dclaude profile wins", v)
	}

	// The canonical path is recorded once and reused everywhere: Claude Code
	// keys the Keychain entry by this exact string.
	p, err := profile.Create(cwd, filepath.Join(root, profile.DirName), a.Now())
	if err != nil {
		return err
	}
	fmt.Fprintf(a.Stdout, "%s %s\n", a.out().ok("created profile:"), p.ConfigDir())
	if err := a.chooseSharing(p, share); err != nil {
		return err
	}
	if err := a.syncSharing(p); err != nil {
		return err
	}
	if err := a.ensureEnvrc(p); err != nil {
		return err
	}
	return a.login(p, args)
}
