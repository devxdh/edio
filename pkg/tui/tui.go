package tui

import (
	"fmt"
	"strings"

	"github.com/devxdh/edio/pkg/gitengine"
	"github.com/devxdh/edio/pkg/session"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const emptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

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

	// 1. Configure Theme Palette
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
	diffHeader := tview.NewTextView()
	diffView := tview.NewTextView()
	bottomBar := tview.NewTextView()

	turnList.SetBackgroundColor(darkGreyBG)
	diffHeader.SetBackgroundColor(panelGreyBG)
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

	// Keyboard Input Capture on turnList
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

	// Mouse Capture on turnList
	turnList.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		cur := turnList.GetCurrentItem()
		total := turnList.GetItemCount()

		switch action {
		case tview.MouseScrollUp:
			if cur > 0 {
				turnList.SetCurrentItem(cur - 1)
			}
			return action, nil
		case tview.MouseScrollDown:
			if cur < total-1 {
				turnList.SetCurrentItem(cur + 1)
			}
			return action, nil
		}
		return action, event
	})

	// Reverse Chronological Order
	revHistory := make([]session.TurnRecord, len(history))
	for i, turn := range history {
		revHistory[len(history)-1-i] = turn
	}

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

	// 3. Right Pane: Header & Diff Viewport
	diffHeader.SetDynamicColors(true).
		SetWrap(true).
		SetBorder(true).
		SetBorderColor(borderGrey)

	diffView.SetBorder(true).
		SetBorderColor(borderGrey)

	diffView.SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(false)

	currentMaxLines := 0
	currentMaxCols := 0

	updateDiffForTurn := func(record session.TurnRecord) {
		ref := sess.ActiveRef(record.Turn)
		sha, _ := gitengine.GetRef(ref)

		parentSHA := emptyTreeSHA
		if record.Turn == 1 {
			if sess.BaseCommitSHA != "" {
				parentSHA = sess.BaseCommitSHA
			}
		} else {
			parentRef := sess.ActiveRef(record.Turn - 1)
			pSHA, err := gitengine.GetRef(parentRef)
			if err == nil && pSHA != "" {
				parentSHA = pSHA
			}
		}

		// Calculate churn stats
		churnText := ""
		diffStat, err := gitengine.RunGit("diff", "--shortstat", parentSHA, sha)
		if err == nil && strings.TrimSpace(diffStat) != "" {
			churnText = fmt.Sprintf("[gray](%s)[-]", strings.TrimSpace(diffStat))
		}

		rawDiff, err := gitengine.RunGit("diff", parentSHA, sha)
		if err != nil {
			rawDiff = ""
		}

		shaDisplay := getSafeSHA(sha)
		fullMsg := strings.TrimSpace(record.Message)
		if fullMsg == "" {
			fullMsg = "turn snapshot"
		}

		// Render multi-line formatted metadata header with guaranteed zero truncation
		diffHeader.SetTitle(fmt.Sprintf(" [::b]Turn %d: %s[::-] ", record.Turn, shaDisplay))
		diffHeader.SetText(fmt.Sprintf(" [white::b]%s[-]  %s", tview.Escape(fullMsg), churnText))

		formattedText, numLines, maxLineLen := FormatDiff(rawDiff)
		currentMaxLines = numLines
		currentMaxCols = maxLineLen

		diffView.SetTitle(fmt.Sprintf(" [::b]Changes vs Parent (%s)[::-] ", getSafeSHA(parentSHA)))
		diffView.SetText(formattedText)
		diffView.ScrollToBeginning()
	}

	// Populate Timeline List
	for _, turn := range revHistory {
		record := turn
		ref := sess.ActiveRef(record.Turn)
		sha, _ := gitengine.GetRef(ref)
		shaDisplay := getSafeSHA(sha)

		mainText := fmt.Sprintf("● Turn %-2d [gray][[-][aqua]%s[gray]][-]", record.Turn, shaDisplay)
		msg := strings.TrimSpace(record.Message)
		if msg == "" {
			msg = "turn snapshot"
		}
		secText := fmt.Sprintf("  %s", tview.Escape(msg))

		turnList.AddItem(mainText, secText, 0, nil)
	}

	if len(revHistory) > 0 {
		updateDiffForTurn(revHistory[0])
	} else {
		diffHeader.SetText("  [gray]No active turns[-]")
		diffView.SetText("\n  [gray]No turns recorded in active session.[-]")
	}

	turnList.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(revHistory) {
			updateDiffForTurn(revHistory[index])
		}
	})

	// Diff Viewport Keyboard Scrolling
	diffView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, col := diffView.GetScrollOffset()
		_, _, viewWidth, viewHeight := diffView.GetInnerRect()

		maxColScroll := currentMaxCols - viewWidth
		if maxColScroll < 0 {
			maxColScroll = 0
		}
		maxRowScroll := currentMaxLines - viewHeight
		if maxRowScroll < 0 {
			maxRowScroll = 0
		}

		switch event.Key() {
		case tcell.KeyLeft:
			col -= 4
			if col < 0 {
				col = 0
			}
			diffView.ScrollTo(row, col)
			return nil
		case tcell.KeyRight:
			col += 4
			if col > maxColScroll {
				col = maxColScroll
			}
			diffView.ScrollTo(row, col)
			return nil
		case tcell.KeyUp:
			row -= 3
			if row < 0 {
				row = 0
			}
			diffView.ScrollTo(row, col)
			return nil
		case tcell.KeyDown:
			row += 3
			if row > maxRowScroll {
				row = maxRowScroll
			}
			diffView.ScrollTo(row, col)
			return nil
		}

		switch event.Rune() {
		case 'j':
			row += 3
			if row > maxRowScroll {
				row = maxRowScroll
			}
			diffView.ScrollTo(row, col)
			return nil
		case 'k':
			row -= 3
			if row < 0 {
				row = 0
			}
			diffView.ScrollTo(row, col)
			return nil
		case 'h':
			col -= 4
			if col < 0 {
				col = 0
			}
			diffView.ScrollTo(row, col)
			return nil
		case 'l':
			col += 4
			if col > maxColScroll {
				col = maxColScroll
			}
			diffView.ScrollTo(row, col)
			return nil
		}
		return event
	})

	// Diff Viewport Mouse Scrolling
	diffView.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		row, col := diffView.GetScrollOffset()
		_, _, viewWidth, viewHeight := diffView.GetInnerRect()

		maxColScroll := currentMaxCols - viewWidth
		if maxColScroll < 0 {
			maxColScroll = 0
		}
		maxRowScroll := currentMaxLines - viewHeight
		if maxRowScroll < 0 {
			maxRowScroll = 0
		}

		switch action {
		case tview.MouseScrollLeft:
			col -= 4
			if col < 0 {
				col = 0
			}
			diffView.ScrollTo(row, col)
			return action, nil
		case tview.MouseScrollRight:
			col += 4
			if col > maxColScroll {
				col = maxColScroll
			}
			diffView.ScrollTo(row, col)
			return action, nil
		case tview.MouseScrollUp:
			row -= 3
			if row < 0 {
				row = 0
			}
			diffView.ScrollTo(row, col)
			return action, nil
		case tview.MouseScrollDown:
			row += 3
			if row > maxRowScroll {
				row = maxRowScroll
			}
			diffView.ScrollTo(row, col)
			return action, nil
		}
		return action, event
	})

	// 4. Bottom Bar
	bottomBar.SetDynamicColors(true)
	renderBottomBar := func(statusMsg string) {
		baseHelp := " [white][[yellow::b]j/k/↑/↓[white::-]][white] Select Turn  [white]•[white]  [white][[yellow::b]Tab[white::-]][white] Switch Pane  [white]•[white]  [white][[yellow::b]h/l/←/→[white::-]][white] Scroll Diff  [white]•[white]  [white][[yellow::b]r[white::-]][white] Revert Workspace  [white]•[white]  [white][[yellow::b]q/Esc[white::-]][white] Quit"
		if statusMsg != "" {
			bottomBar.SetText(fmt.Sprintf(" %s  [white]•[white]%s", statusMsg, baseHelp))
		} else {
			bottomBar.SetText(baseHelp)
		}
	}
	renderBottomBar("")

	// Right Pane Layout: Multi-line Header (3 lines) + Diff View (remainder)
	rightPaneFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(diffHeader, 3, 0, false).
		AddItem(diffView, 0, 1, false)

	// Main Horizontal Split (35% Timeline, 65% Diff View)
	mainSplit := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(turnList, 0, 1, true).
		AddItem(rightPaneFlex, 0, 2, false)

	// Root Flex
	rootFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(mainSplit, 0, 1, true).
		AddItem(bottomBar, 1, 0, false)

	updateFocusColors := func() {
		if app.GetFocus() == turnList {
			turnList.SetBorderColor(tcell.ColorAqua)
			diffView.SetBorderColor(borderGrey)
			diffHeader.SetBorderColor(borderGrey)
		} else {
			turnList.SetBorderColor(borderGrey)
			diffView.SetBorderColor(tcell.ColorAqua)
			diffHeader.SetBorderColor(tcell.ColorAqua)
		}
	}

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
					renderBottomBar(fmt.Sprintf("[green]✔ Workspace reverted to Turn %d (%s)[-]", selectedTurn.Turn, safeSHA))
					updateDiffForTurn(selectedTurn)
				}
			}
			return nil
		}

		return event
	})

	return app.SetRoot(rootFlex, true).Run()
}
