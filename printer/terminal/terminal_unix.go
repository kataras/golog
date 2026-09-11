//go:build unix

package terminal

import (
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// IsTerminal reports whether "w" writes to a terminal device.
//
// The ioctl request differs per platform; see the terminal_ioctl_*.go files.
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}

	_, err := unix.IoctlGetTermios(int(f.Fd()), ioctlReadTermios)
	return err == nil
}
