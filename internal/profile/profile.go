// Package profile manages dclaude profiles: a .claude_config directory that
// becomes CLAUDE_CONFIG_DIR for the directory tree containing it.
package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Names of the profile directory and the marker file inside it.
const (
	DirName    = ".claude_config"
	MarkerName = ".dclaude"
)

// ErrNotFound is returned by Find when no profile covers the directory.
var ErrNotFound = errors.New("no dclaude profile found")

// Marker identifies a dclaude-managed profile and records the CLAUDE_CONFIG_DIR
// string chosen at init: the one value the shell, dclaude and Claude Code share.
type Marker struct {
	Version   int       `json:"version"`
	ConfigDir string    `json:"configDir"`
	Created   time.Time `json:"created"`
	Sharing   Sharing   `json:"sharing,omitempty"`
	Links     []string  `json:"links,omitempty"` // directories `dclaude ln` linked to this profile
}

// Profile is a profile as reached from the current directory.
type Profile struct {
	Root   string // directory holding .claude_config and .envrc, as reached from the cwd
	Marker Marker
}

// ConfigDir is the recorded CLAUDE_CONFIG_DIR value.
func (p *Profile) ConfigDir() string { return p.Marker.ConfigDir }

// Dir is the .claude_config directory on disk.
func (p *Profile) Dir() string { return filepath.Join(p.Root, DirName) }

// EnvrcPath is the .envrc that activates the profile.
func (p *Profile) EnvrcPath() string { return filepath.Join(p.Root, ".envrc") }

func (p *Profile) markerPath() string { return filepath.Join(p.Dir(), MarkerName) }

// Linked reports whether the profile is reached through a .claude_config
// symlink made by `dclaude ln`, rather than living here.
func (p *Profile) Linked() bool {
	info, err := os.Lstat(p.Dir())
	return err == nil && info.Mode()&fs.ModeSymlink != 0
}

// Moved reports whether the recorded path no longer names this directory,
// because the directory was moved or copied.
func (p *Profile) Moved() bool {
	recorded, err1 := os.Stat(p.ConfigDir())
	actual, err2 := os.Stat(p.Dir())
	return err1 != nil || err2 != nil || !os.SameFile(recorded, actual)
}

// Save rewrites the marker, e.g. after re-pointing a moved profile.
func (p *Profile) Save() error {
	data, err := json.MarshalIndent(p.Marker, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p.markerPath(), append(data, '\n'), 0o600)
}

// Create makes root/.claude_config (mode 0700) with a self-ignoring
// .gitignore and a marker recording configDir as its CLAUDE_CONFIG_DIR.
func Create(root, configDir string, now time.Time) (*Profile, error) {
	p := &Profile{Root: root, Marker: Marker{Version: 1, ConfigDir: configDir, Created: now.UTC()}}
	dir := p.Dir()
	if err := os.Mkdir(dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0o700); err != nil { // independent of umask
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*\n"), 0o644); err != nil {
		return nil, err
	}
	if err := p.Save(); err != nil {
		return nil, err
	}
	return p, nil
}

// Find walks up from start (the logical cwd, as direnv does) to the nearest
// profile, returning ErrNotFound when there is none.
func Find(start string) (*Profile, error) {
	for dir := start; ; dir = filepath.Dir(dir) {
		m, err := readMarker(filepath.Join(dir, DirName, MarkerName))
		if err == nil {
			return &Profile{Root: dir, Marker: m}, nil
		}
		if !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, fs.ErrPermission) {
			return nil, err
		}
		if filepath.Dir(dir) == dir {
			return nil, ErrNotFound
		}
	}
}

// IsProfileDir reports whether dir is a .claude_config created by dclaude.
func IsProfileDir(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, MarkerName))
	return err == nil
}

// Canonical resolves symlinks, so every route into a directory yields one string.
func Canonical(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// CheckInitTarget refuses directories where a profile would cover far more
// than intended. root and home must be canonical.
func CheckInitTarget(root, home string) error {
	sep := string(filepath.Separator)
	switch {
	case root == sep:
		return errors.New("refusing to create a profile in the root directory")
	case root == home:
		return errors.New("refusing to create a profile in the home directory (it would apply to everything under it)")
	case isWithin(root, filepath.Join(home, ".claude")):
		return errors.New("refusing to create a profile inside ~/.claude")
	case filepath.Base(root) == DirName || strings.Contains(root, sep+DirName+sep):
		return errors.New("refusing to create a profile inside a .claude_config directory")
	}
	return nil
}

func isWithin(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func readMarker(path string) (Marker, error) {
	var m Marker
	data, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("%s: %w", path, err)
	}
	if m.ConfigDir == "" {
		return m, fmt.Errorf("%s: configDir is empty", path)
	}
	return m, nil
}
