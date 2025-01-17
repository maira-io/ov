package oviewer

import (
	"fmt"
	"log"
	"strings"

	"github.com/gdamore/tcell/v2"
)

const statusline = 1

// draw is the main routine that draws the screen.
func (root *Root) draw() {
	m := root.Doc

	root.Screen.Clear()
	if m.BufEndNum() == 0 || root.vHight == 0 {
		m.topLN = 0
		root.statusDraw()
		root.Show()
		return
	}

	// Header
	lY := root.drawHeader()

	lX := 0
	if m.WrapMode {
		lX = m.topLX
	}

	m.topLN = max(m.topLN, 0)

	// Body
	lX, lY = root.drawBody(lX, lY)

	root.bottomLN = m.topLN + max(lY, 0)
	root.bottomLX = lX

	if root.mouseSelect {
		root.drawSelect(root.x1, root.y1, root.x2, root.y2, true)
	}

	root.statusDraw()
	root.Show()
}

// drawHeader draws header.
func (root *Root) drawHeader() int {
	m := root.Doc

	// lY is a logical line.
	lY := m.SkipLines
	// lX is the x position of Contents.
	lX := 0

	// wrap is the number of wrapped lines.
	wrap := 0
	// hy is the drawing line.
	hy := 0
	for ; lY < m.firstLine(); hy++ {
		if hy > root.vHight {
			break
		}

		lc := m.getContents(lY, m.TabWidth)
		// column highlight
		if m.ColumnMode {
			str, posCV := ContentsToStr(lc)
			start, end := rangePosition(str, m.ColumnDelimiter, m.columnNum)
			root.columnHighlight(lc, posCV[start], posCV[end])
		}

		// line number mode
		if m.LineNumMode {
			numC := StrToContents(strings.Repeat(" ", root.startX-1), m.TabWidth)
			root.setContentString(0, hy, numC)
		}

		root.lnumber[hy] = lineNumber{
			line: lY,
			wrap: wrap,
		}

		if m.WrapMode {
			lX, lY = root.drawWrapLine(hy, lX, lY, lc)
			if lX > 0 {
				wrap++
			} else {
				wrap = 0
			}
		} else {
			lX, lY = root.drawNoWrapLine(hy, m.x, lY, lc)
		}

		// header style
		for x := 0; x < root.vWidth; x++ {
			r, c, style, _ := root.GetContent(x, hy)
			root.Screen.SetContent(x, hy, r, c, applyStyle(style, root.StyleHeader))
		}
	}
	root.headerLen = hy
	return lY
}

func (root *Root) drawBody(lX int, lY int) (int, int) {
	m := root.Doc

	listX, err := root.leftMostX(m.topLN + root.Doc.firstLine() + lY)
	if err != nil {
		log.Printf("drawBody %d:%s", m.topLN+lY, err)
	}
	wrap := numOfSlice(listX, lX)

	markStyleWidth := min(root.vWidth, root.Doc.general.MarkStyleWidth)

	root.Doc.lastContentsNum = -1

	// lc, lineStr, byteMap store the previous value.
	// Because it may be a continuation from the previous line in wrap mode.
	lastLY := -1
	var lc lineContents
	var lineStr string
	var posCV map[int]int
	for y := root.headerLen; y < root.vHight-statusline; y++ {
		if lastLY != lY {
			lc = m.getContents(m.topLN+lY, m.TabWidth)
			lineStr, posCV = m.getContentsStr(m.topLN+lY, lc)
			lastLY = lY

			root.lineStyle(lc, root.StyleBody)
			root.lnumber[y] = lineNumber{
				line: -1,
				wrap: 0,
			}
		}

		// column highlight
		if root.Doc.ColumnMode {
			start, end := rangePosition(lineStr, m.ColumnDelimiter, m.columnNum)
			root.columnHighlight(lc, posCV[start], posCV[end])
		}

		// search highlight
		if root.searchWord != "" {
			poss := root.searchPosition(m.topLN+lY, lineStr)
			for _, r := range poss {
				root.searchHighlight(lc, posCV[r[0]], posCV[r[1]])
			}
		}

		// line number mode
		if m.LineNumMode {
			numC := StrToContents(fmt.Sprintf("%*d", root.startX-1, m.topLN+lY-m.firstLine()+1), m.TabWidth)
			for i := 0; i < len(numC); i++ {
				numC[i].style = applyStyle(tcell.StyleDefault, root.StyleLineNumber)
			}
			root.setContentString(0, y, numC)
		}

		root.lnumber[y] = lineNumber{
			line: m.topLN + lY,
			wrap: wrap,
		}

		var nextY int
		if m.WrapMode {
			lX, nextY = root.drawWrapLine(y, lX, lY, lc)
			if lX > 0 {
				wrap++
			} else {
				wrap = 0
			}
		} else {
			lX, nextY = root.drawNoWrapLine(y, m.x, lY, lc)
		}

		// alternate style applies from beginning to end of line, not content.
		if m.AlternateRows {
			if (m.topLN+lY)%2 == 1 {
				for x := 0; x < root.vWidth; x++ {
					r, c, style, _ := root.GetContent(x, y)
					root.SetContent(x, y, r, c, applyStyle(style, root.StyleAlternate))
				}
			}
		}

		// mark style.
		if containsInt(m.marked, m.topLN+lY) {
			for x := 0; x < markStyleWidth; x++ {
				r, c, style, _ := root.GetContent(x, y)
				root.SetContent(x, y, r, c, applyStyle(style, root.StyleMarkLine))
			}
		}

		lY = nextY
	}

	return lX, lY
}

// drawWrapLine wraps and draws the contents and returns the next drawing position.
func (root *Root) drawWrapLine(y int, lX int, lY int, lc lineContents) (int, int) {
	if lX < 0 {
		log.Printf("Illegal lX:%d", lX)
		return 0, 0
	}

	for x := 0; ; x++ {
		if lX+x >= len(lc) {
			// EOL
			lX = 0
			lY++
			break
		}
		content := lc[lX+x]
		if x+content.width+root.startX > root.vWidth {
			// EOL
			lX += x
			break
		}
		root.Screen.SetContent(root.startX+x, y, content.mainc, content.combc, content.style)
	}

	return lX, lY
}

// drawNoWrapLine draws contents without wrapping and returns the next drawing position.
func (root *Root) drawNoWrapLine(y int, lX int, lY int, lc lineContents) (int, int) {
	if lX < root.minStartX {
		lX = root.minStartX
	}

	for x := 0; root.startX+x < root.vWidth; x++ {
		if lX+x >= len(lc) {
			// EOL
			break
		}
		content := DefaultContent
		if lX+x >= 0 {
			content = lc[lX+x]
		}
		root.Screen.SetContent(root.startX+x, y, content.mainc, content.combc, content.style)
	}
	lY++

	return lX, lY
}

// lineStyle applies the style for one line.
func (root *Root) lineStyle(lc lineContents, style ovStyle) {
	RangeStyle(lc, 0, len(lc), style)
}

// searchHighlight applies the style of the search highlight.
func (root *Root) searchHighlight(lc lineContents, start int, end int) {
	RangeStyle(lc, start, end, root.StyleSearchHighlight)
}

// columnHighlight applies the style of the column highlight.
func (root *Root) columnHighlight(lc lineContents, start int, end int) {
	RangeStyle(lc, start, end, root.StyleColumnHighlight)
}

// RangeStyle applies the style to the specified range.
func RangeStyle(lc lineContents, start int, end int, style ovStyle) {
	for x := start; x < end; x++ {
		lc[x].style = applyStyle(lc[x].style, style)
	}
}

// statusDraw draws a status line.
func (root *Root) statusDraw() {
	leftContents, cursorPos := root.leftStatus()
	root.setContentString(0, root.statusPos, leftContents)

	rightContents := root.rightStatus()
	root.setContentString(root.vWidth-len(rightContents), root.statusPos, rightContents)

	root.Screen.ShowCursor(cursorPos, root.statusPos)
}

func (root *Root) leftStatus() (lineContents, int) {
	if root.input.mode == Normal {
		return root.normalLeftStatus()
	}
	return root.inputLeftStatus()
}

func (root *Root) normalLeftStatus() (lineContents, int) {
	number := ""
	if root.DocumentLen() > 1 && root.screenMode == Docs {
		number = fmt.Sprintf("[%d]", root.CurrentDoc)
	}
	follow := ""
	if root.Doc.FollowMode {
		follow = "(Follow Mode)"
	}
	if root.General.FollowAll {
		follow = "(Follow All)"
	}
	leftStatus := fmt.Sprintf("%s%s%s:%s", number, follow, root.Doc.FileName, root.message)
	leftContents := StrToContents(leftStatus, -1)
	color := tcell.ColorWhite
	if root.CurrentDoc != 0 {
		color = tcell.Color((root.CurrentDoc + 8) % 16)
	}
	for i := 0; i < len(leftContents); i++ {
		leftContents[i].style = leftContents[i].style.Foreground(tcell.ColorValid + color).Reverse(true)
	}
	return leftContents, len(leftContents)
}

func (root *Root) inputLeftStatus() (lineContents, int) {
	input := root.input
	searchMode := ""
	if input.mode == Search || input.mode == Backsearch {
		if root.Config.RegexpSearch {
			searchMode += "(R)"
		}
		if root.Config.Incsearch {
			searchMode += "(I)"
		}
		if root.CaseSensitive {
			searchMode += "(Aa)"
		}
	}
	p := searchMode + input.EventInput.Prompt()
	leftStatus := p + input.value
	leftContents := StrToContents(leftStatus, -1)
	return leftContents, len(p) + input.cursorX
}

func (root *Root) rightStatus() lineContents {
	next := ""
	if !root.Doc.BufEOF() {
		next = "..."
	}
	str := fmt.Sprintf("(%d/%d%s)", root.Doc.topLN, root.Doc.BufEndNum(), next)
	return StrToContents(str, -1)
}

// setContentString is a helper function that draws a string with setContent.
func (root *Root) setContentString(vx int, vy int, lc lineContents) {
	screen := root.Screen
	for x, content := range lc {
		screen.SetContent(vx+x, vy, content.mainc, content.combc, content.style)
	}
	screen.SetContent(vx+len(lc), vy, 0, nil, tcell.StyleDefault.Normal())
}
