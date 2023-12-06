package logger

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"cloud.google.com/go/logging"
	"github.com/go-test/deep"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestNewConsoleExporter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want *ConsoleExporter
	}{
		{
			name: "Simple Constructor",
			want: &ConsoleExporter{},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := NewConsoleExporter(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewConsoleExporter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConsoleExporter_NoColor(t *testing.T) {
	t.Parallel()

	type fields struct {
		noColor bool
	}
	type args struct {
		v bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *ConsoleExporter
	}{
		{
			name: "noColor=true",
			fields: fields{
				noColor: false,
			},
			args: args{
				v: true,
			},
			want: &ConsoleExporter{
				noColor: true,
			},
		},
		{
			name: "noColor=false",
			fields: fields{
				noColor: true,
			},
			args: args{
				v: false,
			},
			want: &ConsoleExporter{
				noColor: false,
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := &ConsoleExporter{
				noColor: tt.fields.noColor,
			}
			if got := e.NoColor(tt.args.v); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConsoleExporter.NoColor() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConsoleExporter_Middleware(t *testing.T) {
	t.Parallel()

	type fields struct {
		noColor bool
	}
	tests := []struct {
		name   string
		fields fields
		want   func(http.Handler) http.Handler
	}{
		{
			name: "call Middleware",
			fields: fields{
				noColor: true,
			},
			want: func(next http.Handler) http.Handler {
				return &consoleHandler{
					next:    next,
					noColor: true,
				}
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
			e := &ConsoleExporter{
				noColor: tt.fields.noColor,
			}
			got := e.Middleware()(next)
			if diff := deep.Equal(got, tt.want(next)); diff != nil {
				t.Errorf("ConsoleExporter.Middleware() = %v", diff)
			}
		})
	}
}

func Test_consoleHandler_ServeHTTP(t *testing.T) {
	t.Parallel()

	type args struct {
		status int
		level  slog.Level
	}
	tests := []struct {
		name            string
		args            args
		wantMaxSeverity logging.Severity
	}{
		{
			name: "info logging",
			args: args{
				status: http.StatusOK,
				level:  slog.LevelInfo,
			},
			wantMaxSeverity: logging.Info,
		},
		{
			name: "warning logging",
			args: args{
				status: http.StatusOK,
				level:  slog.LevelWarn,
			},
			wantMaxSeverity: logging.Warning,
		},
		{
			name: "error logging",
			args: args{
				status: http.StatusOK,
				level:  slog.LevelError,
			},
			wantMaxSeverity: logging.Error,
		},
		{
			name: "logging for error status",
			args: args{
				status: http.StatusInternalServerError,
			},
			wantMaxSeverity: logging.Error,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var handlerCalled bool
			var l *consoleLogger
			handler := &consoleHandler{
				next: http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						switch tt.args.level {
						case slog.LevelInfo:
							Req(r).Info("some log")
						case slog.LevelWarn:
							Req(r).Warn("some log")
						case slog.LevelError:
							Req(r).Error("some log")
						default:
						}

						var ok bool
						l, ok = Req(r).lg.(*consoleLogger)
						if !ok {
							t.Fatal("Failed to get consoleLogger from request")
						}
						l.reqAttributes["test_key_1"] = "test_value_1"
						l.reqAttributes["test_key_2"] = "test_value_2"

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
			if l.maxSeverity != tt.wantMaxSeverity {
				t.Errorf("Level = %v, want %v", l.maxSeverity, tt.wantMaxSeverity)
			}
			wantAttrs := map[string]any{"test_key_1": "test_value_1", "test_key_2": "test_value_2"}
			if cmp.Diff(l.reqAttributes, wantAttrs) != "" {
				t.Errorf("Attributes mismatch (-want +got):\n%s", cmp.Diff(l.reqAttributes, wantAttrs))
			}
		})
	}
}

func TestNewConsoleLogger(t *testing.T) {
	t.Parallel()

	type args struct {
		r       *http.Request
		noColor bool
	}
	tests := []struct {
		name string
		args args
		want ctxLogger
	}{
		{
			name: "some request",
			args: args{
				r:       &http.Request{},
				noColor: true,
			},
			want: &consoleLogger{
				r:             &http.Request{},
				noColor:       true,
				maxSeverity:   logging.Debug,
				rsvdReqKeys:   []string{"requestSize", "responseSize", "logCount"},
				reqAttributes: map[string]any{},
				attributes:    map[string]any{},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := newConsoleLogger(tt.args.r, tt.args.noColor)
			if diff := cmp.Diff(got, tt.want, cmp.AllowUnexported(consoleLogger{}), cmpopts.IgnoreFields(consoleLogger{}, "r", "mu", "root")); diff != "" {
				t.Errorf("NewConsoleLogger() mismatch (-want +got):\n%s", diff)
			}
			if got.root != got {
				t.Errorf("NewConsoleLogger().root is not self")
			}
		})
	}
}

func Test_consoleLogger(t *testing.T) {
	type args struct {
		v  []any
		v2 any
	}
	type fields struct {
		noColor    bool
		attributes map[string]any
	}
	tests := []struct {
		name       string
		args       args
		fields     fields
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
			name: "Test with color", args: args{v: []any{"Message"}, v2: "Message"},
			fields:    fields{attributes: map[string]any{"a test key": "a test value"}},
			wantDebug: "\x1b[37mDEBUG\x1b[0m: Message", wantDebugf: "\x1b[37mDEBUG\x1b[0m: Formatted Message",
			wantInfo: "\x1b[34mINFO \x1b[0m: Message", wantInfof: "\x1b[34mINFO \x1b[0m: Formatted Message",
			wantWarn: "\x1b[33mWARN \x1b[0m: Message", wantWarnf: "\x1b[33mWARN \x1b[0m: Formatted Message",
			wantError: "\x1b[31mERROR\x1b[0m: Message", wantErrorf: "\x1b[31mERROR\x1b[0m: Formatted Message",
		},
		{
			name: "Test no color", args: args{v: []any{"Message"}, v2: "Message"},
			fields:    fields{noColor: true, attributes: map[string]any{"test_key_1": "test_value_1", "test_key_2": "test_value_2"}},
			wantDebug: "DEBUG: Message", wantDebugf: "DEBUG: Formatted Message",
			wantInfo: "INFO : Message", wantInfof: "INFO : Formatted Message",
			wantWarn: "WARN : Message", wantWarnf: "WARN : Formatted Message",
			wantError: "ERROR: Message", wantErrorf: "ERROR: Formatted Message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			ctx := context.Background()
			log.SetOutput(&buf)
			t.Cleanup(func() { log.SetOutput(os.Stderr) })

			u, _ := url.Parse("http://some.domain.com/path")
			l := &consoleLogger{r: &http.Request{Method: http.MethodGet, URL: u}, noColor: tt.fields.noColor, attributes: tt.fields.attributes}
			l.root = l
			format := "Formatted %s"

			verifyLog := func(log, methodName, expectedPrefix string) {
				if !strings.HasPrefix(log, expectedPrefix) {
					t.Errorf("consoleLogger.%s() = %q, missing prefix %q", methodName, log, expectedPrefix)
				}

				for k, v := range tt.fields.attributes {
					attrStr := fmt.Sprintf("%s=%v", k, v)
					if !strings.Contains(log, attrStr) {
						t.Errorf("consoleLogger.%s() missing attribute %s", methodName, attrStr)
					}
				}

				if !strings.HasSuffix(log, "\n") {
					t.Errorf("consoleLogger.%s() = %q, missing suffix \\n", methodName, log)
				}
			}

			l.Debug(ctx, tt.args.v2)
			verifyLog(buf.String()[20:], "Debug", tt.wantDebug)
			buf.Reset()

			l.Debugf(ctx, format, tt.args.v...)
			verifyLog(buf.String()[20:], "Debugf", tt.wantDebugf)
			buf.Reset()

			l.Info(ctx, tt.args.v2)
			verifyLog(buf.String()[20:], "Info", tt.wantInfo)
			buf.Reset()

			l.Infof(ctx, format, tt.args.v...)
			verifyLog(buf.String()[20:], "Infof", tt.wantInfof)
			buf.Reset()

			l.Warn(ctx, tt.args.v2)
			verifyLog(buf.String()[20:], "Warn", tt.wantWarn)
			buf.Reset()

			l.Warnf(ctx, format, tt.args.v...)
			verifyLog(buf.String()[20:], "Warnf", tt.wantWarnf)
			buf.Reset()

			l.Error(ctx, tt.args.v2)
			verifyLog(buf.String()[20:], "Error", tt.wantError)
			buf.Reset()

			l.Errorf(ctx, format, tt.args.v...)
			verifyLog(buf.String()[20:], "Errorf", tt.wantErrorf)
			buf.Reset()
		})
	}
}

func Test_consoleLogger_AddRequestAttribute(t *testing.T) {
	t.Parallel()
	type fields struct {
		root        *consoleLogger
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
				root: &consoleLogger{
					reqAttributes: map[string]any{"test_key_2": "test_value_2"},
				},
				rsvdReqKeys: []string{"test_key 1", "test_key"},
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
				root: &consoleLogger{
					reqAttributes: map[string]any{"test_key_2": "test_value_2"},
				},
				rsvdReqKeys: []string{"test_key 1"},
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
				root: &consoleLogger{
					reqAttributes: map[string]any{"test_key_2": "test_value_2"},
				},
				rsvdReqKeys: []string{"test_key 1"},
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
			l := &consoleLogger{
				root:        tt.fields.root,
				rsvdReqKeys: tt.fields.rsvdReqKeys,
			}
			l.AddRequestAttribute(tt.args.key, tt.args.value)
			if diff := cmp.Diff(l.root.reqAttributes, tt.want); diff != "" {
				t.Errorf("consoleLogger.AddRequestAttribute() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_consoleLogger_WithAttributes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		attributes map[string]any
		want       *consoleAttributer
	}{
		{
			name: "with attributes success",
			attributes: map[string]any{
				"test_key_1": "test_value_1",
				"test_key_2": "test_value_2",
			},
			want: &consoleAttributer{
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
			l := &consoleLogger{
				attributes: tt.attributes,
			}
			got := l.WithAttributes()
			if diff := cmp.Diff(got, tt.want, cmp.AllowUnexported(consoleAttributer{}), cmpopts.IgnoreFields(consoleAttributer{}, "logger")); diff != "" {
				t.Errorf("consoleLogger.WithAttributes() mismatch (-want +got):\n%s", diff)
			}
			if a, ok := got.(*consoleAttributer); !ok {
				t.Errorf("consoleLogger.WithAttributes() type %T, want %T", got, &consoleAttributer{})
			} else if a.logger != l {
				t.Errorf("consoleLogger.WithAttributes().logger != consoleLogger")
			}
		})
	}
}

func Test_consoleAttributer_AddAttribute(t *testing.T) {
	t.Parallel()
	type args struct {
		key   string
		value any
	}
	tests := []struct {
		name       string
		args       args
		attributes map[string]any
		want       map[string]any
	}{
		{
			name: "add attribute",
			args: args{
				key:   "test_key_0",
				value: 0,
			},
			attributes: map[string]any{
				"test_key_1": 1,
				"test_key_2": "test_value_2",
			},
			want: map[string]any{
				"test_key_1": 1,
				"test_key_2": "test_value_2",
				"test_key_0": 0,
			},
		},
		{
			name: "overwrite attribute value",
			args: args{
				key:   "test_key_1",
				value: "512",
			},
			attributes: map[string]any{
				"test_key_1": 1,
				"test_key_2": "test_value_2",
			},
			want: map[string]any{
				"test_key_1": "512",
				"test_key_2": "test_value_2",
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := &consoleAttributer{
				attributes: tt.attributes,
			}
			a.AddAttribute(tt.args.key, tt.args.value)
			if diff := cmp.Diff(a.attributes, tt.want); diff != "" {
				t.Errorf("consoleAttributer.AddAttribute() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_consoleAttributer_Logger(t *testing.T) {
	t.Parallel()
	type fields struct {
		logger     *consoleLogger
		attributes map[string]any
	}
	tests := []struct {
		name string
		fields
		want *consoleLogger
	}{
		{
			name: "success",
			fields: fields{
				logger: &consoleLogger{
					root: &consoleLogger{
						logCount: 123,
					},
					r:             httptest.NewRequest(http.MethodGet, "/test/url", http.NoBody),
					noColor:       true,
					rsvdReqKeys:   []string{"test reserved request key 1", "test reserved request key 2"},
					attributes:    map[string]any{"test_key_1": "test_value_1", "test_key_2": "test_value_2"},
					maxSeverity:   logging.Warning,
					logCount:      456,
					reqAttributes: map[string]any{"test_req_key_1": "test_req_value_1", "test_req_key_2": "test_req_value_2"},
				},
				attributes: map[string]any{"test_key_3": "test_value_3", "test_key_4": "test_value_4"},
			},
			want: &consoleLogger{
				root: &consoleLogger{
					logCount: 123,
				},
				noColor:       true,
				rsvdReqKeys:   []string{"test reserved request key 1", "test reserved request key 2"},
				attributes:    map[string]any{"test_key_3": "test_value_3", "test_key_4": "test_value_4"},
				maxSeverity:   logging.Debug,
				logCount:      0,
				reqAttributes: nil,
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := &consoleAttributer{
				logger:     tt.fields.logger,
				attributes: tt.fields.attributes,
			}

			got := a.Logger()
			if diff := cmp.Diff(got, tt.want, cmp.AllowUnexported(consoleLogger{}), cmpopts.IgnoreFields(consoleLogger{}, "mu", "r")); diff != "" {
				t.Errorf("consoleAttributer.Logger() mismatch (-want +got):\n%s", diff)
			}
			gotConsoleLogger, ok := got.(*consoleLogger)
			if !ok {
				t.Errorf("consoleAttributer.Logger() type %T, want %T", got, &consoleLogger{})
				return
			}
			if gotConsoleLogger.r != a.logger.r {
				t.Error("consoleAttributer.Logger().r is NOT the original request")
			}
		})
	}
}
