// Command dclaude gives a directory, and everything below it, its own Claude
// Code login: a .claude_config profile selected through direnv.
package main

import (
	"os"

	"github.com/powerumc/dclaude-config/internal/cli"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "0.1.0-prototype"

func main() {
	os.Exit(cli.New(version).Run(os.Args[1:]))
}
