package tui

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/rivo/tview"
)

const targetLineWidth = 220

var chromaStyle = styles.Get("github-dark")

func init() {
	if chromaStyle == nil {
		chromaStyle = styles.Fallback
	}
}

func highlightTokensToTview(lexer chroma.Lexer, codeText, bgHex string) (string, int) {
	if lexer == nil {
		lexer = lexers.Fallback
	}

	iterator, err := lexer.Tokenise(nil, codeText)
	if err != nil {
		escaped := tview.Escape(codeText)
		if bgHex != "" {
			return fmt.Sprintf("[:%s][#c9d1d9]%s[-:-]", bgHex, escaped), len(codeText)
		}
		return fmt.Sprintf("[#c9d1d9]%s[-]", escaped), len(codeText)
	}

	var sb strings.Builder
	visibleLen := 0

	for _, token := range iterator.Tokens() {
		val := token.Value
		if val == "" {
			continue
		}

		visibleLen += len(val)
		escapedVal := tview.Escape(val)

		entry := chromaStyle.Get(token.Type)
		colorHex := "#c9d1d9" // GitHub Dark default text color
		if entry.Colour.IsSet() {
			colorHex = entry.Colour.String()
		}

		if bgHex != "" {
			sb.WriteString(fmt.Sprintf("[:%s][%s]%s[-:-]", bgHex, colorHex, escapedVal))
		} else {
			sb.WriteString(fmt.Sprintf("[%s]%s[-]", colorHex, escapedVal))
		}
	}

	return sb.String(), visibleLen
}

// FormatDiff cleans raw git diff output into GitHub/Delta style with full-line edge-to-edge
// background tints, github-dark syntax-highlighted code tokens, and clean section headers.
func FormatDiff(rawDiff string) string {
	if strings.TrimSpace(rawDiff) == "" {
		return "\n  [#8b949e]No modifications between this turn and current workspace.[-]"
	}

	lines := strings.Split(rawDiff, "\n")
	var formattedLines []string

	currentLexer := lexers.Get("go")
	if currentLexer == nil {
		currentLexer = lexers.Fallback
	}

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

		// 3. Clean File Header Bar (+++ b/path) & update language lexer
		if strings.HasPrefix(line, "+++ b/") {
			filePath := strings.TrimPrefix(line, "+++ b/")
			matchedLexer := lexers.Match(filePath)
			if matchedLexer != nil {
				currentLexer = chroma.Coalesce(matchedLexer)
			} else {
				currentLexer = lexers.Fallback
			}

			header := fmt.Sprintf("\n[aqua::b] ▾ %s[-::-]\n[#30363d]%s[-]", filePath, strings.Repeat("─", 60))
			formattedLines = append(formattedLines, header)
			continue
		}

		// 4. Hunk Markers (@@ ... @@ context)
		if strings.HasPrefix(line, "@@") {
			parts := strings.SplitN(line, "@@", 3)
			if len(parts) >= 3 {
				ctxText := strings.TrimSpace(parts[2])
				if ctxText != "" {
					formattedLines = append(formattedLines, fmt.Sprintf("  [#8b949e::d]%s[-::-]", tview.Escape(ctxText)))
				}
			}
			continue
		}

		// 5. Code Additions (+): Full-width dark green background (#16331c) with github-dark syntax tokens
		if strings.HasPrefix(line, "+") {
			codeText := strings.TrimPrefix(line, "+")
			highlighted, vLen := highlightTokensToTview(currentLexer, codeText, "#16331c")
			totalVisLen := 2 + vLen
			padding := ""
			if totalVisLen < targetLineWidth {
				padding = strings.Repeat(" ", targetLineWidth-totalVisLen)
			}
			formattedLines = append(formattedLines, fmt.Sprintf("[:#16331c][#50fa7b]+ [:-]%s[:#16331c]%s[-:-]", highlighted, padding))
			continue
		}

		// 6. Code Deletions (-): Full-width dark red background (#3d1a11) with github-dark syntax tokens
		if strings.HasPrefix(line, "-") {
			codeText := strings.TrimPrefix(line, "-")
			highlighted, vLen := highlightTokensToTview(currentLexer, codeText, "#3d1a11")
			totalVisLen := 2 + vLen
			padding := ""
			if totalVisLen < targetLineWidth {
				padding = strings.Repeat(" ", targetLineWidth-totalVisLen)
			}
			formattedLines = append(formattedLines, fmt.Sprintf("[:#3d1a11][#ff5555]- [:-]%s[:#3d1a11]%s[-:-]", highlighted, padding))
			continue
		}

		// 7. Unchanged Context Lines: Clean, high-contrast code tokens (no dark muddy background)
		if strings.HasPrefix(line, " ") {
			codeText := strings.TrimPrefix(line, " ")
			highlighted, _ := highlightTokensToTview(currentLexer, codeText, "")
			formattedLines = append(formattedLines, fmt.Sprintf("  %s", highlighted))
			continue
		}

		if line != "" {
			escaped := tview.Escape(line)
			formattedLines = append(formattedLines, fmt.Sprintf("  [#c9d1d9]%s[-]", escaped))
		}
	}

	if len(formattedLines) == 0 {
		return "\n  [#8b949e]No modifications between this turn and current workspace.[-]"
	}

	return strings.Join(formattedLines, "\n")
}
