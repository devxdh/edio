package tui

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
)

const targetLineWidth = 220

func formatDiffLine(prefix, content, bgColor, textColor string) string {
	raw := fmt.Sprintf("%s %s", prefix, content)
	visibleLen := len(raw)
	padding := ""
	if visibleLen < targetLineWidth {
		padding = strings.Repeat(" ", targetLineWidth-visibleLen)
	}
	return fmt.Sprintf("[:%s][%s]%s%s[-:-]", bgColor, textColor, tview.Escape(raw), padding)
}

// FormatDiff cleans raw git diff output into GitHub/Delta style with full-line edge-to-edge
// background tints, cyan file section header bars, and italic hunk context lines.
func FormatDiff(rawDiff string) string {
	if strings.TrimSpace(rawDiff) == "" {
		return "\n  [gray]No modifications between this turn and current workspace.[-]"
	}

	lines := strings.Split(rawDiff, "\n")
	var formattedLines []string

	for _, line := range lines {
		// 1. Drop Git Plumbing lines
		if strings.HasPrefix(line, "diff --git ") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "new file mode ") ||
			strings.HasPrefix(line, "deleted file mode ") ||
			strings.HasPrefix(line, "\\ No newline at end of file") {
			continue
		}

		// 2. Discard unwanted file header markers
		if strings.HasPrefix(line, "--- a/") || strings.HasPrefix(line, "--- /dev/null") || strings.HasPrefix(line, "+++ /dev/null") {
			continue
		}

		// 3. Clean File Header Bar (+++ b/path)
		if strings.HasPrefix(line, "+++ b/") {
			filePath := strings.TrimPrefix(line, "+++ b/")
			header := fmt.Sprintf("\n[aqua::b] ▾ %s[-::-]\n[gray]%s[-]", filePath, strings.Repeat("─", 60))
			formattedLines = append(formattedLines, header)
			continue
		}

		// 4. Hunk Markers (@@ ... @@ context)
		if strings.HasPrefix(line, "@@") {
			parts := strings.SplitN(line, "@@", 3)
			if len(parts) >= 3 {
				ctxText := strings.TrimSpace(parts[2])
				if ctxText != "" {
					formattedLines = append(formattedLines, fmt.Sprintf("  [gray::d]%s[-::-]", tview.Escape(ctxText)))
				}
			}
			continue
		}

		// 5. Code Additions (+): Full-width dark green background bar
		if strings.HasPrefix(line, "+") {
			codeText := strings.TrimPrefix(line, "+")
			formattedLines = append(formattedLines, formatDiffLine("+", codeText, "#16331c", "white"))
			continue
		}

		// 6. Code Deletions (-): Full-width dark red background bar
		if strings.HasPrefix(line, "-") {
			codeText := strings.TrimPrefix(line, "-")
			formattedLines = append(formattedLines, formatDiffLine("-", codeText, "#3d1a11", "white"))
			continue
		}

		// 7. Unchanged Context Lines
		if strings.HasPrefix(line, " ") {
			codeText := strings.TrimPrefix(line, " ")
			escaped := tview.Escape(codeText)
			formattedLines = append(formattedLines, fmt.Sprintf("  [white]%s[-]", escaped))
			continue
		}

		if line != "" {
			formattedLines = append(formattedLines, fmt.Sprintf("  [white]%s[-]", tview.Escape(line)))
		}
	}

	if len(formattedLines) == 0 {
		return "\n  [gray]No modifications between this turn and current workspace.[-]"
	}

	return strings.Join(formattedLines, "\n")
}
