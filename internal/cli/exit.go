package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

// ExitError carries the process exit status a command wants. An empty Msg means
// the command already wrote everything the caller needs to stdout — a validation
// report is output, not an error string.
type ExitError struct {
	Code int
	Msg  string
}

func (e *ExitError) Error() string { return e.Msg }

func usageErr(format string, a ...any) error {
	return &ExitError{Code: 2, Msg: fmt.Sprintf(format, a...)}
}

// usagePrefixes are how cobra words the mistakes a caller makes at the command
// line. Cobra returns them as plain errors, which would exit 1 — the status
// reserved for bad *input*. A caller mistake is 2.
var usagePrefixes = []string{
	"unknown command",
	"unknown flag",
	"unknown shorthand flag",
	"invalid argument",
	"flag needs an argument",
	"accepts ",
	"requires at least",
	"unknown subcommand",
}

// isUsage reports whether err is cobra complaining about the command line.
func isUsage(err error) bool {
	msg := err.Error()
	for _, p := range usagePrefixes {
		if strings.HasPrefix(msg, p) {
			return true
		}
	}
	return false
}

// Fail reports a failed Execute and returns the status to exit with.
func Fail(w io.Writer, err error) int {
	var e *ExitError
	if errors.As(err, &e) {
		if e.Msg != "" {
			_, _ = fmt.Fprintln(w, "mkit:", e.Msg)
		}
		if e.Code == 0 {
			return 1
		}
		return e.Code
	}
	_, _ = fmt.Fprintln(w, "mkit:", err)
	if isUsage(err) {
		return 2
	}
	return 1
}
