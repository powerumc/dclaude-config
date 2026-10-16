package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/powerumc/dclaude-config/internal/profile"
)

var errNotWorktree = errors.New("not in a git worktree; name the directory whose profile to use: dclaude ln <dir>")

// cmdLn makes the current directory use an existing profile, login included:
// the one at or above dir, or in a git worktree the main checkout's.
func (a *App) cmdLn(args []string) error {
	if len(args) > 1 || (len(args) == 1 && strings.HasPrefix(args[0], "-")) {
		return errors.New("usage: dclaude ln [dir]")
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
	src, err := a.lnSource(cwd, root, args)
	if err != nil {
		return err
	}

	switch cur, err := profile.Find(cwd); {
	case errors.Is(err, profile.ErrNotFound):
	case err != nil:
		return err
	case cur.ConfigDir() == src.ConfigDir() && cur.Root == cwd && cur.Linked():
		// Linked before: running ln again repairs the .envrc block.
		fmt.Fprintf(a.Stdout, "this directory is already linked to the profile at %s\n", src.Root)
		return a.ensureEnvrc(cur)
	case cur.ConfigDir() == src.ConfigDir():
		fmt.Fprintf(a.Stdout, "this directory already uses the profile at %s\n", src.Root)
		return nil
	case cur.Root == cwd:
		return fmt.Errorf("%s already has its own profile; remove it first with: dclaude deinit", cwd)
	default:
		a.warnf("this link takes precedence over the profile at %s", cur.Root)
	}
	link := filepath.Join(cwd, profile.DirName)
	if exists(link) {
		return fmt.Errorf("%s already exists but is not a dclaude profile", link)
	}

	if err := os.Symlink(src.ConfigDir(), link); err != nil {
		return err
	}
	if !slices.Contains(src.Marker.Links, root) {
		src.Marker.Links = append(src.Marker.Links, root)
		if err := src.Save(); err != nil {
			return err
		}
	}
	s := a.out()
	fmt.Fprintf(a.Stdout, "%s %s -> %s\n", s.ok("linked"), link, src.ConfigDir())
	a.excludeLink(cwd)
	p := &profile.Profile{Root: cwd, Marker: src.Marker}
	if err := a.ensureEnvrc(p); err != nil {
		return err
	}
	switch st, err := a.claudeCLI().AuthStatus(p.ConfigDir()); {
	case err != nil:
		a.warnf("could not check the account: %v", err)
	case st.LoggedIn:
		fmt.Fprintf(a.Stdout, "account: %s\n", describeAccount(st, s))
	default:
		a.warnf("the profile at %s is not logged in; run dclaude login there", src.Root)
	}
	if a.getenv("CLAUDE_CONFIG_DIR") != p.ConfigDir() {
		a.activate(p)
	}
	return nil
}

// lnSource finds the profile to link to. A link names the profile itself, so
// linking to a linked directory reaches the same profile.
func (a *App) lnSource(cwd, root string, args []string) (*profile.Profile, error) {
	var from string
	if len(args) == 1 {
		from = args[0]
		if !filepath.IsAbs(from) {
			from = filepath.Join(cwd, from)
		}
	} else {
		main, err := a.mainCheckout(root)
		if err != nil {
			return nil, err
		}
		from = main
	}
	found, err := profile.Find(from)
	if errors.Is(err, profile.ErrNotFound) {
		return nil, fmt.Errorf("no dclaude profile at or above %s", from)
	}
	if err != nil {
		return nil, err
	}
	if found.Moved() {
		return nil, fmt.Errorf("the profile at %s was moved; run dclaude login there first", found.Root)
	}
	return profile.Find(filepath.Dir(found.ConfigDir()))
}

// mainCheckout maps root, inside a linked git worktree, to the same place in
// the repository's main checkout.
func (a *App) mainCheckout(root string) (string, error) {
	top, err := a.git(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", errNotWorktree
	}
	list, err := a.git(root, "worktree", "list", "--porcelain")
	if err != nil {
		return "", errNotWorktree
	}
	main, ok := strings.CutPrefix(strings.SplitN(list, "\n", 2)[0], "worktree ")
	if !ok || samePath(main, top) {
		return "", errNotWorktree
	}
	rel, err := filepath.Rel(top, root)
	if err != nil {
		return "", err
	}
	return filepath.Join(main, rel), nil
}

// excludeLink keeps the .claude_config link out of git status with a line in
// the repository's info/exclude, which all its worktrees share.
func (a *App) excludeLink(dir string) {
	path, err := a.git(dir, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return // not in a git repository
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		a.warnf("could not read %s: %v", path, err)
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == profile.DirName {
			return
		}
	}
	text := profile.DirName + "\n"
	if len(data) > 0 && data[len(data)-1] != '\n' {
		text = "\n" + text
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		a.warnf("could not update %s: %v", path, err)
		return
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err == nil {
		_, err = f.WriteString(text)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}
	if err != nil {
		a.warnf("could not update %s: %v", path, err)
		return
	}
	fmt.Fprintf(a.Stdout, "added %s to %s\n", profile.DirName, path)
}

// git runs git in dir and returns its trimmed output.
func (a *App) git(dir string, args ...string) (string, error) {
	path, err := a.LookPath("git")
	if err != nil {
		return "", err
	}
	cmd := exec.Command(path, args...)
	cmd.Dir, cmd.Env = dir, a.Env
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
