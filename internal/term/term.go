package term

import (
	"fmt"
	"os"

	xterm "golang.org/x/term"
)

// IsTerminal reports whether f is connected to a terminal (character device).
func IsTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// ReadPassword reads a line from f without echoing it to the terminal, for
// prompting secrets (e.g. API tokens). The trailing newline the user types is
// not echoed either, so a newline is emitted to stdout afterward to move the
// cursor to the next line. The returned string excludes the line terminator.
//
// f must be a terminal; callers should gate on IsTerminal before prompting.
func ReadPassword(f *os.File) (string, error) {
	raw, err := xterm.ReadPassword(int(f.Fd()))
	fmt.Fprintln(os.Stdout)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
