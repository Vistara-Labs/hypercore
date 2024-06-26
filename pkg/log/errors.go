package log

import (
	"errors"
	"fmt"
)

// ErrLogOutputRequired is used when no log output is specified.
var ErrLogOutputRequired = errors.New("you must specify a log output")

type invalidLogFormatError struct {
	format string
}

func (e invalidLogFormatError) Error() string {
	return fmt.Sprintf("logger format %s is invalid", e.format)
}
