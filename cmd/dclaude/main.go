// Command dclaude gives a directory, and everything below it, its own Claude
// Code login: a .claude_config profile selected through direnv.
package main

import (
	"os"
	"runtime/debug"
	"strings"

	"github.com/powerumc/dclaude-config/internal/cli"
)

// version is set at build time with -ldflags "-X main.version=...", as the
// release builds and the Homebrew formula do.
var version string

func main() {
	os.Exit(cli.New(buildVersion()).Run(os.Args[1:]))
}

// buildVersion falls back to the module version Go records in the binary:
// the tag for go install, or a git-derived pseudo-version for go build in a
// clone. The leading v is dropped to match the release builds.
func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return strings.TrimPrefix(info.Main.Version, "v")
	}
	return "dev"
}
