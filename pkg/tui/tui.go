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

	// Palette configuration
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

	// Left Pane: Turn Timeline
	turnList.SetBorder(true).
		SetTitle("[::b]Turn Timeline[::-]").
		SetBorderColor(tcell.ColorAqua)

	turnList.SetMainTextColor(tcell.ColorLightCyan).
		SetSecondaryTextColor(tcell.ColorLightGray).
		SetSelectedBackgroundColor(tcell.NewHexColor(0x21262d)).
		SetSelectedTextColor(tcell.ColorWhite)

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

	// Bounding limits for diff viewport
	currentMaxLines := 0
	currentMaxCols := 0

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
		fullMsg := strings.TrimSpace(record.Message)
		diffView.SetTitle(fmt.Sprintf(" [::b]Turn %d: %s - %s[::-] ", record.Turn, shaDisplay, tview.Escape(fullMsg)))

		formattedText, numLines, maxLineLen := FormatDiff(rawDiff)
		currentMaxLines = numLines
		currentMaxCols = maxLineLen

		diffView.SetText(formattedText)
		diffView.ScrollToBeginning()
	}

	// Populate Left Pane
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
		diffView.SetText("\n  [gray]No turns recorded in active session.[-]")
	}

	turnList.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(revHistory) {
			updateDiffForTurn(revHistory[index])
		}
	})

	// Right Pane: Diff Viewport
	diffView.SetBorder(true).
		SetTitle("[::b]Diff (Turn Timeline vs Working Tree)[::-]").
		SetBorderColor(borderGrey)

	diffView.SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(false)

	// Clamped Keyboard Scrolling
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

	// Clamped Mouse Scrolling
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

	// Bottom Bar
	bottomBar.SetDynamicColors(true)
	renderBottomBar := func(statusMsg string) {
		baseHelp := " [gray][[yellow::b]j/k/↑/↓[gray::-]][white] Select Turn  [gray]•[white]  [gray][[yellow::b]Tab[gray::-]][white] Switch Pane  [gray]•[white]  [gray][[yellow::b]h/l/←/→[gray::-]][white] Scroll Diff  [gray]•[white]  [gray][[yellow::b]r[gray::-]][white] Revert  [gray]•[white]  [gray][[yellow::b]q/Esc[gray::-]][white] Quit"
		if statusMsg != "" {
			bottomBar.SetText(fmt.Sprintf(" %s  [gray]•[white]%s", statusMsg, baseHelp))
		} else {
			bottomBar.SetText(baseHelp)
		}
	}
	renderBottomBar("")

	mainSplit := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(turnList, 0, 2, true).
		AddItem(diffView, 0, 3, false)

	rootFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(mainSplit, 0, 1, true).
		AddItem(bottomBar, 1, 0, false)

	updateFocusColors := func() {
		if app.GetFocus() == turnList {
			turnList.SetBorderColor(tcell.ColorAqua)
			diffView.SetBorderColor(borderGrey)
		} else {
			turnList.SetBorderColor(borderGrey)
			diffView.SetBorderColor(tcell.ColorAqua)
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
