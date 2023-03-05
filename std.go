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
