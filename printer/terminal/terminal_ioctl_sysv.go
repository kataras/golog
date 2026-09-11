//go:build aix || solaris

package terminal

import "golang.org/x/sys/unix"

// Also covers illumos, which satisfies the solaris build tag.
//
// The filename deliberately avoids a _solaris suffix: a GOOS filename suffix
// ANDs with the build tag above, which would exclude this file on aix and
// leave ioctlReadTermios undefined there.
const ioctlReadTermios = unix.TCGETA
