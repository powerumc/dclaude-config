package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/powerumc/dclaude-config/internal/claude"
	"github.com/powerumc/dclaude-config/internal/direnv"
	"github.com/powerumc/dclaude-config/internal/envrc"
	"github.com/powerumc/dclaude-config/internal/profile"
)

func (a *App) cmdLogin(args []string) error {
	share, args, err := splitShareFlags(args)
	if err != nil {
		return err
	}
	p, _, err := a.currentProfile()
	if errors.Is(err, profile.ErrNotFound) {
		return errNoProfile
	}
	if err != nil {
		return err
	}
	if err := a.requireDirenv(); err != nil {
		return err
	}
	if p.Moved() {
		if err := a.repoint(p); err != nil {
			return err
		}
	}
	if err := a.ensureEnvrc(p); err != nil {
		return err
	}
	switch current := p.Marker.Sharing; {
	case current == profile.SharingUnset:
		// Profiles made before sharing existed choose once, here.
		if err := a.chooseSharing(p, share); err != nil {
			return err
		}
	case share != profile.SharingUnset && share != current:
		return fmt.Errorf("this profile already chose %q; changing it is not supported yet", current)
	}
	if err := a.syncSharing(p); err != nil {
		return err
	}
	return a.login(p, args)
}

func (a *App) login(p *profile.Profile, args []string) error {
	fmt.Fprintln(a.Stdout, "starting Claude login...")
	if err := a.claudeCLI().Run(p.ConfigDir(), loginArgs(args)...); err != nil {
		return fmt.Errorf("login did not complete (the profile was kept); retry with: dclaude login (%v)", err)
	}
	st, err := a.claudeCLI().AuthStatus(p.ConfigDir())
	if err != nil {
		return err
	}
	if !st.LoggedIn {
		return errors.New("login could not be confirmed; retry with: dclaude login")
	}
	s := a.out()
	fmt.Fprintf(a.Stdout, "%s %s\n", s.ok("logged in:"), describeAccount(st, s))
	if a.getenv("CLAUDE_CONFIG_DIR") != p.ConfigDir() {
		a.activate(p)
	}
	return nil
}

// requireDirenv stops before any change when direnv is missing: without it,
// entering the directory never switches accounts.
func (a *App) requireDirenv() error {
	if a.direnvCLI().Installed() {
		return nil
	}
	install := "Install it with your package manager (for example: sudo apt install direnv), then run this command again."
	if runtime.GOOS == "darwin" {
		install = "Install it, then run this command again:\n  brew install direnv"
	}
	return fmt.Errorf("direnv is not installed; dclaude needs it to switch accounts when you enter a directory.\n%s\nOther ways to install: %s", install, direnv.InstallDocsURL)
}

// activate tells what is still needed before entering the profile directory
// switches accounts: approving its .envrc, and direnv's shell hook, which it
// offers to add.
func (a *App) activate(p *profile.Profile) {
	s := a.out()
	ready := true
	if rc, err := a.direnvCLI().FoundRC(p.Root); err == nil && rc != nil && samePath(rc.Path, p.EnvrcPath()) && rc.Allowed != direnv.Allowed {
		ready = false
		how := "approve it once"
		if envrc.HasOtherContent(p.EnvrcPath()) {
			how = "review it, then approve it yourself"
		}
		fmt.Fprintf(a.Stdout, "%s; %s:\n  %s\n", s.warn("direnv has not approved "+p.EnvrcPath()+" yet"), how, s.cmd(a.allowCommand(p.Root)))
	}
	h, ok := direnv.HookFor(a.getenv, a.Home)
	switch {
	case !ok:
		fmt.Fprintf(a.Stdout, "%s: %s\n", s.warn("make sure direnv is hooked into your shell"), direnv.HookDocsURL)
	case !h.Installed():
		a.offerHook(h)
	case ready:
		fmt.Fprintln(a.Stdout, s.ok("direnv will apply it to this shell at the next prompt."))
	}
}

// offerHook prints the commands that hook direnv into the shell, then offers
// to append the hook line to the shell's startup file.
func (a *App) offerHook(h *direnv.Hook) {
	s := a.out()
	fmt.Fprintf(a.Stdout, "%s, so entering this directory does not switch accounts. To add the hook and load it in this terminal:\n", s.warn("direnv is not hooked into "+h.Shell+" yet"))
	for _, c := range h.Commands() {
		fmt.Fprintf(a.Stdout, "  %s\n", s.cmd(c))
	}
	if !a.confirm(fmt.Sprintf("Add the hook to %s now?", h.RCDisplay())) {
		return
	}
	if err := h.Install(); err != nil {
		a.warnf("could not update %s: %v", h.RCFile, err)
		return
	}
	fmt.Fprintf(a.Stdout, "%s; new terminals load it. To load it in this terminal now, run:\n  %s\n", s.ok("added the direnv hook to "+h.RCDisplay()), s.cmd(h.Line))
}

func (a *App) allowCommand(root string) string {
	if cwd, err := a.Getwd(); err == nil && samePath(cwd, root) {
		return "direnv allow"
	}
	return "direnv allow " + direnv.Quote(root)
}

// loginArgs passes everything through to `claude auth login`, minus a leading "--".
func loginArgs(args []string) []string {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	return append([]string{"auth", "login"}, args...)
}

// repoint records the new path of a moved or copied profile. The Keychain
// items of the old path are deleted only when nothing lives there anymore;
// a copy's original still owns them.
func (a *App) repoint(p *profile.Profile) error {
	root, err := profile.Canonical(p.Root)
	if err != nil {
		return err
	}
	old, next := p.ConfigDir(), filepath.Join(root, profile.DirName)
	fmt.Fprintf(a.Stdout, "%s %s -> %s (logging in again for the new path)\n", a.out().warn("directory changed:"), old, next)
	if !exists(old) {
		for _, svc := range a.deleteKeychainItems(old) {
			fmt.Fprintf(a.Stdout, "removed Keychain item for the old path: %s\n", svc)
		}
	}
	p.Marker.ConfigDir = next
	return p.Save()
}

// ensureEnvrc writes the block when it is missing or stale, then hands it to direnv.
func (a *App) ensureEnvrc(p *profile.Profile) error {
	if exported, ok := envrc.ExportedConfigDir(p.EnvrcPath()); ok && exported == p.ConfigDir() {
		return nil
	}
	r, err := envrc.Install(p.EnvrcPath(), envrc.Block(p.ConfigDir(), claude.AuthOverrideVars))
	if err != nil {
		return err
	}
	if r.HadOther {
		// Approving would also approve content dclaude did not write; activate
		// asks the user to.
		fmt.Fprintf(a.Stdout, "added the dclaude block to the existing %s\n", p.EnvrcPath())
		return nil
	}
	if err := a.direnvCLI().Allow(p.Root); err != nil {
		a.warnf("direnv allow failed: %v", err)
	}
	return nil
}
