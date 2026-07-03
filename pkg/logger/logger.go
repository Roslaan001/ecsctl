package logger

import (
	"fmt"
	"os"
)

// Info prints an informational message to stdout.
func Info(format string, args ...any) {
	fmt.Printf("  "+format+"\n", args...)
}

// Success prints a success message with a checkmark.
func Success(format string, args ...any) {
	fmt.Printf("✓ "+format+"\n", args...)
}

// Warn prints a warning message to stderr.
func Warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "⚠ "+format+"\n", args...)
}

// Error prints an error message to stderr.
func Error(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "✗ "+format+"\n", args...)
}
