package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/powerumc/dclaude-config/internal/claude"
	"github.com/powerumc/dclaude-config/internal/direnv"
	"github.com/powerumc/dclaude-config/internal/envrc"
	"github.com/powerumc/dclaude-config/internal/keychain"
	"github.com/powerumc/dclaude-config/internal/profile"
	"github.com/powerumc/dclaude-config/internal/usage"
)

func (a *App) cmdInfo(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown option: %s", args[0])
	}
	p, cwd, err := a.currentProfile()
	s := a.out()
	if errors.Is(err, profile.ErrNotFound) {
		a.field("profile", "none; using the global account (~/.claude)")
		if v := a.getenv("CLAUDE_CONFIG_DIR"); v != "" {
			a.warnf("CLAUDE_CONFIG_DIR=%s is set in this shell, so claude uses that directory's login instead", v)
		}
		a.printAccount("", filepath.Join(a.Home, ".claude"), keychain.DefaultOAuthService, "")
		a.warnAuthOverrides(false)
		return nil
	}
	if err != nil {
		return err
	}

	if p.Linked() {
		a.field("profile", p.ConfigDir()+" (linked with dclaude ln)")
	} else {
		a.field("profile", p.ConfigDir())
	}
	a.printSharing(p)
	loads := a.printEnvrcState(p, cwd)
	if p.Moved() {
		// Running claude with the stale path would recreate a profile there.
		a.field("account", s.warn("not checked: the directory moved (recorded path: "+p.ConfigDir()+")")+"; fix with: "+s.cmd("dclaude login"))
		return nil
	}
	active := a.getenv("CLAUDE_CONFIG_DIR") == p.ConfigDir()
	switch h := a.missingHook(); {
	case active:
		a.field("shell", s.ok("active"))
	case h != nil:
		a.field("shell", s.warn("not active: direnv is not hooked into "+h.Shell)+"; add the hook and load it in this shell:")
		for _, c := range h.Commands() {
			a.more(s.cmd(c))
		}
	case loads && a.getenv("DIRENV_DIR") == "":
		if h, ok := direnv.HookFor(a.getenv, a.Home); ok {
			// The startup file has the hook, but this shell started before it was added.
			a.field("shell", s.warn("not active: this shell has not loaded direnv's hook")+"; open a new terminal, or load it here:")
			a.more(s.cmd(h.Line))
		} else {
			a.field("shell", s.warn("not active: this shell has not loaded direnv's hook")+"; set it up: "+direnv.HookDocsURL)
		}
	default:
		a.field("shell", s.warn("not active yet")+" (direnv applies it at the next prompt once the .envrc is allowed)")
	}
	a.printAccount(p.ConfigDir(), p.Dir(), keychain.OAuthService(p.ConfigDir()), "dclaude login")
	a.warnAuthOverrides(active)
	return nil
}

// field prints one "label     value" line of info.
func (a *App) field(label, value string) {
	fmt.Fprintf(a.Stdout, "%s%s\n", a.out().bold(fmt.Sprintf("%-10s", label)), value)
}

// more continues the previous field on a line of its own.
func (a *App) more(value string) {
	fmt.Fprintf(a.Stdout, "%10s%s\n", "", value)
}

// printEnvrcState reports whether direnv loads the profile's block here: the
// .envrc is allowed and no closer .envrc hides it.
func (a *App) printEnvrcState(p *profile.Profile, cwd string) (loads bool) {
	s := a.out()
	path := p.EnvrcPath()
	switch exported, ok := envrc.ExportedConfigDir(path); {
	case !ok:
		a.field(".envrc", s.warn("no dclaude block in "+path)+"; fix with: "+s.cmd("dclaude login"))
	case exported != p.ConfigDir():
		a.field(".envrc", s.warn("exports a different path ("+exported+")")+"; fix with: "+s.cmd("dclaude login"))
	default:
		a.field(".envrc", path)
	}

	dv := a.direnvCLI()
	if !dv.Installed() {
		a.field("direnv", s.bad("not installed")+"; entering this directory does not switch accounts")
		return false
	}
	rc, err := dv.FoundRC(cwd)
	var hiding string
	if err == nil && rc != nil {
		hiding = hidingEnvrc(rc.Path, path)
	}
	switch {
	case err != nil:
		a.field("direnv", s.bad("status check failed")+fmt.Sprintf(": %v", err))
	case rc == nil:
		a.field("direnv", s.warn("no .envrc found"))
	case hiding != "":
		a.field("direnv", s.warn(hiding+" is closer and hides this profile")+"; add "+s.cmd("source_up")+" at the top of it")
	case rc.Allowed == direnv.Allowed:
		a.field("direnv", s.ok("allowed"))
		return true
	case rc.Allowed == direnv.Denied:
		a.field("direnv", s.bad("denied")+"; approve with: "+s.cmd("direnv allow"))
	default:
		a.field("direnv", s.warn("not allowed yet")+"; run: "+s.cmd("direnv allow"))
	}
	return false
}

// printAccount shows who claude would be logged in as, and that login's quota.
func (a *App) printAccount(configDir, configHome, service, loginHint string) {
	s := a.out()
	st, err := a.claudeCLI().AuthStatus(configDir)
	if err != nil {
		a.field("account", s.bad("check failed")+fmt.Sprintf(": %v", err))
		return
	}
	if !st.LoggedIn {
		value := s.warn("not logged in")
		if loginHint != "" {
			value += "; run: " + s.cmd(loginHint)
		}
		a.field("account", value)
		return
	}
	a.field("account", describeAccount(st, s))
	if st.AuthMethod != "claude.ai" {
		a.field("usage", st.AuthMethod+" logins have no subscription usage")
		return
	}

	cred, source, err := claude.ReadCredential(a.Keychain, keychain.Account(a.getenv), service, configHome)
	switch {
	case err != nil:
		a.field("usage", s.warn("unavailable")+fmt.Sprintf(": %v", err))
		return
	case cred == nil || cred.AccessToken == "":
		a.field("usage", s.warn("unavailable")+": no stored token found")
		return
	}
	if source == claude.FromKeychain {
		a.field("storage", "Keychain: "+service)
	} else {
		a.field("storage", "file: "+filepath.Join(configHome, ".credentials.json"))
	}
	if cred.Expired(a.Now()) {
		a.field("usage", s.warn("token expired")+"; claude refreshes it on its next run")
		return
	}
	windows, err := a.Usage.Fetch(cred.AccessToken)
	if err != nil {
		a.field("usage", s.bad("request failed")+fmt.Sprintf(": %v", err))
		return
	}
	lines := usage.Lines(windows, a.Now(), s.level)
	if len(lines) == 0 {
		a.field("usage", "no data")
		return
	}
	a.field("usage", lines[0])
	for _, line := range lines[1:] {
		a.more(line)
	}
}

// warnAuthOverrides flags variables that would make claude ignore the stored login.
func (a *App) warnAuthOverrides(active bool) {
	if active {
		return // the .envrc block unsets them
	}
	var set []string
	for _, k := range claude.AuthOverrideVars {
		if a.getenv(k) != "" {
			set = append(set, k)
		}
	}
	if len(set) > 0 {
		a.warnf("%s is set in this shell, so claude may use it instead of the stored login", strings.Join(set, ", "))
	}
}

// missingHook returns direnv's hook for this shell when direnv is installed
// but neither this shell (no DIRENV_DIR) nor its startup files load it.
func (a *App) missingHook() *direnv.Hook {
	if a.getenv("DIRENV_DIR") != "" || !a.direnvCLI().Installed() {
		return nil
	}
	if h, ok := direnv.HookFor(a.getenv, a.Home); ok && !h.Installed() {
		return h
	}
	return nil
}

// hidingEnvrc follows source_up from rc, the .envrc direnv loads, toward the
// profile's .envrc. It returns the first .envrc that ends the chain before
// reaching the profile, or "" when the profile's block is loaded.
func hidingEnvrc(rc, profileEnvrc string) string {
	for !samePath(rc, profileEnvrc) {
		if !envrc.SourcesUp(rc) {
			return rc
		}
		next := envrcAbove(filepath.Dir(rc))
		if next == "" {
			return rc
		}
		rc = next
	}
	return ""
}

// envrcAbove returns the nearest .envrc above dir, where source_up looks.
func envrcAbove(dir string) string {
	for parent := filepath.Dir(dir); parent != dir; dir, parent = parent, filepath.Dir(parent) {
		if path := filepath.Join(parent, ".envrc"); exists(path) {
			return path
		}
	}
	return ""
}

func samePath(x, y string) bool {
	fx, err1 := os.Stat(x)
	fy, err2 := os.Stat(y)
	if err1 != nil || err2 != nil {
		return x == y
	}
	return os.SameFile(fx, fy)
}
