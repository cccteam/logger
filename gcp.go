package logger

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"cloud.google.com/go/logging"
	"contrib.go.opencensus.io/exporter/stackdriver/propagation"
	"go.opentelemetry.io/otel/trace"
)

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
	attributes := l.attributes
	l.mu.Unlock()

	if !g.logAll && logCount == 0 {
		return
	}

	// status code should also set the minimum maxSeverity to Error
	if sw.Status() > 399 && maxSeverity < logging.Error {
		maxSeverity = logging.Error
	}

	sc := trace.SpanFromContext(r.Context()).SpanContext()

	msg := parentLogEntry
	for k, v := range attributes {
		msg += fmt.Sprintf(", %s: %v", k, v)
	}

	g.parentLogger.Log(logging.Entry{
		Timestamp:    begin,
		Severity:     maxSeverity,
		Trace:        traceID,
		SpanID:       sc.SpanID().String(),
		TraceSampled: sc.IsSampled(),
		Payload: map[string]any{
			"message": msg,
		},
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
	lg          logger
	traceID     string
	mu          sync.Mutex
	maxSeverity logging.Severity
	logCount    int
	attributes  map[string]any
}

func newGCPLogger(lg logger, traceID string) *gcpLogger {
	return &gcpLogger{
		lg:      lg,
		traceID: traceID,
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

// AddAttributes adds attributes to include in middleware-driven logs
func (l *gcpLogger) AddAttributes(attrbs map[string]any) {
	if l.attributes == nil {
		l.attributes = make(map[string]any)
	}

	for k, v := range attrbs {
		l.attributes[k] = v
	}
}

func (l *gcpLogger) log(ctx context.Context, severity logging.Severity, p any) {
	l.mu.Lock()
	if l.maxSeverity < severity {
		l.maxSeverity = severity
	}
	l.logCount++
	l.mu.Unlock()

	if err, ok := p.(error); ok {
		p = err.Error()
	}

	span := trace.SpanFromContext(ctx)

	l.lg.Log(
		logging.Entry{
			Payload: map[string]any{
				"message": p,
			},
			Severity:     severity,
			Trace:        l.traceID,
			SpanID:       span.SpanContext().SpanID().String(),
			TraceSampled: span.SpanContext().IsSampled(),
		},
	)
}
