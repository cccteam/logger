package logger

import (
	"bytes"
	"context"
	"log"
	"os"
	"testing"
)

func Test_stdErrLogger(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

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
			name: "Test 1",
			args: args{
				v:  []any{"Message"},
				v2: "Message",
			},
			wantDebug:  "DEBUG: Message\n",
			wantDebugf: "DEBUG: Formatted Message\n",
			wantInfo:   "INFO : Message\n",
			wantInfof:  "INFO : Formatted Message\n",
			wantWarn:   "WARN : Message\n",
			wantWarnf:  "WARN : Formatted Message\n",
			wantError:  "ERROR: Message\n",
			wantErrorf: "ERROR: Formatted Message\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			l := &stdErrLogger{}
			format := "Formatted %s"

			l.Debug(ctx, tt.args.v2)
			if s := buf.String()[20:]; s != tt.wantDebug {
				t.Errorf("stdErrLogger.Debug() value = %v, wantValue %v", s, tt.wantDebug)
			}
			buf.Reset()

			l.Debugf(ctx, format, tt.args.v...)
			if s := buf.String()[20:]; s != tt.wantDebugf {
				t.Errorf("stdErrLogger.Debug() value = %v, wantValue %v", s, tt.wantDebugf)
			}
			buf.Reset()

			l.Info(ctx, tt.args.v2)
			if s := buf.String()[20:]; s != tt.wantInfo {
				t.Errorf("stdErrLogger.Info() value = %v, wantValue %v", s, tt.wantInfo)
			}
			buf.Reset()

			l.Infof(ctx, format, tt.args.v...)
			if s := buf.String()[20:]; s != tt.wantInfof {
				t.Errorf("stdErrLogger.Info() value = %v, wantValue %v", s, tt.wantInfof)
			}
			buf.Reset()

			l.Warn(ctx, tt.args.v2)
			if s := buf.String()[20:]; s != tt.wantWarn {
				t.Errorf("stdErrLogger.Warn() value = %v, wantValue %v", s, tt.wantWarn)
			}
			buf.Reset()

			l.Warnf(ctx, format, tt.args.v...)
			if s := buf.String()[20:]; s != tt.wantWarnf {
				t.Errorf("stdErrLogger.Warn() value = %v, wantValue %v", s, tt.wantWarnf)
			}
			buf.Reset()

			l.Error(ctx, tt.args.v2)
			if s := buf.String()[20:]; s != tt.wantError {
				t.Errorf("stdErrLogger.Error() value = %v, wantValue %v", s, tt.wantError)
			}
			buf.Reset()

			l.Errorf(ctx, format, tt.args.v...)
			if s := buf.String()[20:]; s != tt.wantErrorf {
				t.Errorf("stdErrLogger.Error() value = %v, wantValue %v", s, tt.wantErrorf)
			}
			buf.Reset()
		})
	}
}
