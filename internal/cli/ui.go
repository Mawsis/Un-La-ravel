package cli

// This file is the CLI's shared presentation layer (ADR 0008, design.md
// "Part 1 — CLI"): the lipgloss styles every subcommand renders with, and the
// TTY-detection seam that decides whether a command emits a styled table
// (interactive terminal), the plain golden-pinned renderer output (piped), or
// leaves stdout alone for --json.
//
// Nothing here touches the Project Model or does analysis — it is pure
// presentation, colocated with the commands it styles rather than with
// internal/render (whose output stays contract-pinned and terminal-agnostic).

import (
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

// isTTY reports whether w is an interactive terminal. It is a var, not a
// plain func, so tests can substitute a fixed answer without a real pty —
// styled output is deliberately never golden-pinned (ADR 0008), but commands
// still need deterministic tests of *which* mode they chose.
var isTTY = func(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// Shared style palette, mirroring design.md's web tokens (ANSI-mapped): cyan
// for headers/hints, green for healthy state, amber for findings, red
// reserved for dead routes and errors, dim for secondary detail.
var (
	styleHeading = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14")) // cyan
	styleGood    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))            // green
	styleWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))            // amber
	styleDanger  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))             // red
	styleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))           // gray
	styleBold    = lipgloss.NewStyle().Bold(true)

	// styleMethod maps an HTTP verb to its table color, matching the web
	// dashboard's route method colors (design.md component inventory).
	styleMethodGET    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10")) // green
	styleMethodPOST   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")) // blue
	styleMethodPUT    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")) // amber
	styleMethodDELETE = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))  // red
)

// methodStyle returns the style for an HTTP method, falling back to bold
// plain text for any verb outside the four styled above.
func methodStyle(method string) lipgloss.Style {
	switch method {
	case "GET":
		return styleMethodGET
	case "POST":
		return styleMethodPOST
	case "PUT", "PATCH":
		return styleMethodPUT
	case "DELETE":
		return styleMethodDELETE
	default:
		return styleBold
	}
}
