package readline

import (
	"container/list"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"

	"github.com/ergochat/readline/internal/term"
)

const (
	charLineStart = 1
	charBackward  = 2
	charInterrupt = 3
	charEOT       = 4
	charLineEnd   = 5
	charForward   = 6
	charBell      = 7
	charCtrlH     = 8
	charTab       = 9
	charCtrlJ     = 10
	charKill      = 11
	charCtrlL     = 12
	charEnter     = 13
	charNext      = 14
	charPrev      = 16
	charBckSearch = 18
	charFwdSearch = 19
	charTranspose = 20
	charCtrlU     = 21
	charCtrlW     = 23
	charCtrlY     = 25
	charCtrlZ     = 26
	charEsc       = 27
	charCtrl_     = 31
	charO         = 79
	charEscapeEx  = 91
	charBackspace = 127
)

const (
	metaBackward rune = -iota - 1
	metaForward
	metaDelete
	metaBackspace
	metaTranspose
	metaShiftTab
	metaDeleteKey
)

type rawModeHandler struct {
	sync.Mutex
	state *term.State
}

func (r *rawModeHandler) Enter() (err error) {
	r.Lock()
	defer r.Unlock()
	r.state, err = term.MakeRaw(int(syscall.Stdin))
	return err
}

func (r *rawModeHandler) Exit() error {
	r.Lock()
	defer r.Unlock()
	if r.state == nil {
		return nil
	}
	err := term.Restore(int(syscall.Stdin), r.state)
	if err == nil {
		r.state = nil
	}
	return err
}

func clearScreen(w io.Writer) error {
	_, err := w.Write([]byte("\x1b[H\x1b[J"))
	return err
}

// -----------------------------------------------------------------------------

// print a linked list to Debug()
func debugList(l *list.List) {
	idx := 0
	for e := l.Front(); e != nil; e = e.Next() {
		debugPrint("%d %+v", idx, e.Value)
		idx++
	}
}

// append log info to another file
func debugPrint(fmtStr string, o ...interface{}) {
	f, _ := os.OpenFile("debug.tmp", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	fmt.Fprintf(f, fmtStr, o...)
	fmt.Fprintln(f)
	f.Close()
}
