package readline

import (
	"bufio"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

type Terminal struct {
	cfg        atomic.Pointer[Config]
	dimensions atomic.Pointer[termDimensions]
	closeOnce  sync.Once
	closeErr   error
	outchan    chan rune
	stopChan   chan struct{}
	kickChan   chan struct{}
	sleeping   int32

	sizeChan chan string
}

// termDimensions stores the terminal width and height (-1 means unknown)
type termDimensions struct {
	width  int
	height int
}

func NewTerminal(cfg *Config) (*Terminal, error) {
	if cfg.useInteractive() {
		if ansiErr := enableANSI(); ansiErr != nil {
			return nil, fmt.Errorf("Could not enable ANSI escapes: %w", ansiErr)
		}
	}
	t := &Terminal{
		kickChan: make(chan struct{}, 1),
		outchan:  make(chan rune),
		stopChan: make(chan struct{}, 1),
		sizeChan: make(chan string, 1),
	}
	t.cfg.Store(cfg)
	// Get and cache the current terminal size.
	t.OnSizeChange()

	go t.ioloop()
	return t, nil
}

// SleepToResume will sleep myself, and return only if I'm resumed.
func (t *Terminal) SleepToResume() {
	if !atomic.CompareAndSwapInt32(&t.sleeping, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&t.sleeping, 0)

	t.ExitRawMode()
	ch := WaitForResume()
	SuspendMe()
	<-ch
	t.EnterRawMode()
}

func (t *Terminal) EnterRawMode() (err error) {
	return t.cfg.Load().FuncMakeRaw()
}

func (t *Terminal) ExitRawMode() (err error) {
	return t.cfg.Load().FuncExitRaw()
}

func (t *Terminal) Write(b []byte) (int, error) {
	return t.cfg.Load().Stdout.Write(b)
}

// WriteStdin prefill the next Stdin fetch
// Next time you call ReadLine() this value will be writen before the user input
func (t *Terminal) WriteStdin(b []byte) (int, error) {
	return t.cfg.Load().StdinWriter.Write(b)
}

func (t *Terminal) GetOffset(f func(offset string)) {
	go func() {
		f(<-t.sizeChan)
	}()
	SendCursorPosition(t)
}

// return rune(0) if meet EOF
func (t *Terminal) GetRune() rune {
	select {
	case ch := <-t.outchan:
		return ch
	case <-t.stopChan:
		return 0
	}
}

func (t *Terminal) KickRead() {
	select {
	case t.kickChan <- struct{}{}:
	default:
	}
}

func (t *Terminal) ioloop() {
	var (
		isEscape       bool
		isEscapeEx     bool
		isEscapeSS3    bool
		expectNextChar bool
	)

	buf := bufio.NewReader(t.cfg.Load().Stdin)
	for {
		if !expectNextChar {
			select {
			case <-t.kickChan:
			case <-t.stopChan:
				return
			}
		}
		expectNextChar = false
		r, _, err := buf.ReadRune()
		if err != nil {
			if strings.Contains(err.Error(), "interrupted system call") {
				expectNextChar = true
				continue
			}
			break
		}

		if isEscape {
			isEscape = false
			if r == CharEscapeEx {
				// ^][
				expectNextChar = true
				isEscapeEx = true
				continue
			} else if r == CharO {
				// ^]O
				expectNextChar = true
				isEscapeSS3 = true
				continue
			}
			r = escapeKey(r, buf)
		} else if isEscapeEx {
			isEscapeEx = false
			if key := readEscKey(r, buf); key != nil {
				r = escapeExKey(key)
				// offset
				if key.typ == 'R' {
					if _, _, ok := key.Get2(); ok {
						select {
						case t.sizeChan <- key.attr:
						default:
						}
					}
					expectNextChar = true
					continue
				}
			}
			if r == 0 {
				expectNextChar = true
				continue
			}
		} else if isEscapeSS3 {
			isEscapeSS3 = false
			if key := readEscKey(r, buf); key != nil {
				r = escapeSS3Key(key)
			}
			if r == 0 {
				expectNextChar = true
				continue
			}
		}

		expectNextChar = true
		switch r {
		case CharEsc:
			if t.cfg.Load().VimMode {
				t.outchan <- r
				break
			}
			isEscape = true
		case CharInterrupt, CharEnter, CharCtrlJ, CharDelete:
			expectNextChar = false
			fallthrough
		default:
			t.outchan <- r
		}
	}
}

func (t *Terminal) Bell() {
	fmt.Fprintf(t, "%c", CharBell)
}

func (t *Terminal) Close() error {
	t.closeOnce.Do(func() {
		close(t.stopChan)
		t.closeErr = t.ExitRawMode()
	})
	return t.closeErr
}

func (t *Terminal) setConfig(c *Config) error {
	t.cfg.Store(c)
	return nil
}

// OnSizeChange gets the current terminal size and caches it
func (t *Terminal) OnSizeChange() {
	cfg := t.cfg.Load()
	width, height := cfg.FuncGetSize()
	t.dimensions.Store(&termDimensions{
		width:  width,
		height: height,
	})
}

// GetWidthHeight returns the cached width, height values from the terminal
func (t *Terminal) GetWidthHeight() (width, height int) {
	dimensions := t.dimensions.Load()
	return dimensions.width, dimensions.height
}
