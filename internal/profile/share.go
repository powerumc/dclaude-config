package profile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

// Sharing records how a profile uses the global Claude Code setup in ~/.claude.
type Sharing string

const (
	// SharingUnset marks profiles that never chose: made before sharing
	// existed, or when ~/.claude did not exist yet.
	SharingUnset Sharing = ""
	// Isolated profiles link nothing.
	Isolated Sharing = "isolated"
	// Shared profiles link everything but the login, account state and
	// conversation history.
	Shared Sharing = "shared"
	// SharedHistory profiles also link the conversation history.
	SharedHistory Sharing = "shared-history"
)

// BackupDirName holds a profile's own entries moved aside to make room for links.
const BackupDirName = ".dclaude-backup"

// own lists what every profile keeps for itself: the login and account state
// Claude Code writes for the account, the state of running sessions, and
// dclaude's files. A name also covers names that extend it with a dot.
var own = []string{
	".credentials", ".claude.json", "policy-limits.json", "remote-settings.json", "mcp-needs-auth-cache.json",
	"backups", "bridge-spawn", "sessions", "session-env", "state",
	MarkerName, BackupDirName, ".gitignore", ".git", ".DS_Store",
}

// history is the conversation history, linked only for SharedHistory: Claude
// Code asks before every auto memory read or write through a symlinked
// projects/ (anthropics/claude-code#98044).
var history = []string{"projects", "history.jsonl", "stats-cache.json"}

// conversations is the part of the history a profile must not lose: the
// sessions with their auto memory, and the prompt history. Claude Code rebuilds
// stats-cache.json.
var conversations = []string{"projects", "history.jsonl"}

// IsConversation reports whether name holds conversations or prompt history.
func IsConversation(name string) bool { return matches(name, conversations) }

// OwnHistory lists the conversations the profile keeps itself rather than
// links: linking ~/.claude's would move them aside.
func (p *Profile) OwnHistory() ([]string, error) {
	var own []string
	for _, n := range conversations {
		info, err := os.Lstat(filepath.Join(p.Dir(), n))
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return nil, err
		case info.Mode()&fs.ModeSymlink == 0:
			own = append(own, n)
		}
	}
	return own, nil
}

// EnsureHistory creates an empty projects/ and history.jsonl in global where
// they are missing, as Claude Code itself would, so that a profile sharing the
// history links them from the start instead of writing a history of its own.
func EnsureHistory(global string) error {
	if err := os.MkdirAll(filepath.Join(global, "projects"), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(global, "history.jsonl"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	return f.Close()
}

func matches(name string, list []string) bool {
	for _, n := range list {
		if name == n || strings.HasPrefix(name, n+".") {
			return true
		}
	}
	return false
}

// Shareable lists the entries of the global config directory that a profile
// links under s, sorted. A missing global directory has none.
func Shareable(global string, s Sharing) ([]string, error) {
	if s != Shared && s != SharedHistory {
		return nil, nil
	}
	entries, err := os.ReadDir(global)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if n := e.Name(); !matches(n, own) && (s == SharedHistory || !matches(n, history)) {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names, nil
}

// Pending lists the shareable entries the profile does not link yet, and which
// of them the profile has a file or directory of its own for. Any symlink
// counts as linked, so links made by hand are left alone.
func (p *Profile) Pending(global string) (link, ownCopy []string, err error) {
	names, err := Shareable(global, p.Marker.Sharing)
	if err != nil {
		return nil, nil, err
	}
	for _, n := range names {
		info, err := os.Lstat(filepath.Join(p.Dir(), n))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			link = append(link, n)
		case err != nil:
			return nil, nil, err
		case info.Mode()&fs.ModeSymlink == 0:
			link = append(link, n)
			ownCopy = append(ownCopy, n)
		}
	}
	return link, ownCopy, nil
}

// ShareResult reports what Share changed.
type ShareResult struct {
	Linked []string // entries linked to the global directory
	Moved  []string // the profile's own copies moved to Backup first
	Backup string
}

// Share links every pending entry to the same name in global, except those in
// skip. The profile's own copy of an entry is moved to a backup directory,
// never deleted.
func (p *Profile) Share(global string, now time.Time, skip []string) (ShareResult, error) {
	link, ownCopy, err := p.Pending(global)
	if err != nil {
		return ShareResult{}, err
	}
	link = slices.DeleteFunc(link, func(n string) bool { return slices.Contains(skip, n) })
	ownCopy = slices.DeleteFunc(ownCopy, func(n string) bool { return slices.Contains(skip, n) })
	r := ShareResult{}
	if len(ownCopy) > 0 {
		r.Backup = filepath.Join(p.Dir(), BackupDirName, now.Format("20060102-150405"))
		if err := os.MkdirAll(r.Backup, 0o700); err != nil {
			return r, err
		}
		for _, n := range ownCopy {
			if err := os.Rename(filepath.Join(p.Dir(), n), filepath.Join(r.Backup, n)); err != nil {
				return r, err
			}
			r.Moved = append(r.Moved, n)
		}
	}
	for _, n := range link {
		if err := os.Symlink(filepath.Join(global, n), filepath.Join(p.Dir(), n)); err != nil {
			return r, err
		}
		r.Linked = append(r.Linked, n)
	}
	return r, nil
}
