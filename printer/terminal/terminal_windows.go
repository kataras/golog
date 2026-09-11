//go:build windows

package terminal

import (
	"io"
	"os"

	"golang.org/x/sys/windows"
)

// Console output modes, from the Windows SetConsoleMode documentation:
// https://learn.microsoft.com/en-us/windows/console/setconsolemode
//
// x/sys/windows exports the ENABLE_*_INPUT half of these but not the output
// half, so they are spelled out here.
const (
	enableVirtualTerminalProcessing = 0x0004
)

// firstColorBuild is the Windows 10 build that introduced 24-bit color
// support in the console host (Threshold 2, November 2015).
const firstColorBuild = 10586

func init() { // modifies the "SupportColors" package-level variable.
	SupportColors = false

	// RtlGetVersion reports the real OS version, unaffected by the
	// application-compatibility shims that make GetVersionEx lie to
	// programs without a matching manifest. It cannot fail.
	//
	// This used to shell out to `cmd ver` and parse the build number out of
	// its output, which cost a process launch in every binary that imported
	// golog and reads to antivirus heuristics like a program hiding a shell.
	v := windows.RtlGetVersion()
	if v.MajorVersion < 10 {
		return
	}

	// Activate virtual terminal processing so ANSI escape sequences are
	// interpreted rather than printed literally. Both standard handles are
	// tried: the default Printer writes to stdout, and when stderr alone is
	// redirected to a file the console still needs the mode set through the
	// handle that is a console. A redirected handle fails GetConsoleMode and
	// is skipped, which is not a reason to disable color for the other one.
	// The existing flags are kept and only the VT bit is added.
	for _, f := range []*os.File{os.Stdout, os.Stderr} {
		enableVirtualTerminal(windows.Handle(f.Fd()))
	}

	SupportColors = v.BuildNumber >= firstColorBuild
}

func enableVirtualTerminal(h windows.Handle) {
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return // not a console.
	}
	if mode&enableVirtualTerminalProcessing != 0 {
		return // already on, as it is under Windows Terminal.
	}
	_ = windows.SetConsoleMode(h, mode|enableVirtualTerminalProcessing)
}

// IsTerminal reports whether "w" writes to a terminal device.
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}

	var mode uint32
	err := windows.GetConsoleMode(windows.Handle(f.Fd()), &mode)
	return err == nil
}
