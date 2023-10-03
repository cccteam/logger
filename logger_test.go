package logger

import (
	"bytes"
	"context"
	"errors"
	"net/http"
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
				v:  []any{"Message"},
				v2: errors.New("Message"),
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

			var buf bytes.Buffer
			ctx := newContext(context.Background(), &gcpLogger{
				lg: &testLogger{
					buf: &buf,
				},
			})

			r := &http.Request{}
			r = r.WithContext(ctx)

			for _, l := range []*Logger{Ctx(ctx), Req(r)} {
				format := "Formatted %s"

				l.Debug(tt.args.v2)
				if s := buf.String(); s != tt.wantDebug {
					t.Errorf("Logger.Debug() value = %v, wantValue %v", s, tt.wantDebug)
				}
				buf.Reset()

				l.Debugf(format, tt.args.v...)
				if s := buf.String(); s != tt.wantDebugf {
					t.Errorf("Logger.Debugf() value = %v, wantValue %v", s, tt.wantDebugf)
				}
				buf.Reset()

				l.Info(tt.args.v2)
				if s := buf.String(); s != tt.wantInfo {
					t.Errorf("Logger.Info() value = %v, wantValue %v", s, tt.wantInfo)
				}
				buf.Reset()

				l.Infof(format, tt.args.v...)
				if s := buf.String(); s != tt.wantInfof {
					t.Errorf("Logger.Infof() value = %v, wantValue %v", s, tt.wantInfof)
				}
				buf.Reset()

				l.Warn(tt.args.v2)
				if s := buf.String(); s != tt.wantWarn {
					t.Errorf("Logger.Warn() value = %v, wantValue %v", s, tt.wantWarn)
				}
				buf.Reset()

				l.Warnf(format, tt.args.v...)
				if s := buf.String(); s != tt.wantWarnf {
					t.Errorf("Logger.Warnf() value = %v, wantValue %v", s, tt.wantWarnf)
				}
				buf.Reset()

				l.Error(tt.args.v2)
				if s := buf.String(); s != tt.wantError {
					t.Errorf("Logger.Error() value = %v, wantValue %v", s, tt.wantError)
				}
				buf.Reset()

				l.Errorf(format, tt.args.v...)
				if s := buf.String(); s != tt.wantErrorf {
					t.Errorf("Logger.Errorf() value = %v, wantValue %v", s, tt.wantErrorf)
				}
				buf.Reset()
			}
		})
	}
}
