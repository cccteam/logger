package logger

import (
	"bytes"
	"context"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
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

						w.WriteHeader(tt.args.status)
						handlerCalled = true

						Req(r).AddRequestAttribute("test_key_1", "test_value_1")
						Req(r).AddRequestAttribute("test_key_2", "test_value_2")

						var ok bool
						l, ok = Req(r).lg.(*consoleLogger)
						if !ok {
							t.Fatalf("Failed to get consoleLogger from request")
						}
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
		v       []any
		v2      any
		noColor bool
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
			name: "Test with color", args: args{v: []any{"Message"}, v2: "Message"},
			wantDebug: "\x1b[37mDEBUG\x1b[0m: Message\n", wantDebugf: "\x1b[37mDEBUG\x1b[0m: Formatted Message\n",
			wantInfo: "\x1b[34mINFO \x1b[0m: Message\n", wantInfof: "\x1b[34mINFO \x1b[0m: Formatted Message\n",
			wantWarn: "\x1b[33mWARN \x1b[0m: Message\n", wantWarnf: "\x1b[33mWARN \x1b[0m: Formatted Message\n",
			wantError: "\x1b[31mERROR\x1b[0m: Message\n", wantErrorf: "\x1b[31mERROR\x1b[0m: Formatted Message\n",
		},
		{
			name: "Test no color", args: args{v: []any{"Message"}, v2: "Message", noColor: true},
			wantDebug: "DEBUG: Message\n", wantDebugf: "DEBUG: Formatted Message\n",
			wantInfo: "INFO : Message\n", wantInfof: "INFO : Formatted Message\n",
			wantWarn: "WARN : Message\n", wantWarnf: "WARN : Formatted Message\n",
			wantError: "ERROR: Message\n", wantErrorf: "ERROR: Formatted Message\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			ctx := context.Background()
			log.SetOutput(&buf)
			t.Cleanup(func() { log.SetOutput(os.Stderr) })

			u, _ := url.Parse("http://some.domain.com/path")
			l := &consoleLogger{r: &http.Request{Method: http.MethodGet, URL: u}, noColor: tt.args.noColor}
			l.root = l
			format := "Formatted %s"

			l.Debug(ctx, tt.args.v2)
			if s := buf.String(); s[20:] != tt.wantDebug {
				t.Errorf("stdErrLogger.Debug() value = %v, wantValue %v", s[20:], tt.wantDebug)
			}
			buf.Reset()

			l.Debugf(ctx, format, tt.args.v...)
			if s := buf.String(); s[20:] != tt.wantDebugf {
				t.Errorf("stdErrLogger.Debug() value = %v, wantValue %v", s[20:], tt.wantDebugf)
			}
			buf.Reset()

			l.Info(ctx, tt.args.v2)
			if s := buf.String(); s[20:] != tt.wantInfo {
				t.Errorf("stdErrLogger.Info() value = %v, wantValue %v", s[20:], tt.wantInfo)
			}
			buf.Reset()

			l.Infof(ctx, format, tt.args.v...)
			if s := buf.String(); s[20:] != tt.wantInfof {
				t.Errorf("stdErrLogger.Info() value = %v, wantValue %v", s[20:], tt.wantInfof)
			}
			buf.Reset()

			l.Warn(ctx, tt.args.v2)
			if s := buf.String(); s[20:] != tt.wantWarn {
				t.Errorf("stdErrLogger.Warn() value = %v, wantValue %v", s[20:], tt.wantWarn)
			}
			buf.Reset()

			l.Warnf(ctx, format, tt.args.v...)
			if s := buf.String(); s[20:] != tt.wantWarnf {
				t.Errorf("stdErrLogger.Warn() value = %v, wantValue %v", s[20:], tt.wantWarnf)
			}
			buf.Reset()

			l.Error(ctx, tt.args.v2)
			if s := buf.String(); s[20:] != tt.wantError {
				t.Errorf("stdErrLogger.Error() value = %v, wantValue %v", s[20:], tt.wantError)
			}
			buf.Reset()

			l.Errorf(ctx, format, tt.args.v...)
			if s := buf.String(); s[20:] != tt.wantErrorf {
				t.Errorf("stdErrLogger.Error() value = %v, wantValue %v", s[20:], tt.wantErrorf)
			}
			buf.Reset()
		})
	}
}

func Test_consoleLogger_newChild(t *testing.T) {
	t.Parallel()
	type fields struct {
		root          *consoleLogger
		r             *http.Request
		noColor       bool
		rsvdReqKeys   []string
		attributes    map[string]any
		maxSeverity   logging.Severity
		logCount      int
		reqAttributes map[string]any
	}
	tests := []struct {
		name string
		fields
		want *consoleLogger
	}{
		{
			name: "success",
			fields: fields{
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
			want: &consoleLogger{
				root: &consoleLogger{
					logCount: 123,
				},
				noColor:       true,
				rsvdReqKeys:   []string{"test reserved request key 1", "test reserved request key 2"},
				attributes:    map[string]any{},
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
			l := &consoleLogger{
				root:          tt.fields.root,
				r:             tt.fields.r,
				noColor:       tt.fields.noColor,
				rsvdReqKeys:   tt.fields.rsvdReqKeys,
				attributes:    tt.fields.attributes,
				maxSeverity:   tt.fields.maxSeverity,
				logCount:      tt.fields.logCount,
				reqAttributes: tt.fields.reqAttributes,
			}

			got := l.newChild()
			if diff := cmp.Diff(got, tt.want, cmp.AllowUnexported(consoleLogger{}), cmpopts.IgnoreFields(consoleLogger{}, "mu", "r")); diff != "" {
				t.Errorf("consoleLogger.newChild() mismatch (-want +got):\n%s", diff)
			}
			if got.r != l.r {
				t.Error("consoleLogger.newChild().r != consoleLogger.r")
			}
			if &got.mu == &l.mu {
				t.Errorf("&consoleLogger.newChild().mu = &consoleLogger.mu")
			}
		})
	}
}
