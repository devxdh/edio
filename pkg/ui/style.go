package ui

import (
	"fmt"
	"os"
)

var colorEnabled = checkColorSupport()

func checkColorSupport() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

const (
	reset     = "\033[0m"
	bold      = "\033[1m"
	dim       = "\033[2m"
	red       = "\033[31m"
	green     = "\033[32m"
	yellow    = "\033[33m"
	cyan      = "\033[36m"
	boldRed   = "\033[1;31m"
	boldGreen = "\033[1;32m"
	boldYel   = "\033[1;33m"
	boldCyan  = "\033[1;36m"
)

func colorize(code, text string) string {
	if !colorEnabled {
		return text
	}
	return code + text + reset
}

// Bold returns text wrapped in bold formatting.
func Bold(s string) string {
	return colorize(bold, s)
}

// Dim returns text wrapped in dim/faint formatting.
func Dim(s string) string {
	return colorize(dim, s)
}

// TurnBadge returns [Turn N] formatted in bold cyan.
func TurnBadge(turn int) string {
	return colorize(boldCyan, fmt.Sprintf("[Turn %d]", turn))
}

// SHABadge returns (sha) formatted in bold yellow with 7-char short SHA.
func SHABadge(sha string) string {
	short := sha
	if len(short) >= 7 {
		short = short[:7]
	}
	return colorize(boldYel, fmt.Sprintf("(%s)", short))
}

// BranchBadge returns [branch] formatted in bold green.
func BranchBadge(branch string) string {
	return colorize(boldGreen, fmt.Sprintf("[%s]", branch))
}

// Success formats a success message with a green prefix.
func Success(msg string) string {
	return colorize(boldGreen, "success:") + " " + msg
}

// Warning formats a warning message with a yellow prefix.
func Warning(msg string) string {
	return colorize(boldYel, "warning:") + " " + msg
}

// Error formats an error message with a red prefix.
func Error(msg string) string {
	return colorize(boldRed, "error:") + " " + msg
}

// Bullet returns an ASCII bullet point with a green bullet indicator.
func Bullet(msg string) string {
	return colorize(green, "  •") + " " + msg
}
