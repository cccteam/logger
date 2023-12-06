package logger

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"
)

func TestLogger(t *testing.T) {
	t.Parallel()

	type args struct {
		v  []any
		v2 any
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
				v:  []any{"Message"},
				v2: "Message",
			},
			wantDebug:  "Debug: Message, testCtxValue",
			wantDebugf: "Debugf: Formatted Message, testCtxValue",
			wantInfo:   "Info: Message, testCtxValue",
			wantInfof:  "Infof: Formatted Message, testCtxValue",
			wantWarn:   "Warn: Message, testCtxValue",
			wantWarnf:  "Warnf: Formatted Message, testCtxValue",
			wantError:  "Error: Message, testCtxValue",
			wantErrorf: "Errorf: Formatted Message, testCtxValue",
		},
		{
			name: "String & Error",
			args: args{
				v:  []any{"Message"},
				v2: errors.New("Message"),
			},
			wantDebug:  "Debug: Message, testCtxValue",
			wantDebugf: "Debugf: Formatted Message, testCtxValue",
			wantInfo:   "Info: Message, testCtxValue",
			wantInfof:  "Infof: Formatted Message, testCtxValue",
			wantWarn:   "Warn: Message, testCtxValue",
			wantWarnf:  "Warnf: Formatted Message, testCtxValue",
			wantError:  "Error: Message, testCtxValue",
			wantErrorf: "Errorf: Formatted Message, testCtxValue",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			l := &testCtxLogger{buf: &buf}
			ctx := newContext(context.WithValue(context.Background(), l, " testCtxValue"), l)

			r := &http.Request{}
			r = r.WithContext(ctx)

			for _, l := range []*Logger{Ctx(ctx), Req(r)} {
				format := "Formatted %s"

				l.Debug(tt.args.v2)
				if s := buf.String(); s != tt.wantDebug {
					t.Errorf("Logger.Debug() = %q, wantValue %q", s, tt.wantDebug)
				}
				buf.Reset()

				l.Debugf(format, tt.args.v...)
				if s := buf.String(); s != tt.wantDebugf {
					t.Errorf("Logger.Debugf() = %q, wantValue %q", s, tt.wantDebugf)
				}
				buf.Reset()

				l.Info(tt.args.v2)
				if s := buf.String(); s != tt.wantInfo {
					t.Errorf("Logger.Info() = %q, wantValue %q", s, tt.wantInfo)
				}
				buf.Reset()

				l.Infof(format, tt.args.v...)
				if s := buf.String(); s != tt.wantInfof {
					t.Errorf("Logger.Infof() = %q, wantValue %q", s, tt.wantInfof)
				}
				buf.Reset()

				l.Warn(tt.args.v2)
				if s := buf.String(); s != tt.wantWarn {
					t.Errorf("Logger.Warn() = %q, wantValue %q", s, tt.wantWarn)
				}
				buf.Reset()

				l.Warnf(format, tt.args.v...)
				if s := buf.String(); s != tt.wantWarnf {
					t.Errorf("Logger.Warnf() = %q, wantValue %q", s, tt.wantWarnf)
				}
				buf.Reset()

				l.Error(tt.args.v2)
				if s := buf.String(); s != tt.wantError {
					t.Errorf("Logger.Error() = %q, wantValue %q", s, tt.wantError)
				}
				buf.Reset()

				l.Errorf(format, tt.args.v...)
				if s := buf.String(); s != tt.wantErrorf {
					t.Errorf("Logger.Errorf() = %q, wantValue %q", s, tt.wantErrorf)
				}
				buf.Reset()
			}
		})
	}
}

func TestLogger_AddRequestAttribute(t *testing.T) {
	t.Parallel()
	type args struct {
		key   string
		value any
	}
	tests := []struct {
		name string
		args args
		want *Logger
	}{
		{
			name: "success add request attribute",
			args: args{
				key:   "new_req_key",
				value: "new_req_value",
			},
			want: &Logger{
				lg: &testCtxLogger{
					reqAttributes: map[string]any{
						"existing_req_key_1": "existing_req_value_1",
						"existing_req_key_2": "existing_req_value_2",
						"new_req_key":        "new_req_value",
					},
					attributes: map[string]any{
						"existing_key_1": "existing_value_1",
						"existing_key_2": "existing_value_2",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			l := &Logger{
				lg: &testCtxLogger{
					reqAttributes: map[string]any{
						"existing_req_key_1": "existing_req_value_1",
						"existing_req_key_2": "existing_req_value_2",
					},
					attributes: map[string]any{
						"existing_key_1": "existing_value_1",
						"existing_key_2": "existing_value_2",
					},
				},
			}
			if got := l.AddRequestAttribute(tt.args.key, tt.args.value); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Logger.AddRequestAttribute() = %v, want %v", got, tt.want)
			}
		})
	}
}

var _ ctxLogger = &testCtxLogger{}

type testCtxLogger struct {
	buf           *bytes.Buffer
	reqAttributes map[string]any
	attributes    map[string]any
}

func (l *testCtxLogger) Debug(ctx context.Context, v any) {
	l.buf.WriteString("Debug: " + fmt.Sprint(v) + "," + fmt.Sprint(ctx.Value(l)))
}

func (l *testCtxLogger) Debugf(ctx context.Context, format string, v ...any) {
	l.buf.WriteString("Debugf: " + fmt.Sprintf(format, v...) + "," + fmt.Sprint(ctx.Value(l)))
}

func (l *testCtxLogger) Info(ctx context.Context, v any) {
	l.buf.WriteString("Info: " + fmt.Sprint(v) + "," + fmt.Sprint(ctx.Value(l)))
}

func (l *testCtxLogger) Infof(ctx context.Context, format string, v ...any) {
	l.buf.WriteString("Infof: " + fmt.Sprintf(format, v...) + "," + fmt.Sprint(ctx.Value(l)))
}

func (l *testCtxLogger) Warn(ctx context.Context, v any) {
	l.buf.WriteString("Warn: " + fmt.Sprint(v) + "," + fmt.Sprint(ctx.Value(l)))
}

func (l *testCtxLogger) Warnf(ctx context.Context, format string, v ...any) {
	l.buf.WriteString("Warnf: " + fmt.Sprintf(format, v...) + "," + fmt.Sprint(ctx.Value(l)))
}

func (l *testCtxLogger) Error(ctx context.Context, v any) {
	l.buf.WriteString("Error: " + fmt.Sprint(v) + "," + fmt.Sprint(ctx.Value(l)))
}

func (l *testCtxLogger) Errorf(ctx context.Context, format string, v ...any) {
	l.buf.WriteString("Errorf: " + fmt.Sprintf(format, v...) + "," + fmt.Sprint(ctx.Value(l)))
}

func (l *testCtxLogger) AddRequestAttribute(key string, value any) {
	l.reqAttributes[key] = value
}

func (l *testCtxLogger) WithAttributes() attributer {
	attributes := make(map[string]any)
	for k, v := range l.reqAttributes {
		attributes[k] = v
	}
	return &testAttributer{logger: l, attributes: attributes}
}

var _ attributer = &testAttributer{}

type testAttributer struct {
	logger     *testCtxLogger
	attributes map[string]any
}

func (a *testAttributer) AddAttribute(key string, value any) {
	a.attributes[key] = value
}

func (a *testAttributer) Logger() ctxLogger {
	attributes := make(map[string]any)
	for k, v := range a.attributes {
		attributes[k] = v
	}

	return &testCtxLogger{
		buf:           a.logger.buf,
		reqAttributes: nil,
		attributes:    attributes,
	}
}
