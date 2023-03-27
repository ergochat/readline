package readline

import (
	"container/list"
	"fmt"
	"os"
	"sync"

	"github.com/ergochat/readline/internal/platform"
	"github.com/ergochat/readline/internal/term"
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

type rawModeHandler struct {
	sync.Mutex
	state *term.State
}

func (r *rawModeHandler) Enter() (err error) {
	r.Lock()
	defer r.Unlock()
	r.state, err = term.MakeRaw(platform.GetStdin())
	return err
}

func (r *rawModeHandler) Exit() error {
	r.Lock()
	defer r.Unlock()
	if r.state == nil {
		return nil
	}
	err := term.Restore(platform.GetStdin(), r.state)
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
