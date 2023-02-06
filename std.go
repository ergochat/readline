package readline

import (
	"io"
	"os"
	"sync"
)

var (
	Stdin  io.ReadCloser  = os.Stdin
	Stdout io.WriteCloser = os.Stdout
	Stderr io.WriteCloser = os.Stderr
)

var (
	std     *Instance
	stdOnce sync.Once
)

// global instance will not submit history automatic
func getInstance() *Instance {
	stdOnce.Do(func() {
		std, _ = NewEx(&Config{
			DisableAutoSaveHistory: true,
		})
	})
	return std
}

// let readline load history from filepath
// and try to persist history into disk
// set fp to "" to prevent readline persisting history to disk
// so the `AddHistory` will return nil error forever.
func SetHistoryPath(fp string) {
	ins := getInstance()
	cfg := ins.Config.Clone()
	cfg.HistoryFile = fp
	ins.SetConfig(cfg)
}

// set auto completer to global instance
func SetAutoComplete(completer AutoCompleter) {
	ins := getInstance()
	cfg := ins.Config.Clone()
	cfg.AutoComplete = completer
	ins.SetConfig(cfg)
}

// add history to global instance manually
// raise error only if `SetHistoryPath` is set with a non-empty path
func AddHistory(content string) error {
	ins := getInstance()
	return ins.SaveHistory(content)
}

func Password(prompt string) ([]byte, error) {
	ins := getInstance()
	return ins.ReadPassword(prompt)
}

// readline with global configs
func Line(prompt string) (string, error) {
	ins := getInstance()
	ins.SetPrompt(prompt)
	return ins.Readline()
}

// FillableStdin is a stdin reader which can prepend some data before
// reading into the real stdin
type FillableStdin struct {
	sync.Mutex
	stdin       io.Reader
	buf         []byte
}

// NewFillableStdin gives you FillableStdin
func NewFillableStdin(stdin io.Reader) io.ReadWriter {
	return &FillableStdin{
		stdin:       stdin,
	}
}

// Write adds data to the buffer that is prepended to the real stdin.
func (s *FillableStdin) Write(p []byte) (n int, err error) {
	s.Lock()
	defer s.Unlock()
	s.buf = append(s.buf, p...)
	return len(p), nil
}

// Read will read from the local buffer and if no data, read from stdin
func (s *FillableStdin) Read(p []byte) (n int, err error) {
	s.Lock()
	if len(s.buf) > 0 {
		// copy buffered data, slide back and reslice
		n = copy(p, s.buf)
		remaining := copy(s.buf, s.buf[n:])
		s.buf = s.buf[:remaining]
	}
	s.Unlock()

	if n > 0 {
		return n, nil
	}

	return s.stdin.Read(p)
}
