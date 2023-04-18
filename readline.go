package readline

import (
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/ergochat/readline/internal/platform"
)

type Instance struct {
	terminal  *terminal
	operation *operation

	closeOnce sync.Once
	closeErr  error
}

type Config struct {
	// prompt supports ANSI escape sequence, so we can color some characters even in windows
	Prompt string

	// readline will persist historys to file where HistoryFile specified
	HistoryFile string
	// specify the max length of historys, it's 500 by default, set it to -1 to disable history
	HistoryLimit           int
	DisableAutoSaveHistory bool
	// enable case-insensitive history searching
	HistorySearchFold bool

	// AutoCompleter will called once user press TAB
	AutoComplete AutoCompleter

	// Any key press will pass to Listener
	// NOTE: Listener will be triggered by (nil, 0, 0) immediately
	Listener Listener

	Painter Painter

	// If VimMode is true, readline will in vim.insert mode by default
	VimMode bool

	InterruptPrompt string
	EOFPrompt       string

	FuncGetWidth func() int
	// Function that returns width, height of the terminal or -1,-1 if unknown
	FuncGetSize func() (width int, height int)

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	EnableMask bool
	MaskRune   rune

	// erase the editing line after user submited it
	// it use in IM usually.
	UniqueEditLine bool

	// filter input runes (may be used to disable CtrlZ or for translating some keys to different actions)
	// -> output = new (translated) rune and true/false if continue with processing this one
	FuncFilterInputRune func(rune) (rune, bool)

	// force use interactive even stdout is not a tty
	FuncIsTerminal      func() bool
	FuncMakeRaw         func() error
	FuncExitRaw         func() error
	FuncOnWidthChanged  func(func())
	ForceUseInteractive bool

	// private fields
	inited        bool
	isInteractive bool
}

func (c *Config) init() error {
	if c.inited {
		return nil
	}
	c.inited = true
	if c.Stdin == nil {
		c.Stdin = os.Stdin
	}

	if c.Stdout == nil {
		c.Stdout = os.Stdout
	}
	if c.Stderr == nil {
		c.Stderr = os.Stderr
	}
	if c.HistoryLimit == 0 {
		c.HistoryLimit = 500
	}

	if c.InterruptPrompt == "" {
		c.InterruptPrompt = "^C"
	} else if c.InterruptPrompt == "\n" {
		c.InterruptPrompt = ""
	}
	if c.EOFPrompt == "" {
		c.EOFPrompt = "^D"
	} else if c.EOFPrompt == "\n" {
		c.EOFPrompt = ""
	}

	if c.AutoComplete == nil {
		c.AutoComplete = &TabCompleter{}
	}
	if c.FuncGetWidth == nil {
		c.FuncGetWidth = platform.GetScreenWidth
	}
	if c.FuncGetSize == nil {
		c.FuncGetSize = platform.GetScreenSize
	}
	if c.FuncIsTerminal == nil {
		c.FuncIsTerminal = platform.DefaultIsTerminal
	}
	rm := new(rawModeHandler)
	if c.FuncMakeRaw == nil {
		c.FuncMakeRaw = rm.Enter
	}
	if c.FuncExitRaw == nil {
		c.FuncExitRaw = rm.Exit
	}
	if c.FuncOnWidthChanged == nil {
		c.FuncOnWidthChanged = platform.DefaultOnSizeChanged
	}
	if c.Painter == nil {
		c.Painter = &defaultPainter{}
	}

	c.isInteractive = c.ForceUseInteractive || c.FuncIsTerminal()

	return nil
}

func (c *Config) SetListener(f func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool)) {
	c.Listener = FuncListener(f)
}

// NewFromConfig creates a readline instance from the specified configuration.
func NewFromConfig(cfg *Config) (*Instance, error) {
	if err := cfg.init(); err != nil {
		return nil, err
	}
	t, err := newTerminal(cfg)
	if err != nil {
		return nil, err
	}
	o := newOperation(t)
	return &Instance{
		terminal:  t,
		operation: o,
	}, nil
}

// NewEx is an alias for NewFromConfig, for compatibility.
var NewEx = NewFromConfig

// New creates a readline instance with default configuration.
func New(prompt string) (*Instance, error) {
	return NewFromConfig(&Config{Prompt: prompt})
}

func (i *Instance) ResetHistory() {
	i.operation.ResetHistory()
}

func (i *Instance) SetPrompt(s string) {
	cfg := i.GetConfig()
	cfg.Prompt = s
	i.SetConfig(cfg)
}

func (i *Instance) SetMaskRune(r rune) {
	cfg := i.GetConfig()
	cfg.MaskRune = r
	i.SetConfig(cfg)
}

// readline will refresh automatic when write through Stdout()
func (i *Instance) Stdout() io.Writer {
	return i.operation.Stdout()
}

// readline will refresh automatic when write through Stdout()
func (i *Instance) Stderr() io.Writer {
	return i.operation.Stderr()
}

// switch VimMode in runtime
func (i *Instance) SetVimMode(on bool) {
	cfg := i.GetConfig()
	cfg.VimMode = on
	i.SetConfig(cfg)
}

func (i *Instance) IsVimMode() bool {
	return i.operation.vim.IsEnableVimMode()
}

func (i *Instance) GenPasswordConfig() *Config {
	return i.operation.GenPasswordConfig()
}

// we can generate a config by `i.GenPasswordConfig()`
func (i *Instance) ReadPasswordWithConfig(cfg *Config) ([]byte, error) {
	return i.operation.PasswordWithConfig(cfg)
}

func (i *Instance) ReadPassword(prompt string) ([]byte, error) {
	return i.operation.Password(prompt)
}

// err is one of (nil, io.EOF, readline.ErrInterrupt)
func (i *Instance) Readline() (string, error) {
	return i.operation.String()
}

// SetDefault prefills a default value for the next call to Readline()
// or related methods. The value will appear after the prompt for the user
// to edit, with the cursor at the end of the line.
func (i *Instance) SetDefault(defaultValue string) {
	i.operation.SetBuffer(defaultValue)
}

func (i *Instance) ReadlineWithDefault(what string) (string, error) {
	i.SetDefault(what)
	return i.operation.String()
}

func (i *Instance) SaveHistory(content string) error {
	return i.operation.SaveHistory(content)
}

// same as readline
func (i *Instance) ReadSlice() ([]byte, error) {
	return i.operation.Slice()
}

// Close() closes the readline instance, cleaning up state changes to the
// terminal. It interrupts any concurrent Readline() operation, so it can be
// asynchronously or from a signal handler. It is concurrency-safe and
// idempotent, so it can be called multiple times.
func (i *Instance) Close() error {
	i.closeOnce.Do(func() {
		// TODO reorder these?
		i.operation.Close()
		i.closeErr = i.terminal.Close()
	})
	return i.closeErr
}

// CaptureExitSignal registers handlers for common exit signals that will
// close the readline instance.
func (i *Instance) CaptureExitSignal() {
	cSignal := make(chan os.Signal, 1)
	// TODO handle other signals in a portable way?
	signal.Notify(cSignal, os.Interrupt, syscall.SIGTERM)
	go func() {
		for range cSignal {
			i.Close()
		}
	}()
}

func (i *Instance) Clean() {
	i.operation.Clean()
}

func (i *Instance) Write(b []byte) (int, error) {
	return i.Stdout().Write(b)
}

// GetConfig returns a copy of the current config.
func (i *Instance) GetConfig() *Config {
	cfg := i.operation.GetConfig()
	result := new(Config)
	*result = *cfg
	return result
}

func (i *Instance) SetConfig(cfg *Config) error {
	_, err := i.operation.SetConfig(cfg)
	return err
}

func (i *Instance) Refresh() {
	i.operation.Refresh()
}

// HistoryDisable the save of the commands into the history
func (i *Instance) HistoryDisable() {
	i.operation.history.Disable()
}

// HistoryEnable the save of the commands into the history (default on)
func (i *Instance) HistoryEnable() {
	i.operation.history.Enable()
}
