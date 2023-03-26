package readline

import (
	"container/list"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/ergochat/readline/internal/runes"
	"github.com/ergochat/readline/internal/term"
)

var (
	isWindows = false
)

const (
	CharLineStart = 1
	CharBackward  = 2
	CharInterrupt = 3
	CharDelete    = 4
	CharLineEnd   = 5
	CharForward   = 6
	CharBell      = 7
	CharCtrlH     = 8
	CharTab       = 9
	CharCtrlJ     = 10
	CharKill      = 11
	CharCtrlL     = 12
	CharEnter     = 13
	CharNext      = 14
	CharPrev      = 16
	CharBckSearch = 18
	CharFwdSearch = 19
	CharTranspose = 20
	CharCtrlU     = 21
	CharCtrlW     = 23
	CharCtrlY     = 25
	CharCtrlZ     = 26
	CharEsc       = 27
	CharO         = 79
	CharEscapeEx  = 91
	CharBackspace = 127
)

const (
	MetaBackward rune = -iota - 1
	MetaForward
	MetaDelete
	MetaBackspace
	MetaTranspose
	MetaShiftTab
)

// WaitForResume need to call before current process got suspend.
// It will run a ticker until a long duration is occurs,
// which means this process is resumed.
func WaitForResume() chan struct{} {
	ch := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		t := time.Now()
		wg.Done()
		for {
			now := <-ticker.C
			if now.Sub(t) > 100*time.Millisecond {
				break
			}
			t = now
		}
		ticker.Stop()
		ch <- struct{}{}
	}()
	wg.Wait()
	return ch
}

func IsPrintable(key rune) bool {
	isInSurrogateArea := key >= 0xd800 && key <= 0xdbff
	return key >= 32 && !isInSurrogateArea
}

// split prompt + runes into lines by screenwidth starting from an offset.
// the prompt should be filtered before passing to only its display runes.
// if you know the width of the next character, pass it in as it is used
// to decide if we generate an extra empty rune array to show next is new
// line.
func SplitByLine(prompt, rs []rune, offset, screenWidth, nextWidth int) [][]rune {
	ret := make([][]rune, 0)
	prs := append(prompt, rs...)
	si := 0
	currentWidth := offset
	for i, r := range prs {
		w := runes.Width(r)
		if r == '\n' {
			ret = append(ret, prs[si:i+1])
			si = i + 1
			currentWidth = 0
		} else if currentWidth + w > screenWidth {
			ret = append(ret, prs[si:i])
			si = i
			currentWidth = 0
		}
		currentWidth += w
	}
	ret = append(ret, prs[si:len(prs)])
	if currentWidth + nextWidth > screenWidth {
		ret = append(ret, []rune{})
	}
	return ret
}

// calculate how many lines for N character
func LineCount(screenWidth, w int) int {
	r := w / screenWidth
	if w%screenWidth != 0 {
		r++
	}
	return r
}

func IsWordBreak(i rune) bool {
	switch {
	case i >= 'a' && i <= 'z':
	case i >= 'A' && i <= 'Z':
	case i >= '0' && i <= '9':
	default:
		return true
	}
	return false
}

func GetInt(s []string, def int) int {
	if len(s) == 0 {
		return def
	}
	c, err := strconv.Atoi(s[0])
	if err != nil {
		return def
	}
	return c
}

type rawModeHandler struct {
	sync.Mutex
	state *term.State
}

func (r *rawModeHandler) Enter() (err error) {
	r.Lock()
	defer r.Unlock()
	r.state, err = term.MakeRaw(GetStdin())
	return err
}

func (r *rawModeHandler) Exit() error {
	r.Lock()
	defer r.Unlock()
	if r.state == nil {
		return nil
	}
	err := term.Restore(GetStdin(), r.state)
	if err == nil {
		r.state = nil
	}
	return err
}

// -----------------------------------------------------------------------------

// print a linked list to Debug()
func debugList(l *list.List) {
	idx := 0
	for e := l.Front(); e != nil; e = e.Next() {
		debugPrint(idx, fmt.Sprintf("%+v", e.Value))
		idx++
	}
}

// append log info to another file
func debugPrint(o ...interface{}) {
	f, _ := os.OpenFile("debug.tmp", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	fmt.Fprintln(f, o...)
	f.Close()
}
