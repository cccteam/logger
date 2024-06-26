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

// ConsoleExporter implements exporting to the console
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

// Middleware returns a middleware that exports logs to the console
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
	sw := newResponseRecorder(w)

	c.next.ServeHTTP(sw, r)

	l.mu.Lock()
	logCount := l.logCount
	maxSeverity := l.maxSeverity
	attributes := l.reqAttributes
	l.mu.Unlock()

	// status code should also set the minimum maxSeverity to Error
	if sw.Status() > 499 && maxSeverity < logging.Error {
		maxSeverity = logging.Error
	}

	msg := fmt.Sprintf("%s %s %d %s %s=%d %s=%d %s=%d", r.Method, r.URL.Path, sw.Status(), time.Since(begin),
		cslReqSize, requestSize(r.Header.Get("Content-Length")), cslRespSize, sw.Length(), cslLogCount, logCount,
	)
	for k, v := range attributes {
		msg += fmt.Sprintf(" %s=%v", k, v)
	}
	l.console(maxSeverity, severityColor(maxSeverity), msg)
}

type consoleLogger struct {
	root          *consoleLogger
	r             *http.Request
	noColor       bool
	rsvdReqKeys   []string
	attributes    map[string]any // attributes for child (trace) logs
	mu            sync.Mutex
	maxSeverity   logging.Severity
	logCount      int
	reqAttributes map[string]any // attributes for the parent request log
}

// newConsoleLogger logs all output to console
func newConsoleLogger(r *http.Request, noColor bool) *consoleLogger {
	l := &consoleLogger{
		r: r, noColor: noColor,
		rsvdReqKeys:   []string{cslReqSize, cslRespSize, cslLogCount},
		maxSeverity:   logging.Info,
		reqAttributes: make(map[string]any),
		attributes:    make(map[string]any),
	}
	l.root = l // root is self

	return l
}

// newChild returns a new child consoleLogger
func (l *consoleLogger) newChild() *consoleLogger {
	return &consoleLogger{
		root:          l.root,
		r:             l.r,
		noColor:       l.noColor,
		rsvdReqKeys:   l.rsvdReqKeys,
		maxSeverity:   logging.Debug,
		attributes:    make(map[string]any),
		reqAttributes: nil, // reqAttributes is only used in the root logger, never the child.
	}
}

// Debug logs a debug message.
func (l *consoleLogger) Debug(_ context.Context, v any) {
	l.console(logging.Debug, gray, fmt.Sprint(v))
}

// Debugf logs a debug message with format.
func (l *consoleLogger) Debugf(_ context.Context, format string, v ...any) {
	l.console(logging.Debug, gray, fmt.Sprintf(format, v...))
}

// Info logs a info message.
func (l *consoleLogger) Info(_ context.Context, v any) {
	l.console(logging.Info, blue, fmt.Sprint(v))
}

// Infof logs a info message with format.
func (l *consoleLogger) Infof(_ context.Context, format string, v ...any) {
	l.console(logging.Info, blue, fmt.Sprintf(format, v...))
}

// Warn logs a warning message.
func (l *consoleLogger) Warn(_ context.Context, v any) {
	l.console(logging.Warning, yellow, fmt.Sprint(v))
}

// Warnf logs a warning message with format.
func (l *consoleLogger) Warnf(_ context.Context, format string, v ...any) {
	l.console(logging.Warning, yellow, fmt.Sprintf(format, v...))
}

// Error logs an error message.
func (l *consoleLogger) Error(_ context.Context, v any) {
	l.console(logging.Error, red, fmt.Sprint(v))
}

// Errorf logs an error message with format.
func (l *consoleLogger) Errorf(_ context.Context, format string, v ...any) {
	l.console(logging.Error, red, fmt.Sprintf(format, v...))
}

// AddRequestAttribute adds an attribute (key, value) for the parent request log
// If the key matches a reserved key, it will be prefixed with "custom_"
// If the key already exists, its value is overwritten
func (l *consoleLogger) AddRequestAttribute(key string, value any) {
	if slices.Contains(l.rsvdReqKeys, key) {
		key = customPrefix + key
	}

	l.root.mu.Lock()
	defer l.root.mu.Unlock()
	l.root.reqAttributes[key] = value
}

// WithAttributes returns an attributer that can be used to add child (trace) log attributes
func (l *consoleLogger) WithAttributes() attributer {
	attrs := make(map[string]any)
	for k, v := range l.attributes {
		attrs[k] = v
	}

	return &consoleAttributer{logger: l, attributes: attrs}
}

// TraceID returns an empty string for the console logger
func (l *consoleLogger) TraceID() string {
	return ""
}

func (l *consoleLogger) console(level logging.Severity, c color, msg string) {
	for k, v := range l.attributes {
		msg += fmt.Sprintf(", %s=%v", k, v)
	}

	log.Printf(l.colorPrint(level, c)+": %s", msg)
}

func (l *consoleLogger) colorPrint(level logging.Severity, c color) string {
	l.root.mu.Lock()
	if l.root.maxSeverity < level {
		l.root.maxSeverity = level
	}
	l.root.logCount++
	l.root.mu.Unlock()

	strLevel := strings.ToUpper(level.String())
	if level == logging.Warning {
		strLevel = strLevel[:4]
	}

	if l.noColor {
		return fmt.Sprintf("%-5s", strLevel)
	}

	return fmt.Sprintf("%s%-5s%s", string([]byte{0x1b, '[', byte('0' + c/10), byte('0' + c%10), 'm'}), strLevel, "\x1b[0m")
}

var _ attributer = (*consoleAttributer)(nil)

type consoleAttributer struct {
	logger     *consoleLogger
	attributes map[string]any
}

// AddAttribute adds an attribute (key, value) for the child (trace) log
// If the key already exists, its value is overwritten
func (a *consoleAttributer) AddAttribute(key string, value any) {
	a.attributes[key] = value
}

// Logger returns a ctxLogger with the child (trace) attributes embedded
func (a *consoleAttributer) Logger() ctxLogger {
	l := a.logger.newChild()
	for k, v := range a.attributes {
		l.attributes[k] = v
	}

	return l
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
