package logger

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/logging"
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
	begin := time.Now()
	l := newConsoleLogger(r, c.noColor)
	r = r.WithContext(newContext(r.Context(), l))
	sw := &statusWriter{ResponseWriter: w}

	c.next.ServeHTTP(sw, r)

	l.mu.Lock()
	logCount := l.logCount
	maxSeverity := l.maxSeverity
	l.mu.Unlock()

	// status code should also set the minimum maxSeverity to Error
	if sw.Status() > 399 && maxSeverity < logging.Error {
		maxSeverity = logging.Error
	}

	l.consolef(
		maxSeverity, severityColor(maxSeverity), "%s %s %d %s requestSize=%d responseSize=%d logCount=%d",
		r.Method, r.URL.Path, sw.Status(), time.Since(begin), requestSize(r.Header.Get("Content-Length")), sw.length, logCount,
	)
}

type consoleLogger struct {
	r           *http.Request
	noColor     bool
	mu          sync.Mutex
	maxSeverity logging.Severity
	logCount    int
	attributes  map[string]any
}

// newConsoleLogger logs all output to console
func newConsoleLogger(r *http.Request, noColor bool) *consoleLogger {
	return &consoleLogger{r: r, noColor: noColor, maxSeverity: logging.Debug}
}

// Debug logs a debug message.
func (l *consoleLogger) Debug(_ context.Context, v any) {
	l.console(logging.Debug, gray, v)
}

// Debugf logs a debug message with format.
func (l *consoleLogger) Debugf(_ context.Context, format string, v ...any) {
	l.consolef(logging.Debug, gray, format, v...)
}

// Info logs a info message.
func (l *consoleLogger) Info(_ context.Context, v any) {
	l.console(logging.Info, blue, v)
}

// Infof logs a info message with format.
func (l *consoleLogger) Infof(_ context.Context, format string, v ...any) {
	l.consolef(logging.Info, blue, format, v...)
}

// Warn logs a warning message.
func (l *consoleLogger) Warn(_ context.Context, v any) {
	l.console(logging.Warning, yellow, v)
}

// Warnf logs a warning message with format.
func (l *consoleLogger) Warnf(_ context.Context, format string, v ...any) {
	l.consolef(logging.Warning, yellow, format, v...)
}

// Error logs an error message.
func (l *consoleLogger) Error(_ context.Context, v any) {
	l.console(logging.Error, red, v)
}

// Errorf logs an error message with format.
func (l *consoleLogger) Errorf(_ context.Context, format string, v ...any) {
	l.consolef(logging.Error, red, format, v...)
}

// AddAttributes adds attributes to include in middleware-driven logs
func (l *consoleLogger) AddAttributes(attrbs map[string]any) {
	if l.attributes == nil {
		l.attributes = make(map[string]any)
	}

	for k, v := range attrbs {
		l.attributes[k] = v
	}
}

func (l *consoleLogger) console(level logging.Severity, c color, v any) {
	log.Printf(l.colorPrint(level, c)+": %s", v)
}

func (l *consoleLogger) consolef(level logging.Severity, c color, format string, v ...any) {
	log.Printf(l.colorPrint(level, c)+": "+format, v...)
}

func (l *consoleLogger) colorPrint(level logging.Severity, c color) string {
	l.mu.Lock()
	if l.maxSeverity < level {
		l.maxSeverity = level
	}
	l.logCount++
	l.mu.Unlock()

	strLevel := strings.ToUpper(level.String())
	if level == logging.Warning {
		strLevel = strLevel[:4]
	}

	if l.noColor {
		return fmt.Sprintf("%-5s", strLevel)
	}

	return fmt.Sprintf("%s%-5s%s", string([]byte{0x1b, '[', byte('0' + c/10), byte('0' + c%10), 'm'}), strLevel, "\x1b[0m")
}

func severityColor(level logging.Severity) color {
	switch level {
	case logging.Error:
		return red
	case logging.Warning:
		return yellow
	case logging.Info:
		return blue
	default:
		return gray
	}
}
