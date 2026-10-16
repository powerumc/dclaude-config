//go:build !unix

package claude

// LiveSessions is unix-only; dclaude targets macOS and Linux.
func LiveSessions(string) []int { return nil }
