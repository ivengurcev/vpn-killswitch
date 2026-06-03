package log

import (
	"fmt"
	"io"
	"os"
)

type Logger struct {
	Out     io.Writer
	Err     io.Writer
	Verbose bool
	Quiet   bool
}

func New(verbose, quiet bool) Logger {
	return Logger{Out: os.Stdout, Err: os.Stderr, Verbose: verbose, Quiet: quiet}
}

func (l Logger) Info(format string, args ...any) {
	if l.Quiet {
		return
	}
	fmt.Fprintf(l.Out, "INFO  "+format+"\n", args...)
}

func (l Logger) Verbosef(format string, args ...any) {
	if l.Verbose && !l.Quiet {
		fmt.Fprintf(l.Out, "INFO  "+format+"\n", args...)
	}
}

func (l Logger) Warn(format string, args ...any) {
	fmt.Fprintf(l.Err, "WARN  "+format+"\n", args...)
}

func (l Logger) Error(format string, args ...any) {
	fmt.Fprintf(l.Err, "ERROR "+format+"\n", args...)
}
