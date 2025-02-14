//go:build js

package platform

import (
	"syscall"

	"github.com/cogentcore/readline/internal/term"
)

const (
	IsWindows = false
)

func SuspendProcess() {
}

// GetScreenSize returns the width, height of the terminal or -1,-1
func GetScreenSize() (width int, height int) {
	width, height, err := term.GetSize(int(syscall.Stdout))
	if err == nil {
		return width, height
	} else {
		return 0, 0
	}
}

func DefaultIsTerminal() bool {
	return term.IsTerminal(int(syscall.Stdin)) && term.IsTerminal(int(syscall.Stdout))
}

func DefaultOnWidthChanged(f func()) {
	DefaultOnSizeChanged(f)
}

func DefaultOnSizeChanged(f func()) {
}
