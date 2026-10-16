package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/powerumc/dclaude-config/internal/claude"
	"github.com/powerumc/dclaude-config/internal/profile"
)

// splitShareFlags takes --isolated and --share-history out of args; the rest
// goes to claude auth login.
func splitShareFlags(args []string) (choice profile.Sharing, rest []string, err error) {
	for i, arg := range args {
		var c profile.Sharing
		switch arg {
		case "--":
			return choice, append(rest, args[i:]...), nil
		case "--isolated":
			c = profile.Isolated
		case "--share-history":
			c = profile.SharedHistory
		default:
			rest = append(rest, arg)
			continue
		}
		if choice != profile.SharingUnset && choice != c {
			return "", nil, errors.New("--isolated and --share-history cannot be used together")
		}
		choice = c
	}
	return choice, rest, nil
}

// globalDir is the global Claude Code config directory profiles share.
func (a *App) globalDir() string { return filepath.Join(a.Home, ".claude") }

// chooseSharing records how p uses ~/.claude: choice when given, otherwise the
// user's answer. Nothing is asked while ~/.claude does not exist.
func (a *App) chooseSharing(p *profile.Profile, choice profile.Sharing) error {
	if choice == profile.SharingUnset {
		if !exists(a.globalDir()) {
			return nil
		}
		fmt.Fprintln(a.Stdout, "This profile can use your global Claude Code setup in ~/.claude (settings, CLAUDE.md, plugins, skills and more) through symlinks.")
		fmt.Fprintln(a.Stdout, "The login, account state and conversation history stay separate.")
		choice = profile.Isolated
		if a.ask("Share ~/.claude with this profile?", true) {
			choice = profile.Shared
		}
	}
	p.Marker.Sharing = choice
	return p.Save()
}

// syncSharing links what ~/.claude has and a sharing profile does not link yet.
func (a *App) syncSharing(p *profile.Profile) error {
	if p.Marker.Sharing != profile.Shared && p.Marker.Sharing != profile.SharedHistory {
		return nil
	}
	_, ownCopy, err := p.Pending(a.globalDir())
	if err != nil {
		return err
	}
	if pids := claude.LiveSessions(p.Dir()); len(ownCopy) > 0 && len(pids) > 0 {
		a.warnf("not linking ~/.claude yet: a Claude session is using this profile (PID %v); close it, then run: dclaude login", pids)
		return nil
	}
	r, err := p.Share(a.globalDir(), a.Now())
	if len(r.Moved) > 0 {
		fmt.Fprintf(a.Stdout, "moved this profile's own %s to %s\n", strings.Join(r.Moved, ", "), r.Backup)
	}
	if len(r.Linked) > 0 {
		fmt.Fprintf(a.Stdout, "%s %d entries of ~/.claude into this profile (%s)\n", a.out().ok("linked"), len(r.Linked), sharingText(p.Marker.Sharing))
	}
	return err
}

func sharingText(s profile.Sharing) string {
	if s == profile.SharedHistory {
		return "everything but the login and account state"
	}
	return "everything but the login, account state and conversation history"
}

func (a *App) printSharing(p *profile.Profile) {
	s := a.out()
	switch p.Marker.Sharing {
	case profile.Isolated:
		a.field("shared", "nothing; this profile is isolated from ~/.claude")
		return
	case profile.SharingUnset:
		if !exists(a.globalDir()) {
			a.field("shared", "nothing; ~/.claude does not exist")
		} else {
			a.field("shared", s.warn("not chosen yet")+"; dclaude login asks whether to share ~/.claude")
		}
		return
	}
	value := "~/.claude: " + sharingText(p.Marker.Sharing)
	if link, _, err := p.Pending(a.globalDir()); err == nil && len(link) > 0 {
		value += "; " + s.warn(fmt.Sprintf("%d newer entries not linked yet", len(link))) + " (dclaude login links them)"
	}
	a.field("shared", value)
}
