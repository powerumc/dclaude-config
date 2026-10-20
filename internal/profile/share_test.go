package profile

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// fakeGlobal lays out a ~/.claude with every kind of entry Share sorts.
func fakeGlobal(t *testing.T) string {
	t.Helper()
	global := filepath.Join(t.TempDir(), ".claude")
	for _, d := range []string{"plugins", "skills", "projects", "backups", "sessions", "session-env", "state", "bridge-spawn", ".git"} {
		if err := os.MkdirAll(filepath.Join(global, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{
		"settings.json", "settings.json.bak", "CLAUDE.md", "history.jsonl", "stats-cache.json",
		".credentials.json", ".credentials.lock", "policy-limits.json", "policy-limits.json.stamp.json",
		"remote-settings.json", "mcp-needs-auth-cache.json", ".gitignore", ".DS_Store",
	} {
		if err := os.WriteFile(filepath.Join(global, f), []byte("global "+f), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return global
}

func TestShareableKeepsLoginAccountStateAndHistory(t *testing.T) {
	global := fakeGlobal(t)
	shared := []string{"CLAUDE.md", "plugins", "settings.json", "settings.json.bak", "skills"}
	for _, tc := range []struct {
		s    Sharing
		want []string
	}{
		{Shared, shared},
		{SharedHistory, []string{"CLAUDE.md", "history.jsonl", "plugins", "projects", "settings.json", "settings.json.bak", "skills", "stats-cache.json"}},
		{Isolated, nil},
		{SharingUnset, nil},
	} {
		got, err := Shareable(global, tc.s)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Shareable(%q) = %q, %v; want %q", tc.s, got, err, tc.want)
		}
	}
	if got, err := Shareable(filepath.Join(t.TempDir(), "missing"), Shared); got != nil || err != nil {
		t.Errorf("missing global dir: %q, %v", got, err)
	}
}

func TestShareLinksAndMovesOwnCopiesAside(t *testing.T) {
	global := fakeGlobal(t)
	root := t.TempDir()
	p, err := Create(root, filepath.Join(root, DirName), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Marker.Sharing = Shared
	if err := os.WriteFile(filepath.Join(p.Dir(), "settings.json"), []byte("own"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(p.Dir(), "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	mine := filepath.Join(t.TempDir(), "my-skills")
	if err := os.Symlink(mine, filepath.Join(p.Dir(), "skills")); err != nil { // linked by hand
		t.Fatal(err)
	}

	r, err := p.Share(global, time.Date(2026, 10, 16, 8, 40, 0, 0, time.UTC), nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"CLAUDE.md", "plugins", "settings.json", "settings.json.bak"}; !reflect.DeepEqual(r.Linked, want) {
		t.Errorf("Linked = %q, want %q", r.Linked, want)
	}
	if want := []string{"plugins", "settings.json"}; !reflect.DeepEqual(r.Moved, want) {
		t.Errorf("Moved = %q, want %q", r.Moved, want)
	}
	if got, _ := os.ReadFile(filepath.Join(r.Backup, "settings.json")); string(got) != "own" {
		t.Errorf("backup settings.json = %q", got)
	}
	if target, _ := os.Readlink(filepath.Join(p.Dir(), "settings.json")); target != filepath.Join(global, "settings.json") {
		t.Errorf("settings.json -> %q", target)
	}
	if target, _ := os.Readlink(filepath.Join(p.Dir(), "skills")); target != mine {
		t.Errorf("a hand-made link was replaced: skills -> %q", target)
	}
	for _, n := range []string{"projects", ".credentials.json", ".gitignore"} {
		if info, err := os.Lstat(filepath.Join(p.Dir(), n)); err == nil && info.Mode()&os.ModeSymlink != 0 {
			t.Errorf("%s was linked", n)
		}
	}

	if link, ownCopy, err := p.Pending(global); len(link) != 0 || len(ownCopy) != 0 || err != nil {
		t.Errorf("after Share, Pending = %q, %q, %v", link, ownCopy, err)
	}
}
