//go:build windows

package platform

import (
	"syscall"
)

const (
	IsWindows = true
)

func SuspendProcess() {
}

func GetStdin() int {
	return int(syscall.Stdin)
}

// GetScreenSize returns the width, height of the terminal or -1,-1
func GetScreenSize() (width int, height int) {
	info, _ := GetConsoleScreenBufferInfo()
	if info == nil {
		return -1, -1
	}
	height = int(info.srWindow.bottom) - int(info.srWindow.top) + 1
	width = int(info.srWindow.right) - int(info.srWindow.left) + 1
	return
}

func DefaultIsTerminal() bool {
	return true
}

func DefaultOnWidthChanged(f func()) {
	DefaultOnSizeChanged(f)
}

func DefaultOnSizeChanged(f func()) {

}
