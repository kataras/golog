//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package terminal

import "golang.org/x/sys/unix"

// Also covers ios, which satisfies the darwin build tag.
const ioctlReadTermios = unix.TIOCGETA
