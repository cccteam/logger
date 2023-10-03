package logger

import (
	"context"
	"log"
)

type stdErrLogger struct{}

// Debug logs a debug message.
func (l *stdErrLogger) Debug(_ context.Context, v any) {
	std("DEBUG", v)
}

// Debugf logs a debug message with format.
func (l *stdErrLogger) Debugf(_ context.Context, format string, v ...any) {
	stdf("DEBUG", format, v...)
}

// Info logs a info message.
func (l *stdErrLogger) Info(_ context.Context, v any) {
	std("INFO ", v)
}

// Infof logs a info message with format.
func (l *stdErrLogger) Infof(_ context.Context, format string, v ...any) {
	stdf("INFO ", format, v...)
}

// Warn logs a warning message.
func (l *stdErrLogger) Warn(_ context.Context, v any) {
	std("WARN ", v)
}

// Warnf logs a warning message with format.
func (l *stdErrLogger) Warnf(_ context.Context, format string, v ...any) {
	stdf("WARN ", format, v...)
}

// Error logs an error message.
func (l *stdErrLogger) Error(_ context.Context, v any) {
	std("ERROR", v)
}

// Errorf logs an error message with format.
func (l *stdErrLogger) Errorf(_ context.Context, format string, v ...any) {
	stdf("ERROR", format, v...)
}

func std(level string, v ...any) {
	log.Printf(level+": %s", v...)
}

func stdf(level, format string, v ...any) {
	log.Printf(level+": "+format, v...)
}
