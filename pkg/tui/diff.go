package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/rivo/tview"
)

const targetLineWidth = 240

var (
	chromaStyle = styles.Get("github-dark")
	hunkRegex   = regexp.MustCompile(`^@@\s+-(\d+)(?:,\d+)?\s+\+(\d+)(?:,\d+)?\s+@@(.*)$`)
)

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

// FormatDiff cleans raw git diff output into GitHub/Delta style with line numbers,
// horizontal line width preservation, and syntax-highlighted tokens.
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

	oldLine := 0
	newLine := 0

	for _, line := range lines {
		// 1. Drop Git Plumbing lines
		if strings.HasPrefix(line, "diff --git ") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "new file mode ") ||
			strings.HasPrefix(line, "deleted file mode ") ||
			strings.HasPrefix(line, "\\ No newline at end of file") {
			continue
		}

		// 2. Discard raw file header markers
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

			header := fmt.Sprintf("\n[aqua::b] ▾ %s[-::-]\n[#30363d]%s[-]", filePath, strings.Repeat("─", 70))
			formattedLines = append(formattedLines, header)
			continue
		}

		// 4. Hunk Markers (@@ -old,len +new,len @@ context)
		if strings.HasPrefix(line, "@@") {
			matches := hunkRegex.FindStringSubmatch(line)
			if len(matches) >= 3 {
				oldLine, _ = strconv.Atoi(matches[1])
				newLine, _ = strconv.Atoi(matches[2])
				ctxText := strings.TrimSpace(matches[3])
				if ctxText != "" {
					formattedLines = append(formattedLines, fmt.Sprintf("          [#8b949e::d]%s[-::-]", tview.Escape(ctxText)))
				}
			}
			continue
		}

		// 5. Code Additions (+): Old empty, New line incremented
		if strings.HasPrefix(line, "+") {
			codeText := strings.TrimPrefix(line, "+")
			gutter := fmt.Sprintf("     %4d + ", newLine)
			newLine++

			highlighted, vLen := highlightTokensToTview(currentLexer, codeText, "#16331c")
			totalVisLen := len(gutter) + vLen
			padding := ""
			if totalVisLen < targetLineWidth {
				padding = strings.Repeat(" ", targetLineWidth-totalVisLen)
			}

			formattedLines = append(formattedLines, fmt.Sprintf("[:#16331c][#50fa7b]%s[:-]%s[:#16331c]%s[-:-]", gutter, highlighted, padding))
			continue
		}

		// 6. Code Deletions (-): Old line incremented, New empty
		if strings.HasPrefix(line, "-") {
			codeText := strings.TrimPrefix(line, "-")
			gutter := fmt.Sprintf("%4d      - ", oldLine)
			oldLine++

			highlighted, vLen := highlightTokensToTview(currentLexer, codeText, "#3d1a11")
			totalVisLen := len(gutter) + vLen
			padding := ""
			if totalVisLen < targetLineWidth {
				padding = strings.Repeat(" ", targetLineWidth-totalVisLen)
			}

			formattedLines = append(formattedLines, fmt.Sprintf("[:#3d1a11][#ff5555]%s[:-]%s[:#3d1a11]%s[-:-]", gutter, highlighted, padding))
			continue
		}

		// 7. Unchanged Context Lines: Both lines incremented
		if strings.HasPrefix(line, " ") {
			codeText := strings.TrimPrefix(line, " ")
			gutter := fmt.Sprintf("[#6e7681]%4d %4d   [-]", oldLine, newLine)
			oldLine++
			newLine++

			highlighted, _ := highlightTokensToTview(currentLexer, codeText, "")
			formattedLines = append(formattedLines, fmt.Sprintf("%s%s", gutter, highlighted))
			continue
		}

		if line != "" {
			escaped := tview.Escape(line)
			formattedLines = append(formattedLines, fmt.Sprintf("          [#c9d1d9]%s[-]", escaped))
		}
	}

	if len(formattedLines) == 0 {
		return "\n  [#8b949e]No modifications between this turn and current workspace.[-]"
	}

	return strings.Join(formattedLines, "\n")
}
