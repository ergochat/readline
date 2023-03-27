//go:build aix || darwin || dragonfly || freebsd || (linux && !appengine) || netbsd || openbsd || os400 || solaris

package platform

import (
	"context"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/ergochat/readline/internal/term"
)

const (
	IsWindows = false
)

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// SuspendProcess suspends the process with SIGTSTP,
// then blocks until it is resumed.
func SuspendProcess() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGCONT)
	defer stop()

	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		panic(err)
	}
	p.Signal(syscall.SIGTSTP)
	// wait for SIGCONT
	<-ctx.Done()
}

// get width of the terminal
func getWidth(stdoutFd int) int {
	cols, _, err := term.GetSize(stdoutFd)
	if err != nil {
		return -1
	}
	return cols
}

func GetScreenWidth() int {
	w := getWidth(syscall.Stdout)
	if w < 0 {
		w = getWidth(syscall.Stderr)
	}
	return w
}

// getWidthHeight of the terminal using given file descriptor
func getWidthHeight(stdoutFd int) (width int, height int) {
	width, height, err := term.GetSize(stdoutFd)
	if err != nil {
		return -1, -1
	}
	return
}

// GetScreenSize returns the width/height of the terminal or -1,-1 or error
func GetScreenSize() (width int, height int) {
	width, height = getWidthHeight(syscall.Stdout)
	if width < 0 {
		width, height = getWidthHeight(syscall.Stderr)
	}
	return
}

// ClearScreen clears the console screen
func ClearScreen(w io.Writer) (int, error) {
	return w.Write([]byte("\033[H"))
}

func DefaultIsTerminal() bool {
	return term.IsTerminal(syscall.Stdin) && (term.IsTerminal(syscall.Stdout) || term.IsTerminal(syscall.Stderr))
}

func GetStdin() int {
	return syscall.Stdin
}

// -----------------------------------------------------------------------------

var (
	sizeChange         sync.Once
	sizeChangeCallback func()
)

func DefaultOnWidthChanged(f func()) {
	DefaultOnSizeChanged(f)
}

func DefaultOnSizeChanged(f func()) {
	sizeChangeCallback = f
	sizeChange.Do(func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGWINCH)

		go func() {
			for {
				_, ok := <-ch
				if !ok {
					break
				}
				sizeChangeCallback()
			}
		}()
	})
}
