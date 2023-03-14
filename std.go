package readline

import (
	"io"
	"sync"
)

// fillableStdin is a stdin reader which can prepend some data before
// reading into the real stdin
type fillableStdin struct {
	sync.Mutex
	stdin       io.Reader
	buf         []byte
}

func newFillableStdin(stdin io.Reader) io.ReadWriter {
	return &fillableStdin{
		stdin:       stdin,
	}
}

// Write adds data to the buffer that is prepended to the real stdin.
func (s *fillableStdin) Write(p []byte) (n int, err error) {
	s.Lock()
	defer s.Unlock()
	s.buf = append(s.buf, p...)
	return len(p), nil
}

// Read will read from the local buffer and if no data, read from stdin
func (s *fillableStdin) Read(p []byte) (n int, err error) {
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
