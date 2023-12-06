package logger

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"cloud.google.com/go/logging"
	"github.com/go-test/deep"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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
			got := NewGoogleCloudExporter(tt.args.client, tt.args.projectID, tt.args.opts...)
			if diff := cmp.Diff(got, tt.want, cmp.AllowUnexported(GoogleCloudExporter{}, logging.Client{}), cmpopts.IgnoreFields(logging.Client{}, "client", "loggers", "mu")); diff != "" {
				t.Errorf("NewGoogleCloudExporter() mismatch (-want +got):\n%s", diff)
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
			got := e.LogAll(tt.args.v)
			if diff := cmp.Diff(got, tt.want, cmp.AllowUnexported(GoogleCloudExporter{})); diff != "" {
				t.Errorf("GoogleCloudExporter.LogAll() mismatch (-want +got):\n%s", diff)
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
			name: "logging for error status",
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

						gcpLgr, ok := Req(r).lg.(*gcpLogger)
						if !ok {
							t.Fatalf("Req() = %v, wanted: %T", gcpLgr, &gcpLogger{})
						}
						traceID = gcpLgr.traceID
						gcpLgr.reqAttributes["test_key_1"] = "test_value_1"
						gcpLgr.reqAttributes["test_key_2"] = "test_value_2"

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
			if !tt.fields.logAll && tt.args.logs == 0 {
				return
			}
			if l.e.Severity != tt.wantLevel {
				t.Errorf("Severity = %v, want %v", l.e.Severity, tt.wantLevel)
			}
			if l.e.Trace != traceID {
				t.Errorf("Trace = %v, want %v", l.e.Trace, traceID)
			}

			wantPayload := map[string]any{
				"message":    "Parent Log Entry",
				"test_key_1": "test_value_1",
				"test_key_2": "test_value_2",
			}
			if pl, ok := l.e.Payload.(map[string]any); ok {
				if diff := cmp.Diff(pl, wantPayload); diff != "" {
					t.Errorf("Payload mismatch (-want +got):\n%s", diff)
				}
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
				logger:        &logging.Logger{},
				traceID:       "hello",
				rsvdKeys:      []string{"message"},
				reqAttributes: map[string]any{},
				attributes:    map[string]any{},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := newGCPLogger(tt.args.lg, tt.args.traceID)
			if diff := cmp.Diff(got, tt.want, cmp.AllowUnexported(gcpLogger{}), cmpopts.IgnoreFields(gcpLogger{}, "logger", "mu", "root")); diff != "" {
				t.Errorf("newGCPLogger() mismatch (-want +got):\n%s", diff)
			}
			if got.root != got {
				t.Errorf("newGCPLogger().root is not self")
			}
		})
	}
}

func Test_gcpLogger(t *testing.T) {
	t.Parallel()

	type fields struct {
		attributes map[string]any
		traceID    string
	}
	type args struct {
		format string
		v      []any
		v2     any
	}
	tests := []struct {
		name       string
		fields     fields
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
			fields: fields{
				attributes: map[string]any{"a test key": "a test value"},
				traceID:    "123987",
			},
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
			fields: fields{
				attributes: map[string]any{"test_key_1": "test_value_1", "test_key_2": "test_value_2"},
				traceID:    "987123",
			},
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

			ctx, span := otel.Tracer("test tracer").Start(context.Background(), "a test")
			var buf bytes.Buffer

			l := &gcpLogger{
				logger: &testLogger{
					buf: &buf,
				},
				attributes: tt.fields.attributes,
				traceID:    tt.fields.traceID,
			}
			l.root = l

			verifyOutput := func(output, methodName, expectedMsgVal string, expectedSeverity logging.Severity) {
				expectedVals := []string{
					"message=" + expectedMsgVal,
					"trace=" + tt.fields.traceID,
					"severity=" + expectedSeverity.String(),
					"span=" + span.SpanContext().SpanID().String(),
					"trace_sampled=" + fmt.Sprint(span.SpanContext().IsSampled()),
				}
				for k, v := range tt.fields.attributes {
					expectedVals = append(expectedVals, slog.Any(k, v).String())
				}
				for _, v := range expectedVals {
					if !strings.Contains(output, v) {
						t.Errorf("gcpLogger.%s() = %q, missing: %q", methodName, output, v)
					}
				}
			}

			l.Debug(ctx, tt.args.v2)
			verifyOutput(buf.String(), "Debug", tt.wantDebug, logging.Debug)
			buf.Reset()

			l.Debugf(ctx, tt.args.format, tt.args.v...)
			verifyOutput(buf.String(), "Debugf", tt.wantDebugf, logging.Debug)
			buf.Reset()

			l.Info(ctx, tt.args.v2)
			verifyOutput(buf.String(), "Info", tt.wantInfo, logging.Info)
			buf.Reset()

			l.Infof(ctx, tt.args.format, tt.args.v...)
			verifyOutput(buf.String(), "Infof", tt.wantInfof, logging.Info)
			buf.Reset()

			l.Warn(ctx, tt.args.v2)
			verifyOutput(buf.String(), "Warn", tt.wantWarn, logging.Warning)
			buf.Reset()

			l.Warnf(ctx, tt.args.format, tt.args.v...)
			verifyOutput(buf.String(), "Warnf", tt.wantWarnf, logging.Warning)
			buf.Reset()

			l.Error(ctx, tt.args.v2)
			verifyOutput(buf.String(), "Error", tt.wantError, logging.Error)
			buf.Reset()

			l.Errorf(ctx, tt.args.format, tt.args.v...)
			verifyOutput(buf.String(), "Errorf", tt.wantErrorf, logging.Error)
			buf.Reset()
		})
	}
}

func Test_gcpLogger_AddRequestAttribute(t *testing.T) {
	t.Parallel()
	type fields struct {
		root     *gcpLogger
		rsvdKeys []string
	}
	type args struct {
		key   string
		value any
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   map[string]any
	}{
		{
			name: "prefix reserved key",
			fields: fields{
				root: &gcpLogger{
					reqAttributes: map[string]any{"test_key_2": "test_value_2"},
				},
				rsvdKeys: []string{"test_key 1", "test_key"},
			},
			args: args{
				key:   "test_key",
				value: 512,
			},
			want: map[string]any{"test_key_2": "test_value_2", "custom_test_key": 512},
		},
		{
			name: "add request attribute (non-reserved key)",
			fields: fields{
				root: &gcpLogger{
					reqAttributes: map[string]any{"test_key_2": "test_value_2"},
				},
				rsvdKeys: []string{"test_key 1"},
			},
			args: args{
				key:   "test_key",
				value: 512,
			},
			want: map[string]any{"test_key_2": "test_value_2", "test_key": 512},
		},
		{
			name: "overwrite request attribute value",
			fields: fields{
				root: &gcpLogger{
					reqAttributes: map[string]any{"test_key_2": "test_value_2"},
				},
				rsvdKeys: []string{"test_key 1"},
			},
			args: args{
				key:   "test_key_2",
				value: 512,
			},
			want: map[string]any{"test_key_2": 512},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			l := &gcpLogger{
				root:     tt.fields.root,
				rsvdKeys: tt.fields.rsvdKeys,
			}
			l.AddRequestAttribute(tt.args.key, tt.args.value)
			if diff := cmp.Diff(l.root.reqAttributes, tt.want); diff != "" {
				t.Errorf("gcpLogger.AddRequestAttribute() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_gcpLogger_WithAttributes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		attributes map[string]any
		want       *gcpAttributer
	}{
		{
			name: "with attributes success",
			attributes: map[string]any{
				"test_key_1": "test_value_1",
				"test_key_2": "test_value_2",
			},
			want: &gcpAttributer{
				attributes: map[string]any{
					"test_key_1": "test_value_1",
					"test_key_2": "test_value_2",
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			l := &gcpLogger{
				attributes: tt.attributes,
			}
			got := l.WithAttributes()
			if diff := cmp.Diff(got, tt.want, cmp.AllowUnexported(gcpAttributer{}), cmpopts.IgnoreFields(gcpAttributer{}, "logger")); diff != "" {
				t.Errorf("gcpLogger.WithAttributes() mismatch (-want +got):\n%s", diff)
			}
			if a, ok := got.(*gcpAttributer); !ok {
				t.Errorf("gcpLogger.WithAttributes() type %T, want %T", got, &gcpAttributer{})
			} else if a.logger != l {
				t.Errorf("gcpLogger.WithAttributes().logger != gcpLogger")
			}
		})
	}
}

func Test_gcpAttributer_AddAttribute(t *testing.T) {
	t.Parallel()
	type args struct {
		key   string
		value any
	}
	tests := []struct {
		name       string
		args       args
		rsvdKeys   []string
		attributes map[string]any
		want       map[string]any
	}{
		{
			name: "prefix reserved key",
			args: args{
				key:   "test_key_0",
				value: "test_value_0",
			},
			rsvdKeys: []string{"test_key 0", "test_key_0"},
			attributes: map[string]any{
				"test_key_1": 1,
				"test_key_2": "test_value_2",
			},
			want: map[string]any{
				"test_key_1":        1,
				"test_key_2":        "test_value_2",
				"custom_test_key_0": "test_value_0",
			},
		},
		{
			name: "add attribute (non-reserved key)",
			args: args{
				key:   "test_key_0",
				value: "test_value_0",
			},
			rsvdKeys: []string{"test_key 0"},
			attributes: map[string]any{
				"test_key_1": 1,
				"test_key_2": "test_value_2",
			},
			want: map[string]any{
				"test_key_1": 1,
				"test_key_2": "test_value_2",
				"test_key_0": "test_value_0",
			},
		},
		{
			name: "overwrite attribute value",
			args: args{
				key:   "test_key_1",
				value: 512,
			},
			rsvdKeys: []string{"test_key 0"},
			attributes: map[string]any{
				"test_key_1": 1,
				"test_key_2": "test_value_2",
			},
			want: map[string]any{
				"test_key_1": 512,
				"test_key_2": "test_value_2",
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := &gcpAttributer{
				attributes: tt.attributes,
				logger:     &gcpLogger{rsvdKeys: tt.rsvdKeys},
			}
			a.AddAttribute(tt.args.key, tt.args.value)
			if diff := cmp.Diff(a.attributes, tt.want); diff != "" {
				t.Errorf("gcpAttributer.AddAttribute() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_gcpAttributer_Logger(t *testing.T) {
	t.Parallel()
	type fields struct {
		logger     *gcpLogger
		attributes map[string]any
	}
	tests := []struct {
		name string
		fields
		want *gcpLogger
	}{
		{
			name: "success getting logger",
			fields: fields{
				logger: &gcpLogger{
					root: &gcpLogger{
						traceID: "root trace id",
					},
					logger:        &testLogger{},
					traceID:       "1234567890",
					rsvdKeys:      []string{"test reserved key 1", "test reserved key 2"},
					attributes:    map[string]any{"test_key_1": "test_value_1", "test_key_2": "test_value_2"},
					maxSeverity:   logging.Warning,
					logCount:      2,
					reqAttributes: map[string]any{"test_req_key_1": "test_req_value_1", "test_req_key_2": "test_req_value_2"},
				},
				attributes: map[string]any{"test_key_3": "test_value_3", "test_key_4": "test_value_4"},
			},
			want: &gcpLogger{
				root: &gcpLogger{
					traceID: "root trace id",
				},
				traceID:       "1234567890",
				rsvdKeys:      []string{"test reserved key 1", "test reserved key 2"},
				attributes:    map[string]any{"test_key_3": "test_value_3", "test_key_4": "test_value_4"},
				maxSeverity:   logging.Default,
				logCount:      0,
				reqAttributes: nil,
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := &gcpAttributer{
				logger:     tt.fields.logger,
				attributes: tt.fields.attributes,
			}

			got := a.Logger()
			if diff := cmp.Diff(got, tt.want, cmp.AllowUnexported(gcpLogger{}), cmpopts.IgnoreFields(gcpLogger{}, "mu", "logger")); diff != "" {
				t.Errorf("gcpAttributer.Logger() mismatch (-want +got):\n%s", diff)
			}
			gotGcpLogger, ok := got.(*gcpLogger)
			if !ok {
				t.Errorf("gcpAttributer.Logger() type %T, want %T", got, &gcpLogger{})
				return
			}
			if gotGcpLogger.logger != a.logger.logger {
				t.Errorf("got gcpLogger.logger is NOT the original logger")
			}
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
	logStr := "trace=" + e.Trace + " severity=" + e.Severity.String() + " span=" + e.SpanID + " trace_sampled=" + fmt.Sprint(e.TraceSampled)
	attrs, ok := e.Payload.(map[string]any)
	if ok {
		for k, v := range attrs {
			vStr, ok := v.(string)
			if ok {
				logStr += " " + k + "=" + vStr
			}
		}
	}
	_, _ = t.buf.WriteString(logStr)
}

type captureLogger struct {
	e logging.Entry
}

func (c *captureLogger) Log(e logging.Entry) {
	c.e = e
}
