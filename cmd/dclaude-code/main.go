// Command dclaude-code runs Claude Code with the direnv environment of the
// current directory, for apps that start claude without a shell. It passes
// every argument to claude.
package main

import (
	"os"

	"github.com/powerumc/dclaude-config/internal/code"
)

func main() {
	os.Exit(code.New().Run(os.Args[1:]))
}
