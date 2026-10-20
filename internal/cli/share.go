package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/powerumc/dclaude-config/internal/claude"
	"github.com/powerumc/dclaude-config/internal/profile"
)

// splitShareFlags takes the share options out of args; the rest goes to
// claude auth login.
func splitShareFlags(args []string) (choice profile.Sharing, rest []string, err error) {
	var given string
	for i, arg := range args {
		var c profile.Sharing
		switch arg {
		case "--":
			return choice, append(rest, args[i:]...), nil
		case "--isolated":
			c = profile.Isolated
		case "--share-history":
			c = profile.SharedHistory
		case "--no-share-history":
			c = profile.Shared
		default:
			rest = append(rest, arg)
			continue
		}
		if choice != profile.SharingUnset && choice != c {
			return "", nil, fmt.Errorf("%s and %s cannot be used together", given, arg)
		}
		choice, given = c, arg
	}
	return choice, rest, nil
}

// globalDir is the global Claude Code config directory profiles share.
func (a *App) globalDir() string { return filepath.Join(a.Home, ".claude") }

// chooseSharing records how p uses ~/.claude: choice when given, otherwise the
// user's answers. Nothing is asked while ~/.claude does not exist. The history
// is offered only to a profile without one of its own: linking ~/.claude's
// would move that history aside.
func (a *App) chooseSharing(p *profile.Profile, choice profile.Sharing) error {
	if choice == profile.SharingUnset {
		if !exists(a.globalDir()) {
			return nil
		}
		own, err := p.OwnHistory()
		if err != nil {
			return err
		}
		offerHistory := len(own) == 0
		fmt.Fprintln(a.Stdout, "This profile can use your global Claude Code setup in ~/.claude (settings, CLAUDE.md, plugins, skills and more) through symlinks.")
		if offerHistory {
			fmt.Fprintln(a.Stdout, "The login and account state stay separate.")
		} else {
			fmt.Fprintln(a.Stdout, "The login, account state and conversation history stay separate.")
		}
		choice = profile.Isolated
		if a.ask("Share ~/.claude with this profile?", true) {
			choice = profile.Shared
			if offerHistory && a.askHistory() {
				choice = profile.SharedHistory
			}
		}
	}
	p.Marker.Sharing = choice
	return p.Save()
}

// askHistory tells what sharing the conversation history brings and costs, then asks.
func (a *App) askHistory() bool {
	fmt.Fprintln(a.Stdout, "The conversation history (projects/, history.jsonl) can be shared too, so this profile and ~/.claude see the same sessions and auto memory.")
	fmt.Fprintln(a.Stdout, a.out().warn("Claude Code then asks before every auto memory read or write, and allow rules cannot approve it (anthropics/claude-code#98044)."))
	return a.ask("Share the conversation history too?", true)
}

// syncSharing links what ~/.claude has and a sharing profile does not link yet.
func (a *App) syncSharing(p *profile.Profile) error {
	if p.Marker.Sharing != profile.Shared && p.Marker.Sharing != profile.SharedHistory {
		return nil
	}
	if p.Marker.Sharing == profile.SharedHistory {
		if err := profile.EnsureHistory(a.globalDir()); err != nil {
			return err
		}
	}
	_, ownCopy, err := p.Pending(a.globalDir())
	if err != nil {
		return err
	}
	if pids := claude.LiveSessions(p.Dir()); len(ownCopy) > 0 && len(pids) > 0 {
		a.warnf("not linking ~/.claude yet: a Claude session is using this profile (PID %v); close it, then run: dclaude login", pids)
		return nil
	}
	r, err := p.Share(a.globalDir(), a.Now(), a.keepOwnHistory(p, ownCopy))
	if len(r.Moved) > 0 {
		fmt.Fprintf(a.Stdout, "moved this profile's own %s to %s\n", strings.Join(r.Moved, ", "), r.Backup)
	}
	if len(r.Linked) > 0 {
		fmt.Fprintf(a.Stdout, "%s %d entries of ~/.claude into this profile (%s)\n", a.out().ok("linked"), len(r.Linked), sharingText(p.Marker.Sharing))
	}
	return err
}

// keepOwnHistory asks before the profile's own conversations are moved aside
// for those in ~/.claude, and returns them when the answer is no.
func (a *App) keepOwnHistory(p *profile.Profile, ownCopy []string) []string {
	var own []string
	for _, n := range ownCopy {
		if profile.IsConversation(n) {
			own = append(own, n)
		}
	}
	if len(own) == 0 {
		return nil
	}
	fmt.Fprintf(a.Stdout, "%s; sharing the history in ~/.claude moves it to %s.\n", a.out().warn("this profile has a conversation history of its own ("+strings.Join(own, ", ")+")"), filepath.Join(p.Dir(), profile.BackupDirName))
	if a.confirm("Move it aside and use the history in ~/.claude?") {
		return nil
	}
	fmt.Fprintf(a.Stdout, "kept this profile's own %s; dclaude login asks again\n", strings.Join(own, ", "))
	return own
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
	if link, ownCopy, err := p.Pending(a.globalDir()); err == nil {
		var kept []string
		for _, n := range ownCopy {
			if profile.IsConversation(n) {
				kept = append(kept, n)
			}
		}
		if n := len(link) - len(kept); n > 0 {
			value += "; " + s.warn(fmt.Sprintf("%d newer entries not linked yet", n)) + " (dclaude login links them)"
		}
		if len(kept) > 0 {
			value += "; " + s.warn("keeps its own "+strings.Join(kept, ", ")) + " (dclaude login asks to move it aside)"
		}
	}
	a.field("shared", value)
}
