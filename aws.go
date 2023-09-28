package logger

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"go.opentelemetry.io/otel/trace"
)

type AWSExporter struct {
	logAll bool
}

func NewAWSExporter() *AWSExporter {
	return &AWSExporter{
		logAll: true,
	}
}

func (e *AWSExporter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return &awsHandler{
			next:         next,
			parentLogger: slog.New(slog.NewJSONHandler(os.Stdout, nil)).WithGroup("request_parent_log"),
			childLogger:  slog.New(slog.NewJSONHandler(os.Stdout, nil)).WithGroup("request_child_log"),
			logAll:       e.logAll,
		}
	}
}

type awsHandler struct {
	next         http.Handler
	parentLogger *slog.Logger
	childLogger  *slog.Logger
	logAll       bool
}

func (h *awsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	traceID := r.Header.Get("X-Amzn-Trace-Id")
	l := newAWSLogger(h.childLogger, traceID)
	r = r.WithContext(newContext(r.Context(), l))
	sw := &statusWriter{ResponseWriter: w}

	h.next.ServeHTTP(sw, r)

	l.mu.Lock()
	logCount := l.logCount
	maxLevel := l.maxLevel
	l.mu.Unlock()

	if !h.logAll && logCount == 0 {
		return
	}

	if sw.Status() > 399 && maxLevel < slog.LevelError {
		maxLevel = slog.LevelError
	}

	sc := trace.SpanContextFromContext(r.Context())
	logAttr := []slog.Attr{
		slog.Any("trace", sc.TraceID().String()),
		slog.Any("spanID", sc.SpanID().String()),
	}
	logAttr = append(logAttr, httpRequestAttributes(r)...)
	h.parentLogger.LogAttrs(r.Context(), maxLevel, "Parent Log Entry", logAttr...)
}

type awsLogger struct {
	logger   *slog.Logger
	traceID  string
	mu       sync.Mutex
	maxLevel slog.Level
	logCount int
}

func newAWSLogger(logger *slog.Logger, traceID string) *awsLogger {
	return &awsLogger{
		logger:  logger,
		traceID: traceID,
	}
}

// Debug logs a debug message.
func (l *awsLogger) Debug(ctx context.Context, v interface{}) {
	l.log(ctx, slog.LevelDebug, fmt.Sprint(v))
}

// Debugf logs a debug message with format.
func (l *awsLogger) Debugf(ctx context.Context, format string, v ...interface{}) {
	l.log(ctx, slog.LevelDebug, fmt.Sprintf(format, v...))
}

// Info logs a info message.
func (l *awsLogger) Info(ctx context.Context, v interface{}) {
	l.log(ctx, slog.LevelInfo, fmt.Sprint(v))
}

// Infof logs a info message with format.
func (l *awsLogger) Infof(ctx context.Context, format string, v ...interface{}) {
	l.log(ctx, slog.LevelInfo, fmt.Sprintf(format, v...))
}

// Warn logs a warning message.
func (l *awsLogger) Warn(ctx context.Context, v interface{}) {
	l.log(ctx, slog.LevelWarn, fmt.Sprint(v))
}

// Warnf logs a warning message with format.
func (l *awsLogger) Warnf(ctx context.Context, format string, v ...interface{}) {
	l.log(ctx, slog.LevelWarn, fmt.Sprintf(format, v...))
}

// Error logs an error message.
func (l *awsLogger) Error(ctx context.Context, v interface{}) {
	l.log(ctx, slog.LevelError, fmt.Sprint(v))
}

// Errorf logs an error message with format.
func (l *awsLogger) Errorf(ctx context.Context, format string, v ...interface{}) {
	l.log(ctx, slog.LevelError, fmt.Sprintf(format, v...))
}

func (l *awsLogger) log(ctx context.Context, level slog.Level, message string) {
	l.mu.Lock()
	if l.maxLevel < level {
		l.maxLevel = level
	}
	l.logCount++
	l.mu.Unlock()

	span := trace.SpanFromContext(ctx)

	l.logger.Log(ctx, level, message, slog.Any("spanID", span.SpanContext().SpanID().String()))
}

func httpRequestAttributes(r *http.Request) []slog.Attr {
	return []slog.Attr{
		slog.String("http.method", r.Method),
		slog.String("http.host", r.Host),
		slog.String("http.target", r.URL.Path),
		slog.String("http.user_agent", r.UserAgent()),
		slog.String("http.scheme", r.URL.Scheme),
		slog.String("http.flavor", r.Proto),
		slog.String("http.client_ip", r.RemoteAddr),
		slog.Int64("http.request_content_length", r.ContentLength),
		slog.String("http.request_id", r.Header.Get("X-Request-Id")),
	}
}
