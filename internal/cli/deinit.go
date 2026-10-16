package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/powerumc/dclaude-config/internal/claude"
	"github.com/powerumc/dclaude-config/internal/envrc"
	"github.com/powerumc/dclaude-config/internal/profile"
)

func (a *App) cmdDeinit(args []string) error {
	yes, force := false, false
	for _, arg := range args {
		switch arg {
		case "-y", "--yes":
			yes = true
		case "--force":
			force = true
		default:
			return fmt.Errorf("unknown option: %s", arg)
		}
	}
	p, _, err := a.currentProfile()
	if errors.Is(err, profile.ErrNotFound) {
		return errNoProfile
	}
	if err != nil {
		return err
	}
	if p.Linked() {
		return a.deinitLink(p, yes)
	}
	if pids := claude.LiveSessions(p.Dir()); len(pids) > 0 && !force {
		return fmt.Errorf("a Claude session is still running with this profile (PID %v); stop it first or pass --force", pids)
	}

	// A moved profile is never handed to claude: it would recreate the stale path.
	moved := p.Moved()
	s := a.out()
	fmt.Fprintf(a.Stdout, "profile: %s\n", p.Dir())
	var st *claude.Status
	var stErr error
	if moved {
		fmt.Fprintf(a.Stdout, "%s; cleaning up without logging out.\n", s.warn("this profile was moved (recorded path: "+p.ConfigDir()+")"))
	} else if st, stErr = a.claudeCLI().AuthStatus(p.ConfigDir()); stErr == nil && st.LoggedIn {
		fmt.Fprintf(a.Stdout, "account: %s\n", describeAccount(st, s))
	}
	dir := "the profile directory (including its conversation history)"
	switch p.Marker.Sharing {
	case profile.Shared:
		dir = "the profile directory (including its conversation history; the shared ~/.claude is kept)"
	case profile.SharedHistory:
		dir = "the profile directory (the shared ~/.claude, conversation history included, is kept)"
	}
	steps := "removes the Keychain items, " + dir + " and the dclaude block in .envrc."
	if moved {
		fmt.Fprintln(a.Stdout, s.warn("This "+steps))
	} else {
		fmt.Fprintln(a.Stdout, s.warn("This logs out, then "+steps))
	}
	links := a.liveLinks(p)
	if len(links) > 0 {
		fmt.Fprintln(a.Stdout, s.warn("It also removes the links to this profile in: "+strings.Join(links, ", ")))
	}
	if !yes && !a.confirm("Continue?") {
		return errors.New("cancelled")
	}

	if !moved && (stErr != nil || st.LoggedIn) {
		if err := a.claudeCLI().Run(p.ConfigDir(), "auth", "logout"); err != nil {
			a.warnf("claude auth logout failed: %v", err)
		}
	}
	targets := []string{p.ConfigDir()}
	if moved {
		targets = nil
		if !exists(p.ConfigDir()) { // a copy's original still owns its items
			targets = append(targets, p.ConfigDir())
		}
		if root, err := profile.Canonical(p.Root); err == nil {
			targets = append(targets, filepath.Join(root, profile.DirName))
		}
	}
	for _, dir := range targets {
		for _, svc := range a.deleteKeychainItems(dir) {
			fmt.Fprintf(a.Stdout, "removed Keychain item: %s\n", svc)
		}
	}

	for _, root := range links {
		if err := a.removeLink(root); err != nil {
			return err
		}
	}
	if err := a.removeEnvrcBlock(p.EnvrcPath()); err != nil {
		return err
	}
	if err := os.RemoveAll(p.Dir()); err != nil {
		return err
	}
	fmt.Fprintf(a.Stdout, "%s %s\n", s.ok("removed profile:"), p.Dir())
	return nil
}

// deinitLink undoes `dclaude ln` here. The profile it points to, and that
// profile's login, stay.
func (a *App) deinitLink(p *profile.Profile, yes bool) error {
	fmt.Fprintf(a.Stdout, "link: %s -> %s\n", p.Dir(), p.ConfigDir())
	fmt.Fprintln(a.Stdout, a.out().warn("This removes the link and the dclaude block in .envrc; the profile it points to, and its login, stay."))
	if !yes && !a.confirm("Continue?") {
		return errors.New("cancelled")
	}
	root, err := profile.Canonical(p.Root)
	if err != nil {
		return err
	}
	if err := a.removeLink(p.Root); err != nil {
		return err
	}
	if src, err := profile.Find(filepath.Dir(p.ConfigDir())); err == nil {
		src.Marker.Links = slices.DeleteFunc(src.Marker.Links, func(l string) bool { return l == root })
		if err := src.Save(); err != nil {
			a.warnf("could not update %s: %v", src.Dir(), err)
		}
	}
	return nil
}

// liveLinks lists the directories whose .claude_config still links to p.
func (a *App) liveLinks(p *profile.Profile) []string {
	var roots []string
	for _, root := range p.Marker.Links {
		link := filepath.Join(root, profile.DirName)
		if info, err := os.Lstat(link); err == nil && info.Mode()&fs.ModeSymlink != 0 && samePath(link, p.Dir()) {
			roots = append(roots, root)
		}
	}
	return roots
}

// removeLink deletes the .claude_config link in root and its .envrc block.
func (a *App) removeLink(root string) error {
	link := filepath.Join(root, profile.DirName)
	if err := os.Remove(link); err != nil {
		return err
	}
	if err := a.removeEnvrcBlock(filepath.Join(root, ".envrc")); err != nil {
		return err
	}
	fmt.Fprintf(a.Stdout, "%s %s\n", a.out().ok("removed link:"), link)
	return nil
}

// removeEnvrcBlock takes the dclaude block out of the .envrc at path.
func (a *App) removeEnvrcBlock(path string) error {
	r, err := envrc.Uninstall(path)
	if err != nil {
		return err
	}
	switch {
	case r.FileRemoved:
		fmt.Fprintf(a.Stdout, "removed %s (it only held the dclaude block)\n", path)
	case r.Found:
		fmt.Fprintf(a.Stdout, "removed the dclaude block from %s; approve the remaining content again: %s\n", path, a.out().cmd(a.allowCommand(filepath.Dir(path))))
	}
	return nil
}
