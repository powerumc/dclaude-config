package envrc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var testUnset = []string{"ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN"}

func TestUpsertAndRemoveRoundTrip(t *testing.T) {
	original := "export FOO=1\n"
	block := Block("/x/.claude_config", testUnset)
	added, err := Upsert(original, block)
	if err != nil {
		t.Fatal(err)
	}
	if want := original + "\n" + block; added != want {
		t.Fatalf("Upsert:\n%s\nwant:\n%s", added, want)
	}
	removed, found, err := Remove(added)
	if err != nil || !found {
		t.Fatalf("Remove: found=%v err=%v", found, err)
	}
	if removed != original {
		t.Fatalf("Remove = %q, want %q", removed, original)
	}
}

func TestUpsertReplacesBlockInPlace(t *testing.T) {
	first, _ := Upsert("a\n", Block("/old/.claude_config", testUnset))
	second, err := Upsert(first+"b\n", Block("/new/.claude_config", testUnset))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(second, BlockBegin) != 1 || strings.Contains(second, "/old/") {
		t.Fatalf("block not replaced:\n%s", second)
	}
	if !strings.HasPrefix(second, "a\n") || !strings.HasSuffix(second, "b\n") {
		t.Fatalf("surrounding content changed:\n%s", second)
	}
}

func TestFindRejectsBrokenMarkers(t *testing.T) {
	for _, content := range []string{
		BlockBegin + "\nx\n",
		"x\n" + BlockEnd + "\n",
		BlockBegin + "\n" + BlockBegin + "\n" + BlockEnd + "\n",
	} {
		if _, _, err := find(content); err == nil {
			t.Errorf("no error for %q", content)
		}
	}
}

func TestBlockExportsExactPathInBash(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	dir := filepath.Join(t.TempDir(), "it's a dir", ".claude_config")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	rc := filepath.Join(t.TempDir(), "envrc")
	if err := os.WriteFile(rc, []byte(Block(dir, testUnset)), 0o644); err != nil {
		t.Fatal(err)
	}
	script := `log_error() { echo "LOGERR:$*" >&2; }; export ANTHROPIC_API_KEY=x; source "$1"; printf '%s|%s' "${CLAUDE_CONFIG_DIR-}" "${ANTHROPIC_API_KEY-unset}"`
	bash := func() *exec.Cmd {
		cmd := exec.Command("bash", "-c", script, "_", rc)
		cmd.Env = []string{"PATH=" + os.Getenv("PATH")} // never inherit a real CLAUDE_CONFIG_DIR
		return cmd
	}

	out, err := bash().Output()
	if err != nil {
		t.Fatal(err)
	}
	if want := dir + "|unset"; string(out) != want {
		t.Fatalf("got %q, want %q", out, want)
	}

	// A moved profile must not export a path that no longer exists.
	os.RemoveAll(dir)
	cmd := bash()
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err = cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "|x" || !strings.Contains(stderr.String(), "LOGERR:dclaude:") {
		t.Fatalf("missing dir: out=%q stderr=%q", out, stderr.String())
	}
}

func TestInstallUninstallLifecycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".envrc")
	cfg := filepath.Join(dir, "quote'd", ".claude_config")
	block := Block(cfg, testUnset)

	r, err := Install(path, block)
	if err != nil || !r.Created || r.HadOther {
		t.Fatalf("new file: %+v err=%v", r, err)
	}
	if got, ok := ExportedConfigDir(path); !ok || got != cfg {
		t.Fatalf("ExportedConfigDir = %q, %v", got, ok)
	}
	if r, _ := Install(path, block); r.Created || r.HadOther {
		t.Fatalf("rewrite of own block: %+v", r)
	}
	u, err := Uninstall(path)
	if err != nil || !u.FileRemoved {
		t.Fatalf("uninstall own file: %+v err=%v", u, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal(".envrc should be gone")
	}

	os.WriteFile(path, []byte("export FOO=1\n"), 0o600)
	if r, _ := Install(path, block); r.Created || !r.HadOther {
		t.Fatalf("foreign file: %+v", r)
	}
	if u, _ := Uninstall(path); !u.Found || u.FileRemoved {
		t.Fatalf("uninstall from foreign file: %+v", u)
	}
	data, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	if string(data) != "export FOO=1\n" || info.Mode().Perm() != 0o600 {
		t.Fatalf("foreign content/mode not preserved: %q %v", data, info.Mode().Perm())
	}
}

func TestSourcesUp(t *testing.T) {
	for content, want := range map[string]bool{
		"source_up\n":                      true,
		"export X=1\n  source_up .envrc\n": true,
		"source_up_if_exists\n":            true,
		"export X=1\n":                     false,
		"# source_up\n":                    false,
		"echo source_up\n":                 false,
		"source_up .envrc.local\n":         false,
	} {
		path := filepath.Join(t.TempDir(), ".envrc")
		os.WriteFile(path, []byte(content), 0o644)
		if got := SourcesUp(path); got != want {
			t.Errorf("SourcesUp(%q) = %v, want %v", content, got, want)
		}
	}
	if SourcesUp(filepath.Join(t.TempDir(), "missing")) {
		t.Error("SourcesUp of a missing file")
	}
}

func TestHasOtherContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".envrc")
	block := Block("/x/.claude_config", testUnset)
	for content, want := range map[string]bool{
		block:                      false,
		"\n" + block + "\n":        false,
		"export FOO=1\n\n" + block: true,
	} {
		os.WriteFile(path, []byte(content), 0o644)
		if got := HasOtherContent(path); got != want {
			t.Errorf("HasOtherContent(%q) = %v, want %v", content, got, want)
		}
	}
}
