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

	// Listener is an optional callback to intercept keypresses.
	Listener Listener

	Painter Painter

	// If VimMode is true, readline will in vim.insert mode by default
	VimMode bool

	InterruptPrompt string
	EOFPrompt       string

	// Function that returns width, height of the terminal or -1,-1 if unknown
	FuncGetSize func() (width int, height int)

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	EnableMask bool
	MaskRune   rune

	// Whether to maintain an undo buffer (Ctrl+_ to undo if enabled)
	Undo bool

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
		c.Painter = defaultPainter
	}

	c.isInteractive = c.ForceUseInteractive || c.FuncIsTerminal()

	return nil
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

// GeneratePasswordConfig generates a suitable Config for reading passwords;
// this config can be modified and then used with ReadLineWithConfig, or
// SetConfig.
func (i *Instance) GeneratePasswordConfig() *Config {
	return i.operation.GenPasswordConfig()
}

func (i *Instance) ReadLineWithConfig(cfg *Config) (string, error) {
	return i.operation.ReadLineWithConfig(cfg)
}

func (i *Instance) ReadPassword(prompt string) ([]byte, error) {
	if result, err := i.ReadLineWithConfig(i.GeneratePasswordConfig()); err == nil {
		return []byte(result), nil
	} else {
		return nil, err
	}
}

// ReadLine reads a line from the configured input source, allowing inline editing.
// The returned error is either nil, io.EOF, or readline.ErrInterrupt.
func (i *Instance) ReadLine() (string, error) {
	return i.operation.String()
}

// Readline is an alias for ReadLine, for compatibility.
func (i *Instance) Readline() (string, error) {
	return i.ReadLine()
}

// SetDefault prefills a default value for the next call to Readline()
// or related methods. The value will appear after the prompt for the user
// to edit, with the cursor at the end of the line.
func (i *Instance) SetDefault(defaultValue string) {
	i.operation.SetBuffer(defaultValue)
}

func (i *Instance) ReadLineWithDefault(defaultValue string) (string, error) {
	i.SetDefault(defaultValue)
	return i.operation.String()
}

// SaveToHistory adds a string to the instance's stored history. This is particularly
// relevant when DisableAutoSaveHistory is configured.
func (i *Instance) SaveToHistory(content string) error {
	return i.operation.SaveToHistory(content)
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

// Write writes output to the screen, redrawing the prompt and buffer
// as needed.
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

// SetConfig modifies the current instance's config.
func (i *Instance) SetConfig(cfg *Config) error {
	_, err := i.operation.SetConfig(cfg)
	return err
}

// Refresh redraws the input buffer on screen.
func (i *Instance) Refresh() {
	i.operation.Refresh()
}

// DisableHistory disables the saving of input lines in history.
func (i *Instance) DisableHistory() {
	i.operation.history.Disable()
}

// EnableHistory enables the saving of input lines in history.
func (i *Instance) EnableHistory() {
	i.operation.history.Enable()
}

// ClearScreen clears the screen.
func (i *Instance) ClearScreen() {
	clearScreen(i.operation.Stdout())
}

// Painter is a callback type to allow modifying the buffer before it is rendered
// on screen, for example, to implement real-time syntax highlighting.
type Painter func(line []rune, pos int) []rune

func defaultPainter(line []rune, _ int) []rune {
	return line
}

// Listener is a callback type to listen for keypresses while the line is being
// edited. It is invoked initially with (nil, 0, 0), and then subsequently for
// any keypress until (but not including) the newline/enter keypress that completes
// the input.
type Listener func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool)
