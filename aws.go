package logger

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// AWSExporter is an Exporter that logs to stdout in JSON format to be sent to cloudwatch
type AWSExporter struct {
	// logAll controls if this logger will log all requests, or only requests that have child logs
	logAll bool
}

// NewAWSExporter returns a new AWSExporter
func NewAWSExporter(logAll bool) *AWSExporter {
	return &AWSExporter{
		logAll: logAll,
	}
}

// Middleware returns a middleware that logs the request and injects a Logger into the context.
func (e *AWSExporter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return &awsHandler{
			next:   next,
			logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
			logAll: e.logAll,
		}
	}
}

type awsHandler struct {
	next   http.Handler
	logger awslog
	logAll bool
}

// ServeHTTP implements http.Handler
//
// This performs pre and post request logic for logging
func (h *awsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	begin := time.Now()
	xrayTraceID := awsTraceIDFromRequest(r, generateID)
	l := newAWSLogger(h.logger, xrayTraceID)
	r = r.WithContext(newContext(r.Context(), l))
	sw := &statusWriter{ResponseWriter: w}

	h.next.ServeHTTP(sw, r)

	l.mu.Lock()
	logCount := l.logCount
	maxLevel := l.maxLevel
	attributes := l.attributes
	l.mu.Unlock()

	if !h.logAll && logCount == 0 {
		return
	}

	if sw.Status() > 399 && maxLevel < slog.LevelError {
		maxLevel = slog.LevelError
	}

	sc := trace.SpanFromContext(r.Context()).SpanContext()

	logAttr := []slog.Attr{
		slog.Any("trace_id", xrayTraceID),
		slog.Any("span_id", sc.SpanID().String()),
		slog.String("http.elapsed", time.Since(begin).String()),
	}
	for k, v := range attributes {
		logAttr = append(logAttr, slog.Any(k, v))
	}

	logAttr = append(logAttr, httpAttributes(r, sw)...)
	h.logger.LogAttrs(r.Context(), maxLevel, parentLogEntry, logAttr...)
}

type awsLogger struct {
	logger     awslog
	traceID    string
	mu         sync.Mutex
	maxLevel   slog.Level
	logCount   int
	attributes map[string]any
}

func newAWSLogger(logger awslog, traceID string) *awsLogger {
	return &awsLogger{
		logger:  logger,
		traceID: traceID,
	}
}

type awslog interface {
	LogAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr)
}

// Debug logs a debug message.
func (l *awsLogger) Debug(ctx context.Context, v any) {
	l.log(ctx, slog.LevelDebug, fmt.Sprint(v))
}

// Debugf logs a debug message with format.
func (l *awsLogger) Debugf(ctx context.Context, format string, v ...any) {
	l.log(ctx, slog.LevelDebug, fmt.Sprintf(format, v...))
}

// Info logs a info message.
func (l *awsLogger) Info(ctx context.Context, v any) {
	l.log(ctx, slog.LevelInfo, fmt.Sprint(v))
}

// Infof logs a info message with format.
func (l *awsLogger) Infof(ctx context.Context, format string, v ...any) {
	l.log(ctx, slog.LevelInfo, fmt.Sprintf(format, v...))
}

// Warn logs a warning message.
func (l *awsLogger) Warn(ctx context.Context, v any) {
	l.log(ctx, slog.LevelWarn, fmt.Sprint(v))
}

// Warnf logs a warning message with format.
func (l *awsLogger) Warnf(ctx context.Context, format string, v ...any) {
	l.log(ctx, slog.LevelWarn, fmt.Sprintf(format, v...))
}

// Error logs an error message.
func (l *awsLogger) Error(ctx context.Context, v any) {
	l.log(ctx, slog.LevelError, fmt.Sprint(v))
}

// Errorf logs an error message with format.
func (l *awsLogger) Errorf(ctx context.Context, format string, v ...any) {
	l.log(ctx, slog.LevelError, fmt.Sprintf(format, v...))
}

// AddAttributes adds attributes to include in automated logs
func (l *awsLogger) AddAttributes(attrbs map[string]any) {
	l.mu.Lock()
	if l.attributes == nil {
		l.attributes = make(map[string]any)
	}

	for k, v := range attrbs {
		l.attributes[k] = v
	}
	l.mu.Unlock()
}

func (l *awsLogger) log(ctx context.Context, level slog.Level, message string) {
	l.mu.Lock()
	if l.maxLevel < level {
		l.maxLevel = level
	}
	l.logCount++
	l.mu.Unlock()

	span := trace.SpanFromContext(ctx)
	attr := []slog.Attr{
		slog.String("trace_id", l.traceID),
		slog.String("span_id", span.SpanContext().SpanID().String()),
	}
	l.logger.LogAttrs(ctx, level, message, attr...)
}

// httpAttributes returns a slice of slog.Attr for the http request and response
func httpAttributes(r *http.Request, sw *statusWriter) []slog.Attr {
	return []slog.Attr{
		slog.String("http.method", r.Method),
		slog.String("http.url", r.URL.String()),
		slog.Int("http.status_code", sw.Status()),
		slog.Int64("http.response.length", sw.length),
		slog.String("http.user_agent", r.UserAgent()),
		slog.String("http.remote_ip", r.RemoteAddr),
		slog.String("http.scheme", r.URL.Scheme),
		slog.String("http.proto", r.Proto),
	}
}

// awsTraceIDFromRequest retrieves the trace id from the request if possible
func awsTraceIDFromRequest(r *http.Request, idgen func() string) string {
	var traceID string
	sc := trace.SpanFromContext(r.Context()).SpanContext()
	if sc.IsValid() {
		traceID = sc.TraceID().String()
	} else {
		traceID = idgen()
	}

	return traceID
}
