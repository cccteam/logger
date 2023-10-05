package logger

import (
	"context"
	"log"
	"net/http"
)

type color int

const (
	red    color = 31
	yellow color = 33
	blue   color = 34
	gray   color = 37
)

// ConsoleExporter implements exporting to Google Cloud Logging
type ConsoleExporter struct {
	noColor bool
}

// NewConsoleExporter returns a configured ConsoleExporter
func NewConsoleExporter() *ConsoleExporter {
	return &ConsoleExporter{}
}

// NoColor controls if this logger will use color to highlight log level
func (e *ConsoleExporter) NoColor(v bool) *ConsoleExporter {
	e.noColor = v

	return e
}

// Middleware returns a middleware that exports logs to Google Cloud Logging
func (e *ConsoleExporter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return &consoleHandler{
			next:    next,
			noColor: e.noColor,
		}
	}
}

type consoleHandler struct {
	next    http.Handler
	noColor bool
}

func (c *consoleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = r.WithContext(newContext(r.Context(), newConsoleLogger(r, c.noColor)))
	c.next.ServeHTTP(w, r)
}

type consoleLogger struct {
	r       *http.Request
	noColor bool
}

// newConsoleLogger logs all output to console
func newConsoleLogger(r *http.Request, noColor bool) *consoleLogger {
	return &consoleLogger{r: r, noColor: noColor}
}

// Debug logs a debug message.
func (l *consoleLogger) Debug(_ context.Context, v any) {
	l.console("DEBUG", gray, v)
}

// Debugf logs a debug message with format.
func (l *consoleLogger) Debugf(_ context.Context, format string, v ...any) {
	l.consolef("DEBUG", gray, format, v...)
}

// Info logs a info message.
func (l *consoleLogger) Info(_ context.Context, v any) {
	l.console("INFO ", blue, v)
}

// Infof logs a info message with format.
func (l *consoleLogger) Infof(_ context.Context, format string, v ...any) {
	l.consolef("INFO ", blue, format, v...)
}

// Warn logs a warning message.
func (l *consoleLogger) Warn(_ context.Context, v any) {
	l.console("WARN ", yellow, v)
}

// Warnf logs a warning message with format.
func (l *consoleLogger) Warnf(_ context.Context, format string, v ...any) {
	l.consolef("WARN ", yellow, format, v...)
}

// Error logs an error message.
func (l *consoleLogger) Error(_ context.Context, v any) {
	l.console("ERROR", red, v)
}

// Errorf logs an error message with format.
func (l *consoleLogger) Errorf(_ context.Context, format string, v ...any) {
	l.consolef("ERROR", red, format, v...)
}

func (l *consoleLogger) console(level string, c color, v any) {
	log.Printf(l.colorPrint(level, c)+": %s %s", l.r.URL.Path, v)
}

func (l *consoleLogger) consolef(level string, c color, format string, v ...any) {
	log.Printf(l.colorPrint(level, c)+": "+l.r.Method+" "+l.r.URL.Path+" "+format, v...)
}

func (l *consoleLogger) colorPrint(s string, c color) string {
	if l.noColor {
		return s
	}

	return string([]byte{0x1b, '[', byte('0' + c/10), byte('0' + c%10), 'm'}) + s + "\x1b[0m"
}
