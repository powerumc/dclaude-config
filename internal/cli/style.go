package cli

import (
	"io"
	"os"
)

// style colors text with ANSI escapes; the zero value leaves it plain.
type style struct{ on bool }

func (s style) paint(code, text string) string {
	if !s.on || text == "" {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func (s style) bold(text string) string { return s.paint("1", text) }
func (s style) ok(text string) string   { return s.paint("32", text) } // green: done, working
func (s style) warn(text string) string { return s.paint("33", text) } // yellow: needs a step from the user
func (s style) bad(text string) string  { return s.paint("31", text) } // red: failed or blocked
func (s style) cmd(text string) string  { return s.paint("36", text) } // cyan: a command to run

// level colors a usage percentage by how much of the window is used.
func (s style) level(utilization float64, text string) string {
	switch {
	case utilization >= 80:
		return s.bad(text)
	case utilization >= 50:
		return s.warn(text)
	}
	return s.ok(text)
}

// styleFor colors output to w only when w is a terminal. NO_COLOR turns
// colors off, CLICOLOR_FORCE turns them on even through a pipe.
func (a *App) styleFor(w io.Writer) style {
	if a.getenv("NO_COLOR") != "" {
		return style{}
	}
	if v := a.getenv("CLICOLOR_FORCE"); v != "" && v != "0" {
		return style{on: true}
	}
	f, ok := w.(*os.File)
	if !ok || a.getenv("TERM") == "dumb" {
		return style{}
	}
	info, err := f.Stat()
	return style{on: err == nil && info.Mode()&os.ModeCharDevice != 0}
}

func (a *App) out() style  { return a.styleFor(a.Stdout) }
func (a *App) errs() style { return a.styleFor(a.Stderr) }
