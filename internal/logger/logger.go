package logger

import (
	"fmt"
	"io"
	"os"
)

// Logger provides structured output with verbosity levels.
type Logger struct {
	verbose bool
	out     io.Writer
	errOut  io.Writer
}

// New creates a Logger. If verbose is true, Verbose() messages are printed.
func New(verbose bool) *Logger {
	return &Logger{
		verbose: verbose,
		out:     os.Stdout,
		errOut:  os.Stderr,
	}
}

// Info prints a message to stdout (always visible).
func (l *Logger) Info(format string, args ...any) {
	_, _ = fmt.Fprintf(l.out, format+"\n", args...)
}

// Infof prints without newline (for inline output like progress).
func (l *Logger) Infof(format string, args ...any) {
	_, _ = fmt.Fprintf(l.out, format, args...)
}

// Verbose prints only when verbose mode is enabled.
func (l *Logger) Verbose(format string, args ...any) {
	if l.verbose {
		_, _ = fmt.Fprintf(l.out, format+"\n", args...)
	}
}

// Warn prints a warning to stderr.
func (l *Logger) Warn(format string, args ...any) {
	_, _ = fmt.Fprintf(l.errOut, "Warning: "+format+"\n", args...)
}

// Error prints an error to stderr.
func (l *Logger) Error(format string, args ...any) {
	_, _ = fmt.Fprintf(l.errOut, "Error: "+format+"\n", args...)
}
