package logger

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"time"

	"cloud.google.com/go/logging"
	"contrib.go.opencensus.io/exporter/stackdriver/propagation"
	"go.opentelemetry.io/otel/trace"
)

const gcpMessageKey = "message"

// GoogleCloudExporter implements exporting to Google Cloud Logging
type GoogleCloudExporter struct {
	projectID string
	client    *logging.Client
	opts      []logging.LoggerOption
	logAll    bool
}

// NewGoogleCloudExporter returns a configured GoogleCloudExporter
func NewGoogleCloudExporter(client *logging.Client, projectID string, opts ...logging.LoggerOption) *GoogleCloudExporter {
	return &GoogleCloudExporter{
		projectID: projectID,
		client:    client,
		opts:      opts,
		logAll:    true,
	}
}

// LogAll controls if this logger will log all requests, or only requests that contain
// logs written to the request Logger (default: true)
func (e *GoogleCloudExporter) LogAll(v bool) *GoogleCloudExporter {
	e.logAll = v

	return e
}

// Middleware returns a middleware that exports logs to Google Cloud Logging
func (e *GoogleCloudExporter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return &gcpHandler{
			next:         next,
			parentLogger: e.client.Logger("request_parent_log", e.opts...),
			childLogger:  e.client.Logger("request_child_log", e.opts...),
			projectID:    e.projectID,
			logAll:       e.logAll,
		}
	}
}

type gcpHandler struct {
	next         http.Handler
	parentLogger logger
	childLogger  logger
	projectID    string
	logAll       bool
}

func (g *gcpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	begin := time.Now()
	traceID := gcpTraceIDFromRequest(r, g.projectID, generateID)
	l := newGCPLogger(g.childLogger, traceID)
	r = r.WithContext(newContext(r.Context(), l))
	sw := &statusWriter{ResponseWriter: w}

	g.next.ServeHTTP(sw, r)

	l.mu.Lock()
	logCount := l.logCount
	maxSeverity := l.maxSeverity
	attributes := make(map[string]any)
	for k, v := range l.reqAttributes {
		attributes[k] = v
	}
	l.mu.Unlock()

	if !g.logAll && logCount == 0 {
		return
	}

	// status code should also set the minimum maxSeverity to Error
	if sw.Status() > 399 && maxSeverity < logging.Error {
		maxSeverity = logging.Error
	}

	sc := trace.SpanFromContext(r.Context()).SpanContext()

	attributes[gcpMessageKey] = parentLogEntry

	g.parentLogger.Log(logging.Entry{
		Timestamp:    begin,
		Severity:     maxSeverity,
		Trace:        traceID,
		SpanID:       sc.SpanID().String(),
		TraceSampled: sc.IsSampled(),
		Payload:      attributes,
		HTTPRequest: &logging.HTTPRequest{
			Request:      r,
			RequestSize:  requestSize(r.Header.Get("Content-Length")),
			Latency:      time.Since(begin),
			Status:       sw.Status(),
			ResponseSize: sw.length,
			RemoteIP:     r.Header.Get("X-Forwarded-For"),
		},
	})
}

// gcpTraceIDFromRequest formats a trace_id value for GCP Stackdriver
func gcpTraceIDFromRequest(r *http.Request, projectID string, idgen func() string) string {
	var traceID string
	if sc := trace.SpanFromContext(r.Context()).SpanContext(); sc.IsValid() {
		traceID = sc.TraceID().String()
	} else {
		if sc1, ok := new(propagation.HTTPFormat).SpanContextFromRequest(r); ok {
			traceID = sc1.TraceID.String()
		} else {
			traceID = idgen()
		}
	}

	return fmt.Sprintf("projects/%s/traces/%s", projectID, traceID)
}

// logger interface exists for testability
type logger interface {
	Log(e logging.Entry)
}

type gcpLogger struct {
	root          *gcpLogger
	logger        logger
	traceID       string
	rsvdKeys      []string
	attributes    map[string]any // attributes for child (trace) logs
	mu            sync.Mutex
	maxSeverity   logging.Severity
	logCount      int
	reqAttributes map[string]any // attributes for the parent request log
}

func newGCPLogger(lg logger, traceID string) *gcpLogger {
	l := &gcpLogger{
		logger:        lg,
		traceID:       traceID,
		rsvdKeys:      []string{gcpMessageKey},
		reqAttributes: make(map[string]any),
		attributes:    make(map[string]any),
	}
	l.root = l // root is self

	return l
}

// newChild returns a new child gcpLogger
func (l *gcpLogger) newChild() *gcpLogger {
	return &gcpLogger{
		root:          l.root,
		logger:        l.logger,
		traceID:       l.traceID,
		rsvdKeys:      l.rsvdKeys,
		reqAttributes: make(map[string]any),
		attributes:    make(map[string]any),
	}
}

// Debug logs a debug message.
func (l *gcpLogger) Debug(ctx context.Context, v any) {
	l.log(ctx, logging.Debug, v)
}

// Debugf logs a debug message with format.
func (l *gcpLogger) Debugf(ctx context.Context, format string, v ...any) {
	l.log(ctx, logging.Debug, fmt.Sprintf(format, v...))
}

// Info logs a info message.
func (l *gcpLogger) Info(ctx context.Context, v any) {
	l.log(ctx, logging.Info, v)
}

// Infof logs a info message with format.
func (l *gcpLogger) Infof(ctx context.Context, format string, v ...any) {
	l.log(ctx, logging.Info, fmt.Sprintf(format, v...))
}

// Warn logs a warning message.
func (l *gcpLogger) Warn(ctx context.Context, v any) {
	l.log(ctx, logging.Warning, v)
}

// Warnf logs a warning message with format.
func (l *gcpLogger) Warnf(ctx context.Context, format string, v ...any) {
	l.log(ctx, logging.Warning, fmt.Sprintf(format, v...))
}

// Error logs an error message.
func (l *gcpLogger) Error(ctx context.Context, v any) {
	l.log(ctx, logging.Error, v)
}

// Errorf logs an error message with format.
func (l *gcpLogger) Errorf(ctx context.Context, format string, v ...any) {
	l.log(ctx, logging.Error, fmt.Sprintf(format, v...))
}

// AddRequestAttribute adds an attribute (key, value) for the parent request log
// If the key matches a reserved key, it will be prefixed with "custom_"
// If the key already exists, its value is overwritten
func (l *gcpLogger) AddRequestAttribute(key string, value any) {
	if slices.Contains(l.rsvdKeys, key) {
		key = customPrefix + key
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.reqAttributes[key] = value
}

// WithAttribute adds the provided kv as a child (trace) log attribute and returns an attributer for adding additional attributes
// If the key matches a reserved key, it will be prefixed with "custom_"
// If the key already exists, its value is overwritten
func (l *gcpLogger) WithAttribute(key string, value any) attributer {
	if slices.Contains(l.rsvdKeys, key) {
		key = customPrefix + key
	}

	attrs := make(map[string]any)
	for k, v := range l.attributes {
		attrs[k] = v
	}
	attrs[key] = value

	return &gcpAttributer{logger: l, attributes: attrs}
}

func (l *gcpLogger) log(ctx context.Context, severity logging.Severity, msg any) {
	l.root.mu.Lock()
	if l.root.maxSeverity < severity {
		l.root.maxSeverity = severity
	}
	l.root.logCount++
	l.root.mu.Unlock()

	if err, ok := msg.(error); ok {
		msg = err.Error()
	}

	span := trace.SpanFromContext(ctx)
	attrs := make(map[string]any)
	for k, v := range l.attributes {
		attrs[k] = v
	}
	attrs[gcpMessageKey] = msg

	l.logger.Log(
		logging.Entry{
			Payload:      attrs,
			Severity:     severity,
			Trace:        l.traceID,
			SpanID:       span.SpanContext().SpanID().String(),
			TraceSampled: span.SpanContext().IsSampled(),
		},
	)
}

type gcpAttributer struct {
	logger     *gcpLogger
	attributes map[string]any
}

// AddAttribute adds an attribute (key, value) for the child (trace) log
// If the key matches a reserved key, it will be prefixed with "custom_"
// If the key already exists, its value is overwritten
func (a *gcpAttributer) AddAttribute(key string, value any) {
	if slices.Contains(a.logger.rsvdKeys, key) {
		key = customPrefix + key
	}

	a.attributes[key] = value
}

// Logger returns a ctxLogger with the child (trace) attributes embedded
func (a *gcpAttributer) Logger() ctxLogger {
	l := a.logger.newChild()
	for k, v := range a.attributes {
		l.attributes[k] = v
	}

	return l
}
