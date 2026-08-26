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

	// 1. Configure Dark Grey / Slate Palette
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

	// 2. Left Pane: Turn Timeline
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

	// Safe SHA retrieval
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
		// Display full commit message and SHA in the header
		fullMsg := strings.TrimSpace(record.Message)
		diffView.SetTitle(fmt.Sprintf(" [::b]Turn %d: %s - %s[::-] ", record.Turn, shaDisplay, tview.Escape(fullMsg)))
		diffView.SetText(FormatDiff(rawDiff))
		diffView.ScrollToBeginning()
	}

	// Populate Left Pane List with Escaped Brackets
	for _, turn := range revHistory {
		record := turn
		ref := sess.ActiveRef(record.Turn)
		sha, _ := gitengine.GetRef(ref)
		shaDisplay := getSafeSHA(sha)

		// ESCAPE the brackets so tview doesn't parse the SHA as a color tag
		mainText := fmt.Sprintf("● Turn %-2d [gray][[-][aqua]%s[gray]][-]", record.Turn, shaDisplay)

		// Clean message
		msg := strings.TrimSpace(record.Message)
		if msg == "" {
			msg = "turn snapshot"
		}
		secText := fmt.Sprintf("  %s", tview.Escape(msg))

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

	// 3. Right Pane: Diff Viewport
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

	// 4. Bottom Bar: Escaped Keys
	bottomBar.SetDynamicColors(true)
	renderBottomBar := func(statusMsg string) {
		baseHelp := " [gray][[yellow::b]j/k/↑/↓[gray::-]][white] Select Turn  [gray]•[white]  [gray][[yellow::b]Tab/h/l[gray::-]][white] Switch Pane  [gray]•[white]  [gray][[yellow::b]r[gray::-]][white] Revert Workspace  [gray]•[white]  [gray][[yellow::b]q/Esc[gray::-]][white] Quit"
		if statusMsg != "" {
			bottomBar.SetText(fmt.Sprintf(" %s  [gray]•[white]%s", statusMsg, baseHelp))
		} else {
			bottomBar.SetText(baseHelp)
		}
	}
	renderBottomBar("")

	// Main Split Flex: Left gets 2 units (40%), Right gets 3 units (60%) for wider message display
	mainSplit := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(turnList, 0, 2, true).
		AddItem(diffView, 0, 3, false)

	// Root Flex
	rootFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(mainSplit, 0, 1, true).
		AddItem(bottomBar, 1, 0, false)

	// Focus Management
	updateFocusColors := func() {
		if app.GetFocus() == turnList {
			turnList.SetBorderColor(tcell.ColorAqua)
			diffView.SetBorderColor(borderGrey)
		} else {
			turnList.SetBorderColor(borderGrey)
			diffView.SetBorderColor(tcell.ColorAqua)
		}
	}

	// Global Keyboard Input Capture
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
					renderBottomBar(fmt.Sprintf(
						"[green]✔ Workspace reverted to Turn %d (%s)[-]",
						selectedTurn.Turn, safeSHA,
					))
					updateDiffForTurn(selectedTurn)
				}
			}
			return nil
		}

		return event
	})

	return app.SetRoot(rootFlex, true).Run()
}
