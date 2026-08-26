// Package tui contains the terminal user interface for edio
package tui

import (
	"fmt"
	"strings"

	"github.com/devxdh/edio/pkg/gitengine"
	"github.com/devxdh/edio/pkg/session"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// wrapWords splits long text strings into multi-line chunks of maximum maxLen characters.
func wrapWords(text string, maxLen int) string {
	if maxLen <= 10 {
		maxLen = 30
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	cur := words[0]
	for _, w := range words[1:] {
		if len(cur)+1+len(w) <= maxLen {
			cur += " " + w
		} else {
			lines = append(lines, cur)
			cur = w
		}
	}
	lines = append(lines, cur)
	return strings.Join(lines, "\n  ")
}

// Launch initializes and runs the interactive tview TUI application.
func Launch() error {
	if err := gitengine.EnsureGitRepo(); err != nil {
		return err
	}

	sess, err := session.LoadActiveSession()
	if err != nil {
		return fmt.Errorf("failed to load active session: %w", err)
	}

	history, err := sess.GetTurnHistory()
	if err != nil {
		return fmt.Errorf("failed to retrieve history: %w", err)
	}

	// Configure GitHub Dark Dimmed Palette (#161b22 / #1c2128 / #30363d)
	darkGreyBG := tcell.NewHexColor(0x161b22)
	panelGreyBG := tcell.NewHexColor(0x1c2128)
	borderGrey := tcell.NewHexColor(0x30363d)

	tview.Styles.PrimitiveBackgroundColor = darkGreyBG
	tview.Styles.ContrastBackgroundColor = panelGreyBG
	tview.Styles.MoreContrastBackgroundColor = tcell.NewHexColor(0x21262d)
	tview.Styles.PrimaryTextColor = tcell.ColorWhite
	tview.Styles.SecondaryTextColor = tcell.ColorLightGray
	tview.Styles.BorderColor = borderGrey

	app := tview.NewApplication()
	app.EnableMouse(true)

	// Components
	turnList := tview.NewList()
	diffView := tview.NewTextView()
	bottomBar := tview.NewTextView()

	turnList.SetBackgroundColor(darkGreyBG)
	diffView.SetBackgroundColor(darkGreyBG)
	bottomBar.SetBackgroundColor(panelGreyBG)

	// Left Pane: Turn Timeline (tview.List)
	turnList.SetBorder(true).
		SetTitle("[::b]Turn Timeline[::-]").
		SetBorderColor(tcell.ColorAqua)

	turnList.SetMainTextColor(tcell.ColorLightCyan).
		SetSecondaryTextColor(tcell.ColorLightGray).
		SetSelectedBackgroundColor(tcell.NewHexColor(0x21262d)).
		SetSelectedTextColor(tcell.ColorWhite)

	// Direct Input Capture on turnList for Vim-style j/k navigation
	turnList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		cur := turnList.GetCurrentItem()
		total := turnList.GetItemCount()

		switch event.Rune() {
		case 'j':
			if cur < total-1 {
				turnList.SetCurrentItem(cur + 1)
			}
			return nil
		case 'k':
			if cur > 0 {
				turnList.SetCurrentItem(cur - 1)
			}
			return nil
		}
		return event
	})

	// Reverse Chronological Order (newest turn at top)
	revHistory := make([]session.TurnRecord, len(history))
	for i, turn := range history {
		revHistory[len(history)-1-i] = turn
	}

	// Helper function to format SHA safely
	getSafeSHA := func(rawSHA string) string {
		clean := strings.TrimSpace(rawSHA)
		if len(clean) >= 7 {
			return clean[:7]
		}
		if len(clean) > 0 {
			return clean
		}
		return "-------"
	}

	// Helper function to update diff viewport for a turn
	updateDiffForTurn := func(record session.TurnRecord) {
		ref := sess.ActiveRef(record.Turn)
		sha, _ := gitengine.GetRef(ref)

		parentSHA := "HEAD"
		if record.Turn > 1 {
			parentRef := sess.ActiveRef(record.Turn - 1)
			pSHA, err := gitengine.GetRef(parentRef)
			if err == nil && pSHA != "" {
				parentSHA = pSHA
			}
		}

		rawDiff, err := gitengine.RunGit("diff", parentSHA, sha)
		if err != nil {
			rawDiff = ""
		}

		shaDisplay := getSafeSHA(sha)
		diffView.SetTitle(fmt.Sprintf("[::b]Diff (Turn %d: %s vs Workspace)[::-]", record.Turn, shaDisplay))
		diffView.SetText(FormatDiff(rawDiff))
		diffView.ScrollToBeginning()
	}

	for _, turn := range revHistory {
		record := turn
		ref := sess.ActiveRef(record.Turn)
		sha, _ := gitengine.GetRef(ref)
		shaDisplay := getSafeSHA(sha)
		mainText := fmt.Sprintf("● Turn %-2d [%s]", record.Turn, shaDisplay)

		// Get churn stats for turn
		parentSHA := "HEAD"
		if record.Turn > 1 {
			parentRef := sess.ActiveRef(record.Turn - 1)
			pSHA, _ := gitengine.GetRef(parentRef)
			if pSHA != "" {
				parentSHA = pSHA
			}
		}

		churnText := ""
		diffStat, err := gitengine.RunGit("diff", "--shortstat", parentSHA, sha)
		if err == nil && strings.TrimSpace(diffStat) != "" {
			churnText = " (" + strings.TrimSpace(diffStat) + ")"
		}

		// Multi-line word wrapping for complete message legibility
		rawMsg := strings.TrimSpace(record.Message)
		wrappedDesc := wrapWords(rawMsg+churnText, 32)
		secText := fmt.Sprintf("  %s\n", wrappedDesc)

		turnList.AddItem(mainText, secText, 0, nil)
	}

	// Trigger initial diff rendering for newest turn
	if len(revHistory) > 0 {
		updateDiffForTurn(revHistory[0])
	} else {
		diffView.SetText("\n  [gray]No turns recorded in active session.[-]")
	}

	turnList.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(revHistory) {
			updateDiffForTurn(revHistory[index])
		}
	})

	// Right Pane: Diff Viewport (tview.TextView)
	diffView.SetBorder(true).
		SetTitle("[::b]Diff (Turn Timeline vs Working Tree)[::-]").
		SetBorderColor(borderGrey)

	diffView.SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(false)

	// Direct Input Capture on diffView for 3-line step j/k scrolling
	diffView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, col := diffView.GetScrollOffset()
		switch event.Rune() {
		case 'j':
			diffView.ScrollTo(row+3, col)
			return nil
		case 'k':
			if row >= 3 {
				diffView.ScrollTo(row-3, col)
			} else {
				diffView.ScrollTo(0, col)
			}
			return nil
		}
		return event
	})

	// Bottom Bar (tview.TextView) with Escaped [[R]] Brackets
	bottomBar.SetDynamicColors(true)
	renderBottomBar := func(statusMsg string) {
		baseHelp := " [aqua::b][[j/k/↑/↓]][-::-] Select Turn  •  [aqua::b][[Tab/h/l]][-::-] Switch Pane  •  [aqua::b][[r]][-::-] Revert Workspace  •  [aqua::b][[q/Esc]][-::-] Quit"
		if statusMsg != "" {
			bottomBar.SetText(fmt.Sprintf("%s  •  %s", statusMsg, baseHelp))
		} else {
			bottomBar.SetText(baseHelp)
		}
	}
	renderBottomBar("")

	// Main Split Flex (Row 1)
	mainSplit := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(turnList, 0, 1, true).
		AddItem(diffView, 0, 2, false)

	// Root Flex (Vertical: Main Split + Bottom Bar)
	rootFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(mainSplit, 0, 1, true).
		AddItem(bottomBar, 1, 0, false)

	// Focus Management & Border Color Updates
	updateFocusColors := func() {
		if app.GetFocus() == turnList {
			turnList.SetBorderColor(tcell.ColorAqua)
			diffView.SetBorderColor(borderGrey)
		} else {
			turnList.SetBorderColor(borderGrey)
			diffView.SetBorderColor(tcell.ColorAqua)
		}
	}

	// Global Keyboard Input Capture (ONLY Tab, r, and q/Esc to avoid hijacking widget keys)
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		key := event.Key()
		runeChar := event.Rune()

		switch key {
		case tcell.KeyTab:
			if app.GetFocus() == turnList {
				app.SetFocus(diffView)
			} else {
				app.SetFocus(turnList)
			}
			updateFocusColors()
			return nil

		case tcell.KeyEscape:
			app.Stop()
			return nil
		}

		if runeChar == 'q' || runeChar == 'Q' {
			app.Stop()
			return nil
		}

		if runeChar == 'h' || runeChar == 'l' {
			if app.GetFocus() == turnList {
				app.SetFocus(diffView)
			} else {
				app.SetFocus(turnList)
			}
			updateFocusColors()
			return nil
		}

		if runeChar == 'r' || runeChar == 'R' {
			idx := turnList.GetCurrentItem()
			if idx >= 0 && idx < len(revHistory) {
				selectedTurn := revHistory[idx]
				ref := sess.ActiveRef(selectedTurn.Turn)
				sha, _ := gitengine.GetRef(ref)

				_, err := gitengine.RunGit("checkout", sha, "--", ".")
				if err != nil {
					renderBottomBar(fmt.Sprintf("[red]✘ Error: %v[-]", err))
				} else {
					safeSHA := getSafeSHA(sha)
					renderBottomBar(
						fmt.Sprintf(
							"[green]✔ Workspace reverted to Turn %d (%s)[-]",
							selectedTurn.Turn, safeSHA,
						),
					)
					updateDiffForTurn(selectedTurn)
				}
			}
			return nil
		}

		return event
	})

	return app.SetRoot(rootFlex, true).Run()
}
