package logger

import (
	"context"
	"log"
	"sync"
)

type stdErrLogger struct {
	mu         sync.Mutex
	attributes map[string]any
}

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

// AddRequestAttribute adds an attribute (key, value) for the parent request log
// If the key already exists, its value is overwritten
func (l *stdErrLogger) AddRequestAttribute(key string, value any) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.attributes == nil {
		l.attributes = make(map[string]any)
	}
	l.attributes[key] = value

	return nil
}

// RemoveAttributes removes attributes from the logger
// If a key does not exist, it is ignored
func (l *stdErrLogger) RemoveAttributes(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, k := range keys {
		delete(l.attributes, k)
	}
}

func std(level string, v ...any) {
	log.Printf(level+": %s", v...)
}

func stdf(level, format string, v ...any) {
	log.Printf(level+": "+format, v...)
}
