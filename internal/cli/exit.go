package cli

import (
	"errors"
	"fmt"
	"io"
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
	return 1
}
