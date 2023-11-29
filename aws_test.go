package logger

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestNewAWSExporter(t *testing.T) {
	t.Parallel()
	type args struct {
		logAll bool
	}
	tests := []struct {
		name string
		args args
		want *AWSExporter
	}{
		{
			name: "TestNewAWSExporter with logall true",
			args: args{
				logAll: true,
			},
			want: &AWSExporter{
				logAll: true,
			},
		},
		{
			name: "TestNewAWSExporter with logall false",
			args: args{
				logAll: false,
			},
			want: &AWSExporter{
				logAll: false,
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := NewAWSExporter(tt.args.logAll)

			if diff := cmp.Diff(got, tt.want, cmp.AllowUnexported(AWSExporter{})); diff != "" {
				t.Errorf("NewAWSExporter() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAWSExporter_Middleware(t *testing.T) {
	t.Parallel()

	type fields struct {
		logAll bool
	}
	tests := []struct {
		name   string
		fields fields
		want   func(http.Handler) http.Handler
	}{
		{
			name: "TestAWSExporter_Middleware",
			fields: fields{
				logAll: true,
			},
			want: func(next http.Handler) http.Handler {
				return &awsHandler{
					next:   next,
					logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)).WithGroup("request_parent_log"),
					logAll: true,
				}
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
			e := &AWSExporter{
				logAll: tt.fields.logAll,
			}

			got := e.Middleware()(next)
			if diff := cmp.Diff(got, tt.want(next), cmpopts.IgnoreUnexported(awsHandler{})); diff != "" {
				t.Errorf("AWSExporter.Middleware() = %s", diff)
			}
		})
	}
}

func Test_awsHandler_ServeHTTP(t *testing.T) {
	t.Parallel()

	type args struct {
		status int
		logs   int
		level  slog.Level
	}
	type fields struct {
		projectID string
		logAll    bool
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		wantLevel slog.Level
	}{
		{
			name: "logAll=true",
			fields: fields{
				projectID: "my-big-project",
				logAll:    true,
			},
			args: args{
				status: http.StatusOK,
				logs:   1,
				level:  slog.LevelInfo,
			},
			wantLevel: slog.LevelInfo,
		},
		{
			name: "logAll=true no logging",
			fields: fields{
				projectID: "my-big-project",
				logAll:    true,
			},
			args: args{
				status: http.StatusOK,
			},
			wantLevel: slog.LevelInfo,
		},
		{
			name: "logAll=false no logging",
			fields: fields{
				projectID: "my-big-project",
			},
			args: args{
				status: http.StatusOK,
			},
		},
		{
			name: "logAll=false with logging",
			fields: fields{
				projectID: "my-bigger-project",
			},
			args: args{
				status: http.StatusOK,
				logs:   1,
				level:  slog.LevelWarn,
			},
			wantLevel: slog.LevelWarn,
		},
		{
			name: "logAll=true no logging",
			fields: fields{
				projectID: "my-big-project",
				logAll:    true,
			},
			args: args{
				status: http.StatusInternalServerError,
			},
			wantLevel: slog.LevelError,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var handlerCalled bool
			l := &captureSLogger{}
			handler := &awsHandler{
				logger: l,
				logAll: tt.fields.logAll,
				next: http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						for i := 0; i < tt.args.logs; i++ {
							switch tt.args.level {
							case slog.LevelInfo:
								Req(r).Info("some log")
							case slog.LevelWarn:
								Req(r).Warn("some log")
							case slog.LevelError:
								Req(r).Error("some log")
							default:
							}
						}

						w.WriteHeader(tt.args.status)
						handlerCalled = true
					},
				),
			}

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			handler.ServeHTTP(w, r)

			if !handlerCalled {
				t.Errorf("Failed to call handler")
			}
			if tt.args.logs == 0 {
				return
			}
			if l.level != tt.wantLevel {
				t.Errorf("Level = %v, want %v", l.level, tt.wantLevel)
			}

			if l.attrs == nil {
				t.Errorf("Attrs = %v, want %v", l.attrs, "not nil")
			}

			if pl := l.msg; pl != "" {
				if pl != "Parent Log Entry" {
					t.Errorf("Message = %v, want %v", pl, "Parent Log Entry")
				}
			}
		})
	}
}

func Test_awsTraceIDFromRequest(t *testing.T) {
	t.Parallel()
	type args struct {
		mockReq  func(traceStr string) (*http.Request, string)
		traceStr string
	}
	tests := []struct {
		name         string
		args         args
		wantTraceStr string
		wantBlankStr bool
	}{
		// The order these are significant
		{
			// This test relies on the global tracing provider NOT being set
			name: "no trace in request",
			args: args{
				mockReq: func(wantTraceStr string) (*http.Request, string) {
					return &http.Request{URL: &url.URL{}}, wantTraceStr
				},
				traceStr: "105445aa7843bc8bf206b12000100000",
			},
			wantTraceStr: "105445aa7843bc8bf206b12000100000",
			wantBlankStr: false,
		},
		{
			// This test sets the global tracing provider (I don't think this can be un-done)
			name: "with trace in request",
			args: args{
				mockReq: func(_ string) (r *http.Request, traceStr string) {
					otel.SetTracerProvider(sdktrace.NewTracerProvider())
					ctx, span := otel.Tracer("test/examples").Start(context.Background(), "test trace")

					r = httptest.NewRequest(http.MethodGet, "/", http.NoBody)
					r = r.WithContext(ctx)

					return r, span.SpanContext().TraceID().String()
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r, want := tt.args.mockReq(tt.wantTraceStr)

			if got := awsTraceIDFromRequest(r, func() string { return tt.args.traceStr }); got != want && (got == "0000000000000000") != tt.wantBlankStr {
				t.Errorf("awsTraceIDFromRequest() = %v, want %v", got, want)
			}
		})
	}
}

func Test_newAWSLogger(t *testing.T) {
	t.Parallel()

	type args struct {
		logger  awslog
		traceID string
	}
	tests := []struct {
		name string
		args args
		want *awsLogger
	}{
		{
			name: "Test_newAWSLogger",
			args: args{
				logger:  &testSlogger{},
				traceID: "1234567890",
			},
			want: &awsLogger{
				logger:        &testSlogger{},
				traceID:       "1234567890",
				reservedKeys:  []string{"trace_id", "span_id", "http.elapsed", "http.method", "http.url", "http.status_code", "http.response.length", "http.user_agent", "http.remote_ip", "http.scheme", "http.proto"},
				reqAttributes: map[string]any{},
				attributes:    map[string]any{},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := newAWSLogger(tt.args.logger, tt.args.traceID)
			if diff := cmp.Diff(got, tt.want, cmpopts.IgnoreFields(awsLogger{}, "logger", "mu", "parent"), cmp.AllowUnexported(awsLogger{})); diff != "" {
				t.Errorf("newAWSLogger() mismatch (-want +got):\n%s", diff)
			}
			if got.parent != got {
				t.Errorf("newAWSLogger().parent is not self")
			}
		})
	}
}

func Test_awsLogger(t *testing.T) {
	t.Parallel()

	type args struct {
		format string
		v      []any
		v2     any
	}
	tests := []struct {
		name       string
		args       args
		wantDebug  string
		wantDebugf string
		wantInfo   string
		wantInfof  string
		wantWarn   string
		wantWarnf  string
		wantError  string
		wantErrorf string
	}{
		{
			name: "Strings",
			args: args{
				format: "Formatted %s",
				v:      []any{"Message"},
				v2:     "Message",
			},
			wantDebug:  "Message",
			wantDebugf: "Formatted Message",
			wantInfo:   "Message",
			wantInfof:  "Formatted Message",
			wantWarn:   "Message",
			wantWarnf:  "Formatted Message",
			wantError:  "Message",
			wantErrorf: "Formatted Message",
		},
		{
			name: "String & Error",
			args: args{
				format: "Formatted %s",
				v:      []any{"Message"},
				v2:     errors.New("Message"),
			},
			wantDebug:  "Message",
			wantDebugf: "Formatted Message",
			wantInfo:   "Message",
			wantInfof:  "Formatted Message",
			wantWarn:   "Message",
			wantWarnf:  "Formatted Message",
			wantError:  "Message",
			wantErrorf: "Formatted Message",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			var buf bytes.Buffer

			l := &awsLogger{
				logger: &testSlogger{
					buf: &buf,
				},
			}
			l.parent = l

			l.Debug(ctx, tt.args.v2)
			if s := buf.String(); s != tt.wantDebug {
				t.Errorf("stdErrLogger.Debug() value = %v, wantValue %v", s, tt.wantDebug)
			}
			buf.Reset()

			l.Debugf(ctx, tt.args.format, tt.args.v...)
			if s := buf.String(); s != tt.wantDebugf {
				t.Errorf("stdErrLogger.Debug() value = %v, wantValue %v", s, tt.wantDebugf)
			}
			buf.Reset()

			l.Info(ctx, tt.args.v2)
			if s := buf.String(); s != tt.wantInfo {
				t.Errorf("stdErrLogger.Info() value = %v, wantValue %v", s, tt.wantInfo)
			}
			buf.Reset()

			l.Infof(ctx, tt.args.format, tt.args.v...)
			if s := buf.String(); s != tt.wantInfof {
				t.Errorf("stdErrLogger.Info() value = %v, wantValue %v", s, tt.wantInfof)
			}
			buf.Reset()

			l.Warn(ctx, tt.args.v2)
			if s := buf.String(); s != tt.wantWarn {
				t.Errorf("stdErrLogger.Warn() value = %v, wantValue %v", s, tt.wantWarn)
			}
			buf.Reset()

			l.Warnf(ctx, tt.args.format, tt.args.v...)
			if s := buf.String(); s != tt.wantWarnf {
				t.Errorf("stdErrLogger.Warn() value = %v, wantValue %v", s, tt.wantWarnf)
			}
			buf.Reset()

			l.Error(ctx, tt.args.v2)
			if s := buf.String(); s != tt.wantError {
				t.Errorf("stdErrLogger.Error() value = %v, wantValue %v", s, tt.wantError)
			}
			buf.Reset()

			l.Errorf(ctx, tt.args.format, tt.args.v...)
			if s := buf.String(); s != tt.wantErrorf {
				t.Errorf("stdErrLogger.Error() value = %v, wantValue %v", s, tt.wantErrorf)
			}
			buf.Reset()
		})
	}
}

type testSlogger struct {
	buf *bytes.Buffer
}

func (t *testSlogger) LogAttrs(_ context.Context, _ slog.Level, msg string, _ ...slog.Attr) {
	_, _ = t.buf.WriteString(msg)
}

type captureSLogger struct {
	ctx   context.Context
	level slog.Level
	msg   string
	attrs []slog.Attr
}

func (c *captureSLogger) LogAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	c.ctx = ctx
	c.level = level
	c.msg = msg
	c.attrs = attrs
}
