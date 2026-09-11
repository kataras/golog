//go:build linux

package terminal

import "golang.org/x/sys/unix"

// Also covers android, which satisfies the linux build tag.
const ioctlReadTermios = unix.TCGETS
