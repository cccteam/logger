package logger

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/logging"
	"github.com/go-playground/errors/v5"
)

const (
	cslReqSize  = "requestSize"
	cslRespSize = "responseSize"
	cslLogCount = "logCount"
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
	attributes := l.attributes
	l.mu.Unlock()

	// status code should also set the minimum maxSeverity to Error
	if sw.Status() > 399 && maxSeverity < logging.Error {
		maxSeverity = logging.Error
	}

	msg := fmt.Sprintf("%s %s %d %s %s=%d %s=%d %s=%d", r.Method, r.URL.Path, sw.Status(), time.Since(begin),
		cslReqSize, requestSize(r.Header.Get("Content-Length")), cslRespSize, sw.length, cslLogCount, logCount,
	)
	for k, v := range attributes {
		msg += fmt.Sprintf(" %s=%v", k, v)
	}
	l.console(maxSeverity, severityColor(maxSeverity), msg)
}

type consoleLogger struct {
	r            *http.Request
	noColor      bool
	reservedKeys []string
	mu           sync.Mutex
	maxSeverity  logging.Severity
	logCount     int
	attributes   map[string]any
}

// newConsoleLogger logs all output to console
func newConsoleLogger(r *http.Request, noColor bool) *consoleLogger {
	return &consoleLogger{
		r: r, noColor: noColor,
		reservedKeys: []string{cslReqSize, cslRespSize, cslLogCount},
		maxSeverity:  logging.Debug,
		attributes:   make(map[string]any),
	}
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

// AddRequestAttribute adds an attribute (key, value) for the parent request log
// If the key already exists, its value is overwritten
func (l *consoleLogger) AddRequestAttribute(key string, value any) error {
	if slices.Contains(l.reservedKeys, key) {
		return errors.Newf("'%s' is a reserved key", key)
	}

	l.mu.Lock()
	l.attributes[key] = value
	l.mu.Unlock()

	return nil
}

// RemoveAttributes removes attributes from the logger
// If a key does not exist, it is ignored
func (l *consoleLogger) RemoveAttributes(keys ...string) {
	l.mu.Lock()
	for _, k := range keys {
		delete(l.attributes, k)
	}
	l.mu.Unlock()
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
