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
			name: "logging for error status",
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
						awsLgr, ok := Req(r).lg.(*awsLogger)
						if !ok {
							t.Fatal("Failed to get awsLogger from request")
						}
						awsLgr.reqAttributes["test_req_key_1"] = "test_req_value_1"
						awsLgr.reqAttributes["test_req_key_2"] = "test_req_value_2"

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

			r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			handler.ServeHTTP(httptest.NewRecorder(), r)

			if !handlerCalled {
				t.Errorf("Failed to call handler")
			}
			if !tt.fields.logAll && tt.args.logs == 0 {
				return
			}
			if l.level != tt.wantLevel {
				t.Errorf("Level = %v, want %v", l.level, tt.wantLevel)
			}
			if len(l.attrs) != 13 {
				t.Errorf("Expected %d request attributes, got %d", 13, len(l.attrs))
			}
			if l.msg != "Parent Log Entry" {
				t.Errorf("Message = %v, want %v", l.msg, "Parent Log Entry")
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
				rsvdKeys:      []string{"trace_id", "span_id"},
				rsvdReqKeys:   []string{"trace_id", "span_id", "http.elapsed", "http.method", "http.url", "http.status_code", "http.response.length", "http.user_agent", "http.remote_ip", "http.scheme", "http.proto"},
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
			if diff := cmp.Diff(got, tt.want, cmpopts.IgnoreFields(awsLogger{}, "logger", "mu", "root"), cmp.AllowUnexported(awsLogger{})); diff != "" {
				t.Errorf("newAWSLogger() mismatch (-want +got):\n%s", diff)
			}
			if got.root != got {
				t.Errorf("newAWSLogger().root is not self")
			}
		})
	}
}

func Test_awsLogger(t *testing.T) {
	t.Parallel()

	type fields struct {
		traceID    string
		attributes map[string]any
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
				traceID:    "1234567890",
				attributes: map[string]any{"a test key": "a test value"},
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
				traceID:    "1234567890",
				attributes: map[string]any{"test_key_1": "test_value_1", "test_key_2": "test_value_2"},
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

			l := &awsLogger{
				logger: &testSlogger{
					buf: &buf,
				},
				attributes: tt.fields.attributes,
				traceID:    tt.fields.traceID,
			}
			l.root = l

			verifyLog := func(log, methodName, expectedPrefix string, expectedLvl slog.Level) {
				if !strings.HasPrefix(log, expectedPrefix) {
					t.Errorf("awsLogger.%s() = %q, missing prefix %q", methodName, log, expectedPrefix)
				}

				expectedVals := []string{
					"trace_id=" + tt.fields.traceID,
					"span_id=" + span.SpanContext().SpanID().String(),
					"level=" + expectedLvl.String(),
				}
				for k, v := range tt.fields.attributes {
					expectedVals = append(expectedVals, slog.Any(k, v).String())
				}
				for _, v := range expectedVals {
					if !strings.Contains(log, v) {
						t.Errorf("awsLogger.%s() = %q, missing: %q", methodName, log, v)
					}
				}
			}

			l.Debug(ctx, tt.args.v2)
			verifyLog(buf.String(), "Debug", tt.wantDebug, slog.LevelDebug)
			buf.Reset()

			l.Debugf(ctx, tt.args.format, tt.args.v...)
			verifyLog(buf.String(), "Debugf", tt.wantDebugf, slog.LevelDebug)
			buf.Reset()

			l.Info(ctx, tt.args.v2)
			verifyLog(buf.String(), "Info", tt.wantInfo, slog.LevelInfo)
			buf.Reset()

			l.Infof(ctx, tt.args.format, tt.args.v...)
			verifyLog(buf.String(), "Infof", tt.wantInfof, slog.LevelInfo)
			buf.Reset()

			l.Warn(ctx, tt.args.v2)
			verifyLog(buf.String(), "Warn", tt.wantWarn, slog.LevelWarn)
			buf.Reset()

			l.Warnf(ctx, tt.args.format, tt.args.v...)
			verifyLog(buf.String(), "Warnf", tt.wantWarnf, slog.LevelWarn)
			buf.Reset()

			l.Error(ctx, tt.args.v2)
			verifyLog(buf.String(), "Error", tt.wantError, slog.LevelError)
			buf.Reset()

			l.Errorf(ctx, tt.args.format, tt.args.v...)
			verifyLog(buf.String(), "Errorf", tt.wantErrorf, slog.LevelError)
			buf.Reset()
		})
	}
}

func Test_awsLogger_AddRequestAttribute(t *testing.T) {
	t.Parallel()
	type fields struct {
		root        *awsLogger
		rsvdReqKeys []string
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
			name: "prefix reserved key with 'custom_'",
			fields: fields{
				root: &awsLogger{
					reqAttributes: map[string]any{"test_key": "test_value"},
				},
				rsvdReqKeys: []string{"test_key 1", "test_key"},
			},
			args: args{
				key:   "test_key",
				value: 512,
			},
			want: map[string]any{"test_key": "test_value", "custom_test_key": 512},
		},
		{
			name: "success adding request attribute",
			fields: fields{
				root: &awsLogger{
					reqAttributes: map[string]any{"test_key": "test_value"},
				},
				rsvdReqKeys: []string{"test_key 1"},
			},
			args: args{
				key:   "test_key",
				value: 512,
			},
			want: map[string]any{"test_key": 512},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			l := &awsLogger{
				root:        tt.fields.root,
				rsvdReqKeys: tt.fields.rsvdReqKeys,
			}
			l.AddRequestAttribute(tt.args.key, tt.args.value)
			if diff := cmp.Diff(l.root.reqAttributes, tt.want); diff != "" {
				t.Errorf("awsLogger.AddRequestAttribute() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_awsLogger_WithAttributes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		attributes map[string]any
		want       *awsAttributer
	}{
		{
			name: "with attributes success",
			attributes: map[string]any{
				"test_key_1": "test_value_1",
				"test_key_2": "test_value_2",
			},
			want: &awsAttributer{
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
			l := &awsLogger{
				attributes: tt.attributes,
			}
			got := l.WithAttributes()
			if diff := cmp.Diff(got, tt.want, cmp.AllowUnexported(awsAttributer{}), cmpopts.IgnoreFields(awsAttributer{}, "logger")); diff != "" {
				t.Errorf("awsLogger.WithAttributes() mismatch (-want +got):\n%s", diff)
			}
			if a, ok := got.(*awsAttributer); !ok {
				t.Errorf("awsLogger.WithAttributes() type %T, want %T", got, &awsAttributer{})
			} else if a.logger != l {
				t.Errorf("awsLogger.WithAttributes().logger != awsLogger")
			}
		})
	}
}

func Test_awsAttributer_AddAttribute(t *testing.T) {
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
			name: "success prefixing reserved key",
			args: args{
				key:   "test_key_0",
				value: "test_value_0",
			},
			rsvdKeys: []string{"test_key 0", "test_key_0"},
			attributes: map[string]any{
				"test_key_1": "test_value_1",
				"test_key_2": "test_value_2",
			},
			want: map[string]any{
				"test_key_1":        "test_value_1",
				"test_key_2":        "test_value_2",
				"custom_test_key_0": "test_value_0",
			},
		},
		{
			name: "success adding attribute (non-reserved key)",
			args: args{
				key:   "test_key_0",
				value: "test_value_0",
			},
			rsvdKeys: []string{"test_key 0"},
			attributes: map[string]any{
				"test_key_1": "test_value_1",
				"test_key_2": "test_value_2",
			},
			want: map[string]any{
				"test_key_1": "test_value_1",
				"test_key_2": "test_value_2",
				"test_key_0": "test_value_0",
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := &awsAttributer{
				attributes: tt.attributes,
				logger:     &awsLogger{rsvdKeys: tt.rsvdKeys},
			}
			a.AddAttribute(tt.args.key, tt.args.value)
			if diff := cmp.Diff(a.attributes, tt.want); diff != "" {
				t.Errorf("awsAttributer.AddAttribute() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_awsAttributer_Logger(t *testing.T) {
	t.Parallel()
	type fields struct {
		logger     *awsLogger
		attributes map[string]any
	}
	tests := []struct {
		name string
		fields
		want *awsLogger
	}{
		{
			name: "success getting logger",
			fields: fields{
				logger: &awsLogger{
					root: &awsLogger{
						traceID: "root trace id",
					},
					logger:        &testSlogger{},
					traceID:       "1234567890",
					rsvdKeys:      []string{"test reserved key 1", "test reserved key 2"},
					rsvdReqKeys:   []string{"test reserved request key 1", "test reserved request key 2"},
					attributes:    map[string]any{"test_key_1": "test_value_1", "test_key_2": "test_value_2"},
					maxLevel:      slog.LevelWarn,
					logCount:      2,
					reqAttributes: map[string]any{"test_req_key_1": "test_req_value_1", "test_req_key_2": "test_req_value_2"},
				},
				attributes: map[string]any{"test_key_3": "test_value_3", "test_key_4": "test_value_4"},
			},
			want: &awsLogger{
				root: &awsLogger{
					traceID: "root trace id",
				},
				traceID:       "1234567890",
				rsvdKeys:      []string{"test reserved key 1", "test reserved key 2"},
				rsvdReqKeys:   []string{"test reserved request key 1", "test reserved request key 2"},
				attributes:    map[string]any{"test_key_3": "test_value_3", "test_key_4": "test_value_4"},
				maxLevel:      slog.LevelInfo,
				logCount:      0,
				reqAttributes: nil,
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := &awsAttributer{
				logger:     tt.fields.logger,
				attributes: tt.fields.attributes,
			}

			got := a.Logger()
			if diff := cmp.Diff(got, tt.want, cmp.AllowUnexported(awsLogger{}), cmpopts.IgnoreFields(awsLogger{}, "mu", "logger")); diff != "" {
				t.Errorf("awsAttributer.Logger() mismatch (-want +got):\n%s", diff)
			}
			gotAwsLogger, ok := got.(*awsLogger)
			if !ok {
				t.Errorf("awsAttributer.Logger() type %T, want %T", got, &awsLogger{})
				return
			}
			if gotAwsLogger.logger != a.logger.logger {
				t.Errorf("got awsLogger.logger is NOT the original logger")
			}
		})
	}
}

type testSlogger struct {
	buf *bytes.Buffer
}

func (t *testSlogger) LogAttrs(_ context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	_, _ = fmt.Fprint(t.buf, msg, "level="+level.String(), attrs)
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
