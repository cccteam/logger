package logger

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"testing"

	"cloud.google.com/go/logging"
	"github.com/go-test/deep"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestNewGoogleCloudExporter(t *testing.T) {
	t.Parallel()

	type args struct {
		client    *logging.Client
		projectID string
		opts      []logging.LoggerOption
	}
	tests := []struct {
		name string
		args args
		want *GoogleCloudExporter
	}{
		{
			name: "Simple Constructor",
			args: args{
				client:    &logging.Client{},
				projectID: "My Project ID",
				opts:      []logging.LoggerOption{logging.ConcurrentWriteLimit(5)},
			},
			want: &GoogleCloudExporter{
				projectID: "My Project ID",
				client:    &logging.Client{},
				opts:      []logging.LoggerOption{logging.ConcurrentWriteLimit(5)},
				logAll:    true,
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := NewGoogleCloudExporter(tt.args.client, tt.args.projectID, tt.args.opts...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewGoogleCloudExporter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGoogleCloudExporter_LogAll(t *testing.T) {
	t.Parallel()
	type fields struct {
		logAll bool
	}
	type args struct {
		v bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *GoogleCloudExporter
	}{
		{
			name: "logAll=true",
			args: args{
				v: true,
			},
			want: &GoogleCloudExporter{
				logAll: true,
			},
		},
		{
			name: "logAll=false",
			fields: fields{
				logAll: true,
			},
			args: args{
				v: false,
			},
			want: &GoogleCloudExporter{
				logAll: false,
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := &GoogleCloudExporter{
				logAll: tt.fields.logAll,
			}
			if got := e.LogAll(tt.args.v); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GoogleCloudExporter.LogAll() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGoogleCloudExporter_Middleware(t *testing.T) {
	disableMetaServertest(t)

	type fields struct {
		projectID string
		client    *logging.Client
		opts      []logging.LoggerOption
		logAll    bool
	}
	tests := []struct {
		name   string
		fields fields
		want   func(http.Handler) http.Handler
	}{
		{
			name: "call Middleware",
			fields: fields{
				projectID: "My other project",
				client:    &logging.Client{},
				opts:      []logging.LoggerOption{logging.ConcurrentWriteLimit(5)},
				logAll:    true,
			},
			want: func(next http.Handler) http.Handler {
				client := &logging.Client{}
				opts := []logging.LoggerOption{logging.ConcurrentWriteLimit(5)}

				return &gcpHandler{
					next:         next,
					parentLogger: client.Logger("request_parent_log", opts...),
					childLogger:  client.Logger("request_child_log", opts...),
					projectID:    "My other project",
					logAll:       true,
				}
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
			e := &GoogleCloudExporter{
				projectID: tt.fields.projectID,
				client:    tt.fields.client,
				opts:      tt.fields.opts,
				logAll:    tt.fields.logAll,
			}
			got := e.Middleware()(next)
			if diff := deep.Equal(got, tt.want(next)); diff != nil {
				t.Errorf("GoogleCloudExporter.Middleware() = %v", diff)
			}
		})
	}
}

func Test_gcpHandler_ServeHTTP(t *testing.T) {
	t.Parallel()

	type args struct {
		status int
		logs   int
		level  logging.Severity
	}
	type fields struct {
		projectID string
		logAll    bool
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		wantLevel logging.Severity
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
				level:  logging.Info,
			},
			wantLevel: logging.Info,
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
			wantLevel: logging.Default,
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
				level:  logging.Warning,
			},
			wantLevel: logging.Warning,
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
			wantLevel: logging.Error,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var handlerCalled bool
			var traceID string
			l := &captureLogger{}
			handler := &gcpHandler{
				parentLogger: l,
				childLogger:  &captureLogger{},
				projectID:    tt.fields.projectID,
				logAll:       tt.fields.logAll,
				next: http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						for i := 0; i < tt.args.logs; i++ {
							switch tt.args.level {
							case logging.Info:
								Req(r).Info("some log")
							case logging.Warning:
								Req(r).Warn("some log")
							case logging.Error:
								Req(r).Error("some log")
							default:
							}
						}

						l := Req(r)
						if l, ok := l.lg.(*gcpLogger); ok {
							traceID = l.traceID
						} else {
							t.Fatalf("Req() = %v, wanted: %T", l, &gcpLogger{})
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
			if l.e.Severity != tt.wantLevel {
				t.Errorf("Severity = %v, want %v", l.e.Severity, tt.wantLevel)
			}
			if l.e.Trace != traceID {
				t.Errorf("Trace = %v, want %v", l.e.Trace, traceID)
			}
			if pl, ok := l.e.Payload.(map[string]any); ok {
				if m, ok := pl["message"].(string); ok {
					if m != "Parent Log Entry" {
						t.Errorf("Message = %v, want %v", m, "Parent Log Entry")
					}
				} else {
					t.Fatalf("Message = %T, want %T", pl["message"], "")
				}
			} else {
				t.Fatalf("Payload = %T, want %T", l.e.Payload, map[string]any{})
			}
			if l.e.HTTPRequest.Status != tt.args.status {
				t.Errorf("Status = %v, want %v", l.e.HTTPRequest.Status, tt.args.status)
			}
		})
	}
}

func Test_gcpTraceIDFromRequest(t *testing.T) {
	t.Parallel()
	type args struct {
		mockReq   func(traceStr string) (*http.Request, string)
		projectID string
		traceStr  string
	}
	tests := []struct {
		name            string
		args            args
		wantTracePrefix string
		wantTraceStr    string
	}{
		// The order these are significant
		{
			// This test relies on the global tracing provider NOT being set
			name: "no trace in request",
			args: args{
				mockReq: func(wantTraceStr string) (*http.Request, string) {
					return &http.Request{URL: &url.URL{}}, wantTraceStr
				},
				projectID: "my-project",
				traceStr:  "105445aa7843bc8bf206b12000100000",
			},
			wantTracePrefix: "projects/my-project/traces/",
			wantTraceStr:    "105445aa7843bc8bf206b12000100000",
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
				projectID: "my-project",
			},
			wantTracePrefix: "projects/my-project/traces/",
		},
		{
			// With the global tracing provider set, this test shows that
			// trace Propagation is a higher priority then trace in request context
			name: "with propagation span in headers",
			args: args{
				mockReq: func(wantTraceStr string) (r *http.Request, traceStr string) {
					r = httptest.NewRequest(http.MethodGet, "/", http.NoBody)
					r.Header.Add("X-Cloud-Trace-Context", wantTraceStr+"/1;o=1")

					return r, wantTraceStr
				},
				projectID: "my-project",
			},
			wantTracePrefix: "projects/my-project/traces/",
			wantTraceStr:    "105445aa7843bc8bf206b12000100000",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r, traceStr := tt.args.mockReq(tt.wantTraceStr)
			want := tt.wantTracePrefix + traceStr

			if got := gcpTraceIDFromRequest(r, tt.args.projectID, func() string { return tt.args.traceStr }); got != want {
				t.Errorf("gcpTraceIDFromRequest() = %v, want %v", got, want)
			}
		})
	}
}

func Test_newGCPLogger(t *testing.T) {
	t.Parallel()

	type args struct {
		lg      *logging.Logger
		traceID string
	}
	tests := []struct {
		name string
		args args
		want ctxLogger
	}{
		{
			name: "new",
			args: args{
				lg:      &logging.Logger{},
				traceID: "hello",
			},
			want: &gcpLogger{
				lg:      &logging.Logger{},
				traceID: "hello",
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := newGCPLogger(tt.args.lg, tt.args.traceID); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_gcpLogger(t *testing.T) {
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

			l := &gcpLogger{
				lg: &testLogger{
					buf: &buf,
				},
			}

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

func disableMetaServertest(t *testing.T) {
	t.Helper()

	// Fix issue when logging.Client attempts to detect its
	// env by querying GCE_METADATA_HOST and nothing is there
	// so your test is very slow. This tries to causes the
	// detection to fail faster and not hang your test so long
	curEnv := os.Getenv("GCE_METADATA_HOST")
	t.Cleanup(func() { os.Setenv("GCE_METADATA_HOST", curEnv) })
	_ = os.Setenv("GCE_METADATA_HOST", "localhost")
}

type testLogger struct {
	buf *bytes.Buffer
}

func (t *testLogger) Log(e logging.Entry) {
	_, _ = t.buf.WriteString(e.Payload.(map[string]any)["message"].(string))
}

type captureLogger struct {
	e logging.Entry
}

func (c *captureLogger) Log(e logging.Entry) {
	c.e = e
}
