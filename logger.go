// package logger is an HTTP request logger that implements correlated logging to one of several supported platforms.
// Each HTTP request is logged as the parent log, with all logs generated during the request as child logs.
//
// The Logging destination is configured with an Exporter. This package provides Exporters for Google Cloud Logging, Stdout,
// and Console Logging.
//
// The GoogleCloudExporter will also correlate logs to Cloud Trace if you instrument your code with tracing.
//
// The StdoutExporter supports log correlation to AWS X-Ray if you instrument your code with tracing.
package logger

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// Logger implements logging methods for this package
type Logger struct {
	ctx context.Context
	lg  ctxLogger
}

// Ctx returns the logger from the context. If
// no logger is found, it will write to stderr
func Ctx(ctx context.Context) *Logger {
	return &Logger{
		ctx: ctx,
		lg:  fromCtx(ctx),
	}
}

// Req returns the logger from the http request. If
// no logger is found, it will write to stderr
func Req(r *http.Request) *Logger {
	return &Logger{
		ctx: r.Context(),
		lg:  fromReq(r),
	}
}

// Debug logs a debug message.
func (l *Logger) Debug(v any) {
	l.lg.Debug(l.ctx, v)
}

// Debugf logs a debug message with format.
func (l *Logger) Debugf(format string, v ...any) {
	l.lg.Debugf(l.ctx, format, v...)
}

// Info logs a info message.
func (l *Logger) Info(v any) {
	l.lg.Info(l.ctx, v)
}

// Infof logs a info message with format.
func (l *Logger) Infof(format string, v ...any) {
	l.lg.Infof(l.ctx, format, v...)
}

// Warn logs a warning message.
func (l *Logger) Warn(v any) {
	l.lg.Warn(l.ctx, v)
}

// Warnf logs a warning message with format.
func (l *Logger) Warnf(format string, v ...any) {
	l.lg.Warnf(l.ctx, format, v...)
}

// Error logs an error message.
func (l *Logger) Error(v any) {
	l.lg.Error(l.ctx, v)
}

// Errorf logs an error message with format.
func (l *Logger) Errorf(format string, v ...any) {
	l.lg.Errorf(l.ctx, format, v...)
}

// generateID provides an id that matches the trace id format
func generateID() string {
	t := [16]byte{}

	_, _ = rand.Read(t[:])

	return hex.EncodeToString(t[:])
}
