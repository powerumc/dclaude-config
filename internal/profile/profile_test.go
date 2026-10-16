package profile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func canonicalTemp(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir()) // macOS: /var -> /private/var
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCreateAndFindFromSubdirectory(t *testing.T) {
	root := canonicalTemp(t)
	cfg := filepath.Join(root, DirName)
	if _, err := Create(root, cfg, time.Now()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(cfg)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("profile dir: %v %v", info, err)
	}
	if data, _ := os.ReadFile(filepath.Join(cfg, ".gitignore")); string(data) != "*\n" {
		t.Errorf(".gitignore = %q", data)
	}
	if !IsProfileDir(cfg) {
		t.Error("IsProfileDir = false")
	}

	sub := filepath.Join(root, "a", "b")
	os.MkdirAll(sub, 0o755)
	p, err := Find(sub)
	if err != nil || p.Root != root || p.ConfigDir() != cfg || p.Moved() {
		t.Fatalf("Find = %+v, %v", p, err)
	}
	if _, err := Find(filepath.Dir(root)); !errors.Is(err, ErrNotFound) {
		t.Errorf("Find above the profile = %v, want ErrNotFound", err)
	}
}

func TestMovedAfterRenameAndCopy(t *testing.T) {
	tmp := canonicalTemp(t)
	a := filepath.Join(tmp, "a")
	os.Mkdir(a, 0o755)
	if _, err := Create(a, filepath.Join(a, DirName), time.Now()); err != nil {
		t.Fatal(err)
	}

	b := filepath.Join(tmp, "b")
	os.Rename(a, b)
	if p, err := Find(b); err != nil || !p.Moved() {
		t.Errorf("renamed profile: moved=%v err=%v", p != nil && p.Moved(), err)
	}

	// A copy whose marker names a profile that still exists is also "moved".
	c := filepath.Join(tmp, "c")
	os.MkdirAll(filepath.Join(c, DirName), 0o700)
	original, _ := Find(b)
	original.Marker.ConfigDir = filepath.Join(b, DirName)
	original.Save()
	data, _ := os.ReadFile(original.markerPath())
	os.WriteFile(filepath.Join(c, DirName, MarkerName), data, 0o600)
	if p, err := Find(c); err != nil || !p.Moved() {
		t.Errorf("copied profile not reported as moved: err=%v", err)
	}
	if original.Moved() {
		t.Error("original reported as moved")
	}
}

func TestCheckInitTarget(t *testing.T) {
	home := "/Users/u"
	refused := []string{"/", home, home + "/.claude", home + "/.claude/x", home + "/p/.claude_config", home + "/p/.claude_config/x"}
	for _, dir := range refused {
		if CheckInitTarget(dir, home) == nil {
			t.Errorf("%s accepted", dir)
		}
	}
	for _, dir := range []string{home + "/work/a", home + "/.claudefoo", "/srv/project"} {
		if err := CheckInitTarget(dir, home); err != nil {
			t.Errorf("%s refused: %v", dir, err)
		}
	}
}

func TestLinkedProfileIsTheSourceProfile(t *testing.T) {
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(tmp, "src")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := Create(src, filepath.Join(src, DirName), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(tmp, "wt", "sub")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(p.Dir(), filepath.Join(tmp, "wt", DirName)); err != nil {
		t.Fatal(err)
	}

	got, err := Find(wt)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Linked() || p.Linked() || got.ConfigDir() != p.ConfigDir() || got.Moved() {
		t.Errorf("linked=%v source linked=%v configDir=%q moved=%v", got.Linked(), p.Linked(), got.ConfigDir(), got.Moved())
	}
}
