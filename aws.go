package logger

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"
)

const (
	awsTraceIDKey        = "trace_id"
	awsSpanIDKey         = "span_id"
	awsHTTPElapsedKey    = "http.elapsed"
	awsHTTPMethodKey     = "http.method"
	awsHTTPURLKey        = "http.url"
	awsHTTPStatusCodeKey = "http.status_code"
	awsHTTPRespLengthKey = "http.response.length"
	awsHTTPUserAgentKey  = "http.user_agent"
	awsHTTPRemoteIPKey   = "http.remote_ip"
	awsHTTPSchemeKey     = "http.scheme"
	awsHTTPProtoKey      = "http.proto"
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
	attributes := l.reqAttributes
	l.mu.Unlock()

	if !h.logAll && logCount == 0 {
		return
	}

	if sw.Status() > 399 && maxLevel < slog.LevelError {
		maxLevel = slog.LevelError
	}

	sc := trace.SpanFromContext(r.Context()).SpanContext()

	logAttr := []slog.Attr{
		slog.Any(awsTraceIDKey, xrayTraceID),
		slog.Any(awsSpanIDKey, sc.SpanID().String()),
		slog.String(awsHTTPElapsedKey, time.Since(begin).String()),
	}
	logAttr = append(logAttr, httpAttributes(r, sw)...)
	for k, v := range attributes {
		logAttr = append(logAttr, slog.Any(k, v))
	}

	h.logger.LogAttrs(r.Context(), maxLevel, parentLogEntry, logAttr...)
}

type awsLogger struct {
	parent        *awsLogger
	logger        awslog
	traceID       string
	rsvdKeys      []string
	rsvdReqKeys   []string
	attributes    map[string]any // attributes for child (trace) logs
	mu            sync.Mutex
	maxLevel      slog.Level
	logCount      int
	reqAttributes map[string]any // attributes for the parent request log
}

func newAWSLogger(logger awslog, traceID string) *awsLogger {
	l := &awsLogger{
		logger:   logger,
		traceID:  traceID,
		rsvdKeys: []string{awsTraceIDKey, awsSpanIDKey},
		rsvdReqKeys: []string{
			awsTraceIDKey, awsSpanIDKey,
			awsHTTPElapsedKey, awsHTTPMethodKey, awsHTTPURLKey, awsHTTPStatusCodeKey, awsHTTPRespLengthKey, awsHTTPUserAgentKey, awsHTTPRemoteIPKey, awsHTTPSchemeKey, awsHTTPProtoKey,
		},
		reqAttributes: make(map[string]any),
		attributes:    make(map[string]any),
	}
	l.parent = l

	return l
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

// AddRequestAttribute adds an attribute (key, value) for the parent request log
// If the key matches a reserved key, it will be prefixed with "custom_"
// If the key already exists, its value is overwritten
func (l *awsLogger) AddRequestAttribute(key string, value any) {
	if slices.Contains(l.rsvdReqKeys, key) {
		key = customPrefix + key
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.reqAttributes[key] = value
}

// WithAttribute adds the provided kv as a child (trace) log attribute and returns an attributer for adding additional attributes
// If the key matches a reserved key, it will be prefixed with "custom_"
// If the key already exists, its value is overwritten
func (l *awsLogger) WithAttribute(key string, value any) attributer {
	if slices.Contains(l.rsvdKeys, key) {
		key = customPrefix + key
	}

	attrs := make(map[string]any)
	for k, v := range l.attributes {
		attrs[k] = v
	}
	attrs[key] = value

	return &awsAttributer{logger: l, attributes: attrs}
}

func (l *awsLogger) log(ctx context.Context, level slog.Level, message string) {
	l.parent.mu.Lock()
	if l.parent.maxLevel < level {
		l.parent.maxLevel = level
	}
	l.parent.logCount++
	l.parent.mu.Unlock()

	span := trace.SpanFromContext(ctx)
	attr := []slog.Attr{
		slog.String(awsTraceIDKey, l.traceID),
		slog.String(awsSpanIDKey, span.SpanContext().SpanID().String()),
	}
	for k, v := range l.attributes {
		attr = append(attr, slog.Any(k, v))
	}
	l.logger.LogAttrs(ctx, level, message, attr...)
}

type awsAttributer struct {
	logger     *awsLogger
	attributes map[string]any
}

// AddAttribute adds an attribute (key, value) for the child (trace) log
// If the key matches a reserved key, it will be prefixed with "custom_"
// If the key already exists, its value is overwritten
func (a *awsAttributer) AddAttribute(key string, value any) {
	if slices.Contains(a.logger.rsvdKeys, key) {
		key = customPrefix + key
	}

	a.attributes[key] = value
}

// Logger returns a ctxLogger with the child (trace) attributes embedded
func (a *awsAttributer) Logger() ctxLogger {
	l := newAWSLogger(a.logger.logger, a.logger.traceID)
	l.parent = a.logger.parent
	for k, v := range a.attributes {
		l.attributes[k] = v
	}

	return l
}

// httpAttributes returns a slice of slog.Attr for the http request and response
func httpAttributes(r *http.Request, sw *statusWriter) []slog.Attr {
	return []slog.Attr{
		slog.String(awsHTTPMethodKey, r.Method),
		slog.String(awsHTTPURLKey, r.URL.String()),
		slog.Int(awsHTTPStatusCodeKey, sw.Status()),
		slog.Int64(awsHTTPRespLengthKey, sw.length),
		slog.String(awsHTTPUserAgentKey, r.UserAgent()),
		slog.String(awsHTTPRemoteIPKey, r.RemoteAddr),
		slog.String(awsHTTPSchemeKey, r.URL.Scheme),
		slog.String(awsHTTPProtoKey, r.Proto),
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
