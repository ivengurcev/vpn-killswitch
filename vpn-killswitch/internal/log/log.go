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
	fmt.Fprintf(l.out(), format+"\n", args...)
}

func (l Logger) Verbosef(format string, args ...any) {
	if !l.VerboseEnabled() {
		return
	}
	fmt.Fprintf(l.out(), format+"\n", args...)
}

func (l Logger) Warn(format string, args ...any) {
	if l.Quiet {
		return
	}
	fmt.Fprintf(l.err(), "WARN "+format+"\n", args...)
}

func (l Logger) Error(format string, args ...any) {
	fmt.Fprintf(l.err(), "ERROR "+format+"\n", args...)
}

func (l Logger) VerboseEnabled() bool {
	return l.Verbose && !l.Quiet
}

func (l Logger) out() io.Writer {
	if l.Out != nil {
		return l.Out
	}
	return os.Stdout
}

func (l Logger) err() io.Writer {
	if l.Err != nil {
		return l.Err
	}
	return os.Stderr
}
