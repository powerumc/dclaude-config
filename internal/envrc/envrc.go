// Package envrc maintains dclaude's managed block inside a direnv .envrc and
// leaves every other line of the file alone.
package envrc

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// Markers delimiting the managed block.
const (
	BlockBegin = "# >>> dclaude >>>"
	BlockEnd   = "# <<< dclaude <<<"
)

// Block exports configDir verbatim and unsets the given variables. configDir is
// the literal recorded at init: Claude Code keys its Keychain entry by this
// exact string, so the shell never recomputes it.
func Block(configDir string, unset []string) string {
	q := quote(configDir)
	return strings.Join([]string{
		BlockBegin,
		"# Managed by dclaude. Do not edit this block.",
		"if [[ -d " + q + " ]]; then",
		"  export CLAUDE_CONFIG_DIR=" + q,
		"  unset " + strings.Join(unset, " "),
		"else",
		"  log_error " + quote("dclaude: profile not found: "+configDir+" (if the directory moved, run: dclaude login)"),
		"fi",
		BlockEnd,
	}, "\n") + "\n"
}

// find returns the byte range of the managed block, or -1, -1 when absent.
func find(content string) (start, end int, err error) {
	start, off := -1, 0
	for _, line := range strings.SplitAfter(content, "\n") {
		switch strings.TrimSpace(line) {
		case BlockBegin:
			if start >= 0 {
				return -1, -1, errors.New("dclaude block begins twice")
			}
			start = off
		case BlockEnd:
			if start < 0 {
				return -1, -1, errors.New("dclaude block end marker without a begin marker")
			}
			return start, off + len(line), nil
		}
		off += len(line)
	}
	if start >= 0 {
		return -1, -1, errors.New("dclaude block has no end marker")
	}
	return -1, -1, nil
}

// Upsert replaces an existing block in place, or appends block after a blank line.
func Upsert(content, block string) (string, error) {
	start, end, err := find(content)
	if err != nil {
		return "", err
	}
	if start >= 0 {
		return content[:start] + block + content[end:], nil
	}
	if content == "" {
		return block, nil
	}
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + "\n" + block, nil
}

// Remove deletes the block and the blank separator Upsert put before it.
func Remove(content string) (rest string, found bool, err error) {
	start, end, err := find(content)
	if err != nil || start < 0 {
		return content, false, err
	}
	before, after := content[:start], content[end:]
	if strings.HasSuffix(before, "\n\n") {
		before = before[:len(before)-1]
	}
	return before + after, true, nil
}

// InstallResult describes what Install found.
type InstallResult struct {
	Created  bool // the file did not exist before
	HadOther bool // the file holds content dclaude did not write
}

// Install writes block into the .envrc at path, creating the file if needed.
func Install(path, block string) (InstallResult, error) {
	data, err := os.ReadFile(path)
	created := errors.Is(err, fs.ErrNotExist)
	if err != nil && !created {
		return InstallResult{}, err
	}
	content := string(data)
	rest, _, err := Remove(content)
	if err != nil {
		return InstallResult{}, fmt.Errorf("%s: %w", path, err)
	}
	updated, err := Upsert(content, block)
	if err != nil {
		return InstallResult{}, err
	}
	mode := fs.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(path, []byte(updated), mode); err != nil {
		return InstallResult{}, err
	}
	return InstallResult{Created: created, HadOther: !isBlank(rest)}, nil
}

// UninstallResult describes what Uninstall changed.
type UninstallResult struct {
	Found       bool // a managed block was removed
	FileRemoved bool // nothing else was left, so the file was deleted
}

// Uninstall removes the block from the .envrc at path, and the file itself
// when nothing else is left.
func Uninstall(path string) (UninstallResult, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return UninstallResult{}, nil
	}
	if err != nil {
		return UninstallResult{}, err
	}
	rest, found, err := Remove(string(data))
	if err != nil {
		return UninstallResult{}, fmt.Errorf("%s: %w", path, err)
	}
	if !found {
		return UninstallResult{}, nil
	}
	if isBlank(rest) {
		return UninstallResult{Found: true, FileRemoved: true}, os.Remove(path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return UninstallResult{}, err
	}
	return UninstallResult{Found: true}, os.WriteFile(path, []byte(rest), info.Mode().Perm())
}

// ExportedConfigDir reports the CLAUDE_CONFIG_DIR the block in path exports.
func ExportedConfigDir(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	start, end, err := find(string(data))
	if err != nil || start < 0 {
		return "", false
	}
	for _, line := range strings.Split(string(data)[start:end], "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "export CLAUDE_CONFIG_DIR="); ok {
			return unquote(value), true
		}
	}
	return "", false
}

// HasOtherContent reports whether the .envrc at path holds anything besides
// the managed block.
func HasOtherContent(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	rest, _, err := Remove(string(data))
	return err == nil && !isBlank(rest)
}

// SourcesUp reports whether the .envrc at path loads the next .envrc above it
// with source_up or source_up_if_exists.
func SourcesUp(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || (fields[0] != "source_up" && fields[0] != "source_up_if_exists") {
			continue
		}
		if len(fields) == 1 || fields[1] == ".envrc" {
			return true
		}
	}
	return false
}

func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// unquote reverses quote.
func unquote(s string) string {
	s = strings.TrimSuffix(strings.TrimPrefix(s, "'"), "'")
	return strings.ReplaceAll(s, `'\''`, "'")
}

func isBlank(s string) bool { return strings.TrimSpace(s) == "" }
