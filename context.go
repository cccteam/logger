package logger

import (
	"context"
	"net/http"
)

type key int

const (
	logKey key = iota
)

// fromCtx gets the logger out of the context.
// If no logger is stored in the context, a stderr logger is returned.
func fromCtx(ctx context.Context) ctxLogger {
	if ctx == nil {
		return &stdErrLogger{}
	}
	l, ok := ctx.Value(logKey).(ctxLogger)
	if !ok {
		return &stdErrLogger{}
	}

	return l
}

// fromReq gets the logger in the request's context.
func fromReq(r *http.Request) ctxLogger {
	if r == nil {
		return &stdErrLogger{}
	}

	return fromCtx(r.Context())
}

// newContext returns a copy of the parent context and associates it with the provided logger.
func newContext(ctx context.Context, l ctxLogger) context.Context {
	return context.WithValue(ctx, logKey, l)
}

// ctxLogger defines the logging interface with context
type ctxLogger interface {
	// Debug logs a debug message.
	Debug(ctx context.Context, v any)
	// Debugf logs a debug message with format.
	Debugf(ctx context.Context, format string, v ...any)
	// Info logs a info message.
	Info(ctx context.Context, v any)
	// Infof logs a info message with format.
	Infof(ctx context.Context, format string, v ...any)
	// Warn logs a warning message.
	Warn(ctx context.Context, v any)
	// Warnf logs a warning message with format.
	Warnf(ctx context.Context, format string, v ...any)
	// Error logs an error message.
	Error(ctx context.Context, v any)
	// Errorf logs an error message with format.
	Errorf(ctx context.Context, format string, v ...any)
	// AddAttributes adds attributes to include in middleware-driven logs
	AddAttributes(attrbs map[string]any)
}
