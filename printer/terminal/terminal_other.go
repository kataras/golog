//go:build !unix && !windows

package terminal

import "io"

// IsTerminal reports whether "w" writes to a terminal device.
//
// This build has no way to ask: js/wasm, wasip1/wasm and plan9 have no
// termios ioctl. Reporting false disables colors rather than failing to
// build, which is what this file exists for.
func IsTerminal(w io.Writer) bool {
	return false
}
