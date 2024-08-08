package readline

import (
	"bufio"
	"bytes"
	"fmt"
	"sync/atomic"

	"github.com/ergochat/readline/internal/platform"
	"github.com/ergochat/readline/internal/runes"
)

type AutoCompleter interface {
	// Readline will pass the whole line and current offset to it
	// Completer need to pass all the candidates, and how long they shared the same characters in line
	// Example:
	//   [go, git, git-shell, grep]
	//   Do("g", 1) => ["o", "it", "it-shell", "rep"], 1
	//   Do("gi", 2) => ["t", "t-shell"], 2
	//   Do("git", 3) => ["", "-shell"], 3
	Do(line []rune, pos int) (newLine [][]rune, length int)
}

type opCompleter struct {
	w  *terminal
	op *operation

	inCompleteMode atomic.Uint32 // this is read asynchronously from wrapWriter
	inSelectMode   bool

	candidate          [][]rune // list of candidates
	candidateSource    []rune   // buffer string when tab was pressed
	candidateOff       int      // num runes in common from buf where candidate start
	candidateChoise    int      // candidate chosen (-1 for nothing yet) also used for paging
	candidateColNum    int      // num columns candidates take 0..wraps, 1 col, 2 cols etc.
	candidateColWidth  int      // width of candidate columns
	linesAvail         int      // number of lines below the user's prompt which could be used for rendering the completion
	candidatePageStart []int    // idx of the candidate starting at each page, lazily computed when user navigates to new page
	curPage            int      // current page
}

func newOpCompleter(w *terminal, op *operation) *opCompleter {
	return &opCompleter{
		w:  w,
		op: op,
	}
}

func (o *opCompleter) doSelect() {
	if len(o.candidate) == 1 {
		o.op.buf.WriteRunes(o.candidate[0])
		o.ExitCompleteMode(false)
		return
	}
	o.nextCandidate(1)
	o.CompleteRefresh()
}

func (o *opCompleter) candidateChoiseWithinPage() int {
	return o.candidateChoise - o.candidatePageStart[o.curPage]
}

func (o *opCompleter) updateAbsoluteChoise(choiseWithinPage int) {
	o.candidateChoise = choiseWithinPage + o.candidatePageStart[o.curPage]
}

func (o *opCompleter) nextCandidate(i int) {
	// candidate index relative to the current page start
	idxWithinPage := o.candidateChoiseWithinPage()

	idxWithinPage += i
	idxWithinPage %= o.numCandidateCurPage()
	if idxWithinPage < 0 {
		idxWithinPage += o.numCandidateCurPage()
	}

	o.updateAbsoluteChoise(idxWithinPage)
}

func (o *opCompleter) nextLine() {
	colNum := 1
	if o.candidateColNum > 1 {
		colNum = o.candidateColNum
	}

	idxWithinPage := o.candidateChoiseWithinPage()

	idxWithinPage += colNum
	if idxWithinPage >= o.getMatrixSize() {
		idxWithinPage -= o.getMatrixSize()
	} else if idxWithinPage >= o.numCandidateCurPage() {
		idxWithinPage += colNum
		idxWithinPage -= o.getMatrixSize()
	}

	o.updateAbsoluteChoise(idxWithinPage)
}

func (o *opCompleter) prevLine() {
	colNum := 1
	if o.candidateColNum > 1 {
		colNum = o.candidateColNum
	}

	idxWithinPage := o.candidateChoiseWithinPage()

	idxWithinPage -= colNum
	if idxWithinPage < 0 {
		idxWithinPage += o.getMatrixSize()
		if idxWithinPage >= o.numCandidateCurPage() {
			idxWithinPage -= colNum
		}
	}

	o.updateAbsoluteChoise(idxWithinPage)
}

func (o *opCompleter) lineStart() {
	if o.candidateColNum > 1 {
		idxWithinPage := o.candidateChoiseWithinPage()
		lineOffset := idxWithinPage % o.candidateColNum
		idxWithinPage -= lineOffset
		o.updateAbsoluteChoise(idxWithinPage)
	}
}

func (o *opCompleter) lineEnd() {
	if o.candidateColNum > 1 {
		idxWithinPage := o.candidateChoiseWithinPage()
		offsetToLineEnd := o.candidateColNum - idxWithinPage%o.candidateColNum - 1
		idxWithinPage += offsetToLineEnd
		o.updateAbsoluteChoise(idxWithinPage)
		if o.candidateChoise >= len(o.candidate) {
			o.candidateChoise = len(o.candidate) - 1
		}
	}
}

func (o *opCompleter) nextPage() {
	// Check that this is not the last page already
	nextPageStart := o.candidatePageStart[o.curPage+1]
	if nextPageStart < len(o.candidate) {
		o.curPage += 1
		o.candidateChoise = o.candidatePageStart[o.curPage]
	}
}

func (o *opCompleter) prevPage() {
	if o.curPage > 0 {
		o.curPage -= 1
		o.candidateChoise = o.candidatePageStart[o.curPage]
	}
}

// OnComplete returns true if complete mode is available. Used to ring bell
// when tab pressed if cannot do complete for reason such as width unknown
// or no candidates available.
func (o *opCompleter) OnComplete() (ringBell bool) {
	tWidth, tHeight := o.w.GetWidthHeight()
	if tWidth == 0 || tHeight < 3 {
		return false
	}
	if o.IsInCompleteSelectMode() {
		o.doSelect()
		return true
	}

	buf := o.op.buf
	rs := buf.Runes()

	// If in complete mode and nothing else typed then we must be entering select mode
	if o.IsInCompleteMode() && o.candidateSource != nil && runes.Equal(rs, o.candidateSource) {
		if len(o.candidate) > 1 {
			same, size := runes.Aggregate(o.candidate)
			if size > 0 {
				buf.WriteRunes(same)
				o.ExitCompleteMode(false)
				return false // partial completion so ring the bell
			}
		}
		o.EnterCompleteSelectMode()
		o.doSelect()
		return true
	}

	newLines, offset := o.op.GetConfig().AutoComplete.Do(rs, buf.idx)
	if len(newLines) == 0 || (len(newLines) == 1 && len(newLines[0]) == 0) {
		o.ExitCompleteMode(false)
		return false // will ring bell on initial tab press
	}
	if o.candidateOff > offset {
		// part of buffer we are completing has changed. Example might be that we were completing "ls" and
		// user typed space so we are no longer completing "ls" but now we are completing an argument of
		// the ls command. Instead of continuing in complete mode, we exit.
		o.ExitCompleteMode(false)
		return true
	}
	o.candidateSource = rs

	// only Aggregate candidates in non-complete mode
	if !o.IsInCompleteMode() {
		if len(newLines) == 1 {
			// not yet in complete mode but only 1 candidate so complete it
			buf.WriteRunes(newLines[0])
			o.ExitCompleteMode(false)
			return true
		}

		// check if all candidates have common prefix and return it and its size
		same, size := runes.Aggregate(newLines)
		if size > 0 {
			buf.WriteRunes(same)
			o.ExitCompleteMode(false)
			return false // partial completion so ring the bell
		}
	}

	// otherwise, we just enter complete mode (which does a refresh)
	o.EnterCompleteMode(offset, newLines)
	return true
}

func (o *opCompleter) IsInCompleteSelectMode() bool {
	return o.inSelectMode
}

func (o *opCompleter) IsInCompleteMode() bool {
	return o.inCompleteMode.Load() == 1
}

func (o *opCompleter) HandleCompleteSelect(r rune) (stayInMode bool) {
	next := true
	switch r {
	case CharEnter, CharCtrlJ:
		next = false
		o.op.buf.WriteRunes(o.candidate[o.candidateChoise])
		o.ExitCompleteMode(false)
	case CharLineStart:
		o.lineStart()
	case CharLineEnd:
		o.lineEnd()
	case CharBackspace:
		o.ExitCompleteSelectMode()
		next = false
	case CharTab, CharForward:
		o.nextCandidate(1)
	case CharBell, CharInterrupt:
		o.ExitCompleteMode(true)
		next = false
	case CharNext:
		o.nextLine()
	case CharBackward, MetaShiftTab:
		o.nextCandidate(-1)
	case CharPrev:
		o.prevLine()
	case 'j', 'J':
		o.prevPage()
	case 'k', 'K':
		o.nextPage()
	default:
		next = false
		o.ExitCompleteSelectMode()
	}
	if next {
		o.CompleteRefresh()
		return true
	}
	return false
}

func (o *opCompleter) getMatrixSize() int {
	colNum := 1
	if o.candidateColNum > 1 {
		colNum = o.candidateColNum
	}
	line := o.getMatrixNumRows()
	return line * colNum
}

func (o *opCompleter) numCandidateCurPage() int {
	// Safety: we will always render the first page, and whenever we finished rendering page i,
	// we always populate o.candidatePageStart through at least i + 1, so when this is called, we
	// always know the start of the next page
	return o.candidatePageStart[o.curPage+1] - o.candidatePageStart[o.curPage]
}

// Get number of rows for a full matrix
func (o *opCompleter) getMatrixNumRows() int {
	candidateCurPage := o.numCandidateCurPage()
	// Normal case where there is no wrap
	if o.candidateColNum > 1 {
		numLine := candidateCurPage / o.candidateColNum
		if candidateCurPage%o.candidateColNum != 0 {
			numLine++
		}
		return numLine
	}

	// Now since there are wraps, each candidate will be put on its own line, so the number of lines is just the number of candidate
	return candidateCurPage
}

// setColumnInfo calculates column width and number of columns required
// to present the list of candidates on the terminal.
func (o *opCompleter) setColumnInfo() {
	same := o.op.buf.RuneSlice(-o.candidateOff)
	sameWidth := runes.WidthAll(same)

	colWidth := 0
	for _, c := range o.candidate {
		w := sameWidth + runes.WidthAll(c)
		if w > colWidth {
			colWidth = w
		}
	}
	colWidth++ // whitespace between cols

	tWidth, _ := o.w.GetWidthHeight()

	// -1 to avoid end of line issues
	width := tWidth - 1
	colNum := width / colWidth
	if colNum != 0 {
		colWidth += (width - (colWidth * colNum)) / colNum
	}

	o.candidateColNum = colNum
	o.candidateColWidth = colWidth
}

// CompleteRefresh is used for completemode and selectmode
func (o *opCompleter) CompleteRefresh() {
	if !o.IsInCompleteMode() {
		return
	}

	buf := bufio.NewWriter(o.w)
	// calculate num lines from cursor pos to where choices should be written
	lineCnt := o.op.buf.CursorLineCount()
	buf.Write(bytes.Repeat([]byte("\n"), lineCnt)) // move down from cursor to start of candidates
	buf.WriteString("\033[J")

	same := o.op.buf.RuneSlice(-o.candidateOff)
	tWidth, _ := o.w.GetWidthHeight()

	colIdx := 0
	lines := 0
	sameWidth := runes.WidthAll(same)

	idx := o.candidatePageStart[o.curPage]
	for ; idx < len(o.candidate); idx++ {
		c := o.candidate[idx]
		inSelect := idx == o.candidateChoise && o.IsInCompleteSelectMode()
		cWidth := sameWidth + runes.WidthAll(c)
		cLines := 1
		if tWidth > 0 {
			sWidth := 0
			if platform.IsWindows && inSelect {
				sWidth = 1 // adjust for hightlighting on Windows
			}
			cLines = (cWidth + sWidth) / tWidth
			if (cWidth+sWidth)%tWidth > 0 {
				cLines++
			}
		}

		// If the candidate is written, it will overflow the current page, so
		// we know that it is the start of the next page
		if colIdx+1 >= o.candidateColNum && lines+cLines > o.linesAvail {
			if o.curPage == len(o.candidatePageStart)-1 {
				o.candidatePageStart = append(o.candidatePageStart, idx)
			}
			break
		}

		if lines > 0 && colIdx == 0 {
			// After line 1, if we're printing to the first column
			// goto a new line. We do it here, instead of at the end
			// of the loop, to avoid the last \n taking up a blank
			// line at the end and stealing realestate.
			buf.WriteString("\n")
		}

		if inSelect {
			buf.WriteString("\033[30;47m")
		}

		buf.WriteString(string(same))
		buf.WriteString(string(c))
		if o.candidateColNum >= 1 {
			// only output spaces between columns if everything fits
			buf.Write(bytes.Repeat([]byte(" "), o.candidateColWidth-cWidth))
		}

		if inSelect {
			buf.WriteString("\033[0m")
		}

		colIdx++
		if colIdx >= o.candidateColNum {
			lines += cLines
			colIdx = 0
			if platform.IsWindows {
				// Windows EOL edge-case.
				buf.WriteString("\b")
			}
		}
	}

	if idx == len(o.candidate) {
		// This is the last page
		o.candidatePageStart = append(o.candidatePageStart, len(o.candidate))
	}

	if colIdx > 0 {
		lines++ // mid-line so count it.
	}

	// Show guidance if there are more than one page
	if idx != len(o.candidate) || o.curPage > 0 {
		buf.WriteString("\n-- (j: prev page) (k: next page) --")
		lines++
	}

	// wrote out choices over "lines", move back to cursor (positioned at index)
	fmt.Fprintf(buf, "\033[%dA", lines)
	buf.Write(o.op.buf.getBackspaceSequence())
	buf.Flush()
}

func (o *opCompleter) EnterCompleteSelectMode() {
	o.inSelectMode = true
	o.candidateChoise = -1
}

func (o *opCompleter) EnterCompleteMode(offset int, candidate [][]rune) {
	o.inCompleteMode.Store(1)
	o.candidate = candidate
	o.candidateOff = offset
	o.setColumnInfo()
	o.initPage()
	o.CompleteRefresh()
}

func (o *opCompleter) initPage() {
	_, tHeight := o.w.GetWidthHeight()
	buflineCnt := o.op.buf.LineCount()      // lines taken by buffer content
	o.linesAvail = tHeight - buflineCnt - 2 // lines available without scrolling buffer off screen, reserve some space for the guidance
	o.candidatePageStart = []int{0}         // first page always start at 0
	o.curPage = 0
}

func (o *opCompleter) ExitCompleteSelectMode() {
	o.inSelectMode = false
	o.candidateChoise = -1
}

func (o *opCompleter) ExitCompleteMode(revent bool) {
	o.inCompleteMode.Store(0)
	o.candidate = nil
	o.candidateOff = -1
	o.candidateSource = nil
	o.ExitCompleteSelectMode()
}
