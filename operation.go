package readline

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"

	"github.com/ergochat/readline/internal/platform"
	"github.com/ergochat/readline/internal/runes"
)

var (
	ErrInterrupt = errors.New("Interrupt")
)

type operation struct {
	m       sync.Mutex
	t       *terminal
	buf     *runeBuffer
	wrapOut atomic.Pointer[wrapWriter]
	wrapErr atomic.Pointer[wrapWriter]

	isPrompting bool // true when prompt written and waiting for input

	history   *opHistory
	search    *opSearch
	completer *opCompleter
	vim       *opVim
}

func (o *operation) SetBuffer(what string) {
	o.buf.SetNoRefresh([]rune(what))
}

type wrapWriter struct {
	o      *operation
	target io.Writer
}

func (w *wrapWriter) Write(b []byte) (int, error) {
	return w.o.write(w.target, b)
}

func (o *operation) write(target io.Writer, b []byte) (int, error) {
	o.m.Lock()
	defer o.m.Unlock()

	if !o.isPrompting {
		return target.Write(b)
	}

	var (
		n   int
		err error
	)
	o.buf.Refresh(func() {
		n, err = target.Write(b)
		// Adjust the prompt start position by b
		rout := runes.ColorFilter([]rune(string(b[:])))
		tWidth, _ := o.t.GetWidthHeight()
		sp := runes.SplitByLine(rout, []rune{}, o.buf.ppos, tWidth, 1)
		if len(sp) > 1 {
			o.buf.ppos = len(sp[len(sp)-1])
		} else {
			o.buf.ppos += len(rout)
		}
	})

	if o.search.IsSearchMode() {
		o.search.SearchRefresh(-1)
	}
	if o.completer.IsInCompleteMode() {
		o.completer.CompleteRefresh()
	}
	return n, err
}

func newOperation(t *terminal) *operation {
	cfg := t.GetConfig()
	op := &operation{
		t:   t,
		buf: newRuneBuffer(t),
	}
	op.SetConfig(cfg)
	op.vim = newVimMode(op)
	op.completer = newOpCompleter(op.buf.w, op)
	cfg.FuncOnWidthChanged(t.OnSizeChange)
	return op
}

func (o *operation) GetConfig() *Config {
	return o.t.GetConfig()
}

func (o *operation) readline(deadline chan struct{}) ([]rune, error) {
	for {
		keepInSearchMode := false
		keepInCompleteMode := false
		r, err := o.t.GetRune(deadline)

		if err == nil && o.GetConfig().FuncFilterInputRune != nil {
			var process bool
			r, process = o.GetConfig().FuncFilterInputRune(r)
			if !process {
				o.buf.Refresh(nil) // to refresh the line
				continue           // ignore this rune
			}
		}

		if err == io.EOF {
			if o.buf.Len() == 0 {
				o.buf.Clean()
				return nil, io.EOF
			} else {
				// if stdin got io.EOF and there is something left in buffer,
				// let's flush them by sending CharEnter.
				// And we will got io.EOF int next loop.
				r = CharEnter
			}
		} else if err != nil {
			return nil, err
		}
		isUpdateHistory := true

		if o.completer.IsInPagerMode() {
			keepInCompleteMode = o.completer.HandlePagerMode(r)
			if !keepInCompleteMode {
				o.buf.Refresh(nil)
			}
			continue
		}

		if o.completer.IsInCompleteSelectMode() {
			keepInCompleteMode = o.completer.HandleCompleteSelect(r)
			if keepInCompleteMode {
				continue
			}

			o.buf.Refresh(nil)
			switch r {
			case CharEnter, CharCtrlJ:
				o.history.Update(o.buf.Runes(), false)
				fallthrough
			case CharInterrupt:
				fallthrough
			case CharBell:
				continue
			}
		}

		if o.vim.IsEnableVimMode() {
			r = o.vim.HandleVim(r, func() rune {
				r, err := o.t.GetRune(deadline)
				if err == nil {
					return r
				} else {
					return 0
				}
			})
			if r == 0 {
				continue
			}
		}

		var result []rune

		switch r {
		case CharBell:
			if o.search.IsSearchMode() {
				o.search.ExitSearchMode(true)
				o.buf.Refresh(nil)
			}
			if o.completer.IsInCompleteMode() {
				o.completer.ExitCompleteMode(true)
				o.buf.Refresh(nil)
			}
		case CharTab:
			if o.GetConfig().AutoComplete == nil {
				o.t.Bell()
				break
			}
			if o.completer.OnComplete() {
				if o.completer.IsInCompleteMode() {
					keepInCompleteMode = true
					continue // redraw is done, loop
				}
			} else {
				o.t.Bell()
			}
			o.buf.Refresh(nil)
		case CharBckSearch:
			if !o.search.SearchMode(S_DIR_BCK) {
				o.t.Bell()
				break
			}
			keepInSearchMode = true
		case CharCtrlU:
			o.buf.KillFront()
		case CharFwdSearch:
			if !o.search.SearchMode(S_DIR_FWD) {
				o.t.Bell()
				break
			}
			keepInSearchMode = true
		case CharKill:
			o.buf.Kill()
			keepInCompleteMode = true
		case MetaForward:
			o.buf.MoveToNextWord()
		case CharTranspose:
			o.buf.Transpose()
		case MetaBackward:
			o.buf.MoveToPrevWord()
		case MetaDelete:
			o.buf.DeleteWord()
		case CharLineStart:
			o.buf.MoveToLineStart()
		case CharLineEnd:
			o.buf.MoveToLineEnd()
		case CharBackspace, CharCtrlH:
			if o.search.IsSearchMode() {
				o.search.SearchBackspace()
				keepInSearchMode = true
				break
			}

			if o.buf.Len() == 0 {
				o.t.Bell()
				break
			}
			o.buf.Backspace()
		case CharCtrlZ:
			if !platform.IsWindows {
				o.buf.Clean()
				o.t.SleepToResume()
				o.Refresh()
			}
		case CharCtrlL:
			platform.ClearScreen(o.t)
			o.buf.SetOffset(cursorPosition{1, 1})
			o.Refresh()
		case MetaBackspace, CharCtrlW:
			o.buf.BackEscapeWord()
		case MetaShiftTab:
			// no-op
		case CharCtrlY:
			o.buf.Yank()
		case CharEnter, CharCtrlJ:
			if o.search.IsSearchMode() {
				o.search.ExitSearchMode(false)
			}
			if o.completer.IsInCompleteMode() {
				o.completer.ExitCompleteMode(true)
				o.buf.Refresh(nil)
			}
			o.buf.MoveToLineEnd()
			var data []rune
			if !o.GetConfig().UniqueEditLine {
				o.buf.WriteRune('\n')
				data = o.buf.Reset()
				data = data[:len(data)-1] // trim \n
			} else {
				o.buf.Clean()
				data = o.buf.Reset()
			}
			result = data
			if !o.GetConfig().DisableAutoSaveHistory {
				// ignore IO error
				_ = o.history.New(data)
			} else {
				isUpdateHistory = false
			}
		case CharBackward:
			o.buf.MoveBackward()
		case CharForward:
			o.buf.MoveForward()
		case CharPrev:
			buf := o.history.Prev()
			if buf != nil {
				o.buf.Set(buf)
			} else {
				o.t.Bell()
			}
		case CharNext:
			buf, ok := o.history.Next()
			if ok {
				o.buf.Set(buf)
			} else {
				o.t.Bell()
			}
		case MetaDeleteKey, CharEOT:
			// on Delete key or Ctrl-D, attempt to delete a character:
			if o.buf.Len() > 0 || !o.IsNormalMode() {
				if !o.buf.Delete() {
					o.t.Bell()
				}
				break
			}
			if r != CharEOT {
				break
			}
			// Ctrl-D on an empty buffer: treated as EOF
			if !o.GetConfig().UniqueEditLine {
				o.buf.WriteString(o.GetConfig().EOFPrompt + "\n")
			}
			o.buf.Reset()
			isUpdateHistory = false
			o.history.Revert()
			if o.GetConfig().UniqueEditLine {
				o.buf.Clean()
			}
			return nil, io.EOF
		case CharInterrupt:
			if o.search.IsSearchMode() {
				o.search.ExitSearchMode(true)
				break
			}
			if o.completer.IsInCompleteMode() {
				o.completer.ExitCompleteMode(true)
				o.buf.Refresh(nil)
				break
			}
			o.buf.MoveToLineEnd()
			o.buf.Refresh(nil)
			hint := o.GetConfig().InterruptPrompt + "\n"
			if !o.GetConfig().UniqueEditLine {
				o.buf.WriteString(hint)
			}
			remain := o.buf.Reset()
			if !o.GetConfig().UniqueEditLine {
				remain = remain[:len(remain)-len([]rune(hint))]
			}
			isUpdateHistory = false
			o.history.Revert()
			return nil, ErrInterrupt
		default:
			if o.search.IsSearchMode() {
				o.search.SearchChar(r)
				keepInSearchMode = true
				break
			}
			o.buf.WriteRune(r)
			if o.completer.IsInCompleteMode() {
				o.completer.OnComplete()
				if o.completer.IsInCompleteMode() {
					keepInCompleteMode = true
				} else {
					o.buf.Refresh(nil)
				}
			}
		}

		listener := o.GetConfig().Listener
		if listener != nil {
			newLine, newPos, ok := listener.OnChange(o.buf.Runes(), o.buf.Pos(), r)
			if ok {
				o.buf.SetWithIdx(newPos, newLine)
			}
		}

		o.m.Lock()
		if !keepInSearchMode && o.search.IsSearchMode() {
			o.search.ExitSearchMode(false)
			o.buf.Refresh(nil)
		} else if o.completer.IsInCompleteMode() {
			if !keepInCompleteMode {
				o.completer.ExitCompleteMode(false)
				o.refresh()
			} else {
				o.buf.Refresh(nil)
				o.completer.CompleteRefresh()
			}
		}
		if isUpdateHistory && !o.search.IsSearchMode() {
			// it will cause null history
			o.history.Update(o.buf.Runes(), false)
		}
		o.m.Unlock()

		if result != nil {
			return result, nil
		}
	}
}

func (o *operation) Stderr() io.Writer {
	return o.wrapErr.Load()
}

func (o *operation) Stdout() io.Writer {
	return o.wrapOut.Load()
}

func (o *operation) String() (string, error) {
	r, err := o.Runes()
	return string(r), err
}

func (o *operation) Runes() ([]rune, error) {
	o.t.EnterRawMode()
	defer o.t.ExitRawMode()

	listener := o.GetConfig().Listener
	if listener != nil {
		listener.OnChange(nil, 0, 0)
	}

	// Before writing the prompt and starting to read, get a lock
	// so we don't race with wrapWriter trying to write and refresh.
	o.m.Lock()
	o.isPrompting = true
	// Query cursor position before printing the prompt as there
	// may be existing text on the same line that ideally we don't
	// want to overwrite and cause prompt to jump left.
	o.getAndSetOffset(nil)
	o.buf.Print() // print prompt & buffer contents
	// Prompt written safely, unlock until read completes and then
	// lock again to unset.
	o.m.Unlock()

	defer func() {
		o.m.Lock()
		o.isPrompting = false
		o.buf.SetOffset(cursorPosition{1, 1})
		o.m.Unlock()
	}()

	return o.readline(nil)
}

func (o *operation) getAndSetOffset(deadline chan struct{}) {
	if !o.GetConfig().isInteractive {
		return
	}

	// Handle lineedge cases where existing text before before
	// the prompt is printed would leave us at the right edge of
	// the screen but the next character would actually be printed
	// at the beginning of the next line.
	// TODO ???
	if !platform.IsWindows {
		o.t.Write([]byte(" \b"))
	}

	if offset, err := o.t.GetCursorPosition(deadline); err == nil {
		o.buf.SetOffset(offset)
	}
}

func (o *operation) GenPasswordConfig() *Config {
	baseConfig := o.GetConfig()
	return &Config{
		EnableMask:      true,
		InterruptPrompt: "\n",
		EOFPrompt:       "\n",
		HistoryLimit:    -1,

		Stdin:  baseConfig.Stdin,
		Stdout: baseConfig.Stdout,
		Stderr: baseConfig.Stderr,

		FuncIsTerminal:      baseConfig.FuncIsTerminal,
		FuncMakeRaw:         baseConfig.FuncMakeRaw,
		FuncExitRaw:         baseConfig.FuncExitRaw,
		FuncOnWidthChanged:  baseConfig.FuncOnWidthChanged,
		ForceUseInteractive: baseConfig.ForceUseInteractive,
	}
}

func (o *operation) PasswordWithConfig(cfg *Config) ([]byte, error) {
	backupCfg, err := o.SetConfig(cfg)
	if err != nil {
		return nil, err
	}
	defer func() {
		o.SetConfig(backupCfg)
	}()
	return o.Slice()
}

func (o *operation) Password(prompt string) ([]byte, error) {
	cfg := o.GenPasswordConfig()
	cfg.Prompt = prompt
	return o.PasswordWithConfig(cfg)
}

func (o *operation) SetTitle(t string) {
	o.t.Write([]byte("\033[2;" + t + "\007"))
}

func (o *operation) Slice() ([]byte, error) {
	r, err := o.Runes()
	if err != nil {
		return nil, err
	}
	return []byte(string(r)), nil
}

func (o *operation) Close() {
	o.history.Close()
}

func (o *operation) IsNormalMode() bool {
	return !o.completer.IsInCompleteMode() && !o.search.IsSearchMode()
}

func (op *operation) SetConfig(cfg *Config) (*Config, error) {
	op.m.Lock()
	defer op.m.Unlock()
	old := op.t.GetConfig()
	if err := cfg.init(); err != nil {
		return old, err
	}

	// install the config in its canonical location (inside terminal):
	op.t.SetConfig(cfg)

	op.wrapOut.Store(&wrapWriter{target: cfg.Stdout, o: op})
	op.wrapErr.Store(&wrapWriter{target: cfg.Stderr, o: op})

	if op.history == nil {
		op.history = newOpHistory(op)
	}
	if op.search == nil {
		op.search = newOpSearch(op.buf.w, op.buf, op.history)
	}

	if cfg.AutoComplete != nil && op.completer == nil {
		op.completer = newOpCompleter(op.buf.w, op)
	}

	return old, nil
}

func (o *operation) ResetHistory() {
	o.history.Reset()
}

// if err is not nil, it just mean it fail to write to file
// other things goes fine.
func (o *operation) SaveHistory(content string) error {
	return o.history.New([]rune(content))
}

func (o *operation) Refresh() {
	o.m.Lock()
	defer o.m.Unlock()
	o.refresh()
}

func (o *operation) refresh() {
	if o.isPrompting {
		o.buf.Refresh(nil)
	}
}

func (o *operation) Clean() {
	o.buf.Clean()
}

func FuncListener(f func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool)) Listener {
	return &DumpListener{f: f}
}

type DumpListener struct {
	f func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool)
}

func (d *DumpListener) OnChange(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
	return d.f(line, pos, key)
}

type Listener interface {
	OnChange(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool)
}

type Painter interface {
	Paint(line []rune, pos int) []rune
}

type defaultPainter struct{}

func (p *defaultPainter) Paint(line []rune, _ int) []rune {
	return line
}
