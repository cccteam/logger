package logger

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"cloud.google.com/go/logging"
	"github.com/go-test/deep"
)

func TestNewRequestLogger(t *testing.T) {
	disableMetaServertest(t)

	type args struct {
		e Exporter
	}
	tests := []struct {
		name string
		args args
		want func(http.Handler) http.Handler
	}{
		{
			name: "Google Exporter",
			args: args{
				e: NewGoogleCloudExporter(&logging.Client{}, "My first project"),
			},
			want: func(next http.Handler) http.Handler {
				client := &logging.Client{}

				return &gcpHandler{
					next:         next,
					parentLogger: client.Logger("request_parent_log"),
					childLogger:  client.Logger("request_child_log"),
					projectID:    "My first project",
					logAll:       true,
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
			got := NewRequestLogger(tt.args.e)
			if diff := deep.Equal(got(next), tt.want(next)); diff != nil {
				t.Errorf("NewRequestLogger() = %v", diff)
			}
		})
	}
}

func TestNewCliLogger(t *testing.T) {
	disableMetaServertest(t)

	type args struct {
		e Exporter
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Console Exporter",
			args: args{
				e: NewConsoleExporter(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewCliLogger(tt.args.e)
			if got == nil {
				t.Errorf("NewCliLogger() returned nil")
			}
		})
	}
}

func Test_requestSize(t *testing.T) {
	t.Parallel()

	type args struct {
		length string
	}
	tests := []struct {
		name string
		args args
		want int64
	}{
		{
			name: "success",
			args: args{
				length: "20",
			},
			want: 20,
		},
		{
			name: "falure",
			args: args{
				length: "xxx",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := requestSize(tt.args.length); got != tt.want {
				t.Errorf("requestSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_recorder_Status(t *testing.T) {
	t.Parallel()

	type fields struct {
		status int
	}
	tests := []struct {
		name   string
		fields fields
		want   int
	}{
		{
			name: "Status set",
			fields: fields{
				status: http.StatusForbidden,
			},
			want: 403,
		},
		{
			name: "Status not set",
			want: 200,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := &recorder{
				status: tt.fields.status,
			}
			if got := w.Status(); got != tt.want {
				t.Errorf("recorder.Status() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_recorder_Length(t *testing.T) {
	t.Parallel()

	type fields struct {
		ResponseWriter http.ResponseWriter
	}
	type args struct {
		b []byte
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantLength int64
	}{
		{
			name: "Write 10 bytes",
			fields: fields{
				ResponseWriter: &httptest.ResponseRecorder{},
			},
			args: args{
				b: []byte("0123456789"),
			},
			wantLength: 10,
		},
		{
			name: "Write 0 bytes",
			fields: fields{
				ResponseWriter: &httptest.ResponseRecorder{},
			},
			args: args{
				b: []byte(""),
			},
			wantLength: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := &recorder{
				ResponseWriter: tt.fields.ResponseWriter,
			}
			got, err := w.Write(tt.args.b)
			if err != nil {
				t.Fatalf("recorder.Write() error = %v, wantErr %v", err, false)
			}
			if int64(got) != tt.wantLength {
				t.Errorf("recorder.Write() = %v, wantLength %v", got, tt.wantLength)
			}
			if got := w.Length(); got != tt.wantLength {
				t.Errorf("recorder.Status() = %v, wantLength %v", got, tt.wantLength)
			}
		})
	}
}

func Test_recorder_WriteHeader(t *testing.T) {
	t.Parallel()

	type fields struct {
		ResponseWriter http.ResponseWriter
	}
	type args struct {
		status int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int
	}{
		{
			name: "Success",
			fields: fields{
				ResponseWriter: &httptest.ResponseRecorder{},
			},
			args: args{
				status: 201,
			},
			want: 201,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := &recorder{
				ResponseWriter: tt.fields.ResponseWriter,
			}
			w.WriteHeader(tt.args.status)
			if got := w.Status(); got != tt.want {
				t.Errorf("recorder.Status() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_recorder_Write(t *testing.T) {
	t.Parallel()

	type fields struct {
		ResponseWriter http.ResponseWriter
		status         int
	}
	type args struct {
		b []byte
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantLength int
		wantStatus int
		wantErr    bool
	}{
		{
			name: "No status set",
			fields: fields{
				ResponseWriter: &httptest.ResponseRecorder{},
			},
			args: args{
				b: []byte("0123456789"),
			},
			wantLength: 10,
			wantStatus: 200,
		},
		{
			name: "Status set",
			fields: fields{
				ResponseWriter: &httptest.ResponseRecorder{},
				status:         201,
			},
			args: args{
				b: []byte("01234567891234567890"),
			},
			wantLength: 20,
			wantStatus: 201,
		},
		{
			name: "Write error",
			fields: fields{
				ResponseWriter: &testResponseRecorder{err: errors.New("Bang")},
				status:         201,
			},
			args: args{
				b: []byte("01234567891234567890"),
			},
			wantLength: 20,
			wantStatus: 201,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := &recorder{
				ResponseWriter: tt.fields.ResponseWriter,
				status:         tt.fields.status,
			}
			got, err := w.Write(tt.args.b)
			if (err != nil) != tt.wantErr {
				t.Fatalf("recorder.Write() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.wantLength {
				t.Errorf("recorder.Write() = %v, wantLength %v", got, tt.wantLength)
			}
			if got := w.Status(); got != tt.wantStatus {
				t.Errorf("recorder.Status() = %v, wantStatus %v", got, tt.wantStatus)
			}
		})
	}
}

func Test_generateID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantLen int
	}{
		{
			name:    "Length 16",
			wantLen: 16,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := generateID(); len(got)/2 != tt.wantLen {
				t.Errorf("generateID() = %v, want len=%v", got, tt.wantLen)
			}
		})
	}
}

type testResponseRecorder struct {
	http.ResponseWriter
	err error
}

func (rw *testResponseRecorder) Write(buf []byte) (int, error) {
	return len(buf), rw.err
}

func Test_recorderFlusher_Flush(t *testing.T) {
	t.Parallel()

	type fields struct {
		recorder http.ResponseWriter
	}
	tests := []struct {
		name        string
		fields      fields
		wantFlusher bool
		flushCount  int
	}{
		{
			name: "Flusher",
			fields: fields{
				recorder: newResponseRecorder(&testResponseWriterFlusher{}),
			},
			wantFlusher: true,
			flushCount:  1,
		},
		{
			name: "No flusher",
			fields: fields{
				recorder: newResponseRecorder(&testResponseWriter{}),
			},
			wantFlusher: false,
			flushCount:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := tt.fields.recorder
			f, gotFlusher := r.(http.Flusher)
			if gotFlusher {
				f.Flush()
			}
			if gotFlusher != tt.wantFlusher {
				t.Fatalf("recorder foundFlusher = %v, want %v", gotFlusher, tt.wantFlusher)
			}

			if tt.wantFlusher {
				rf, ok := r.(*recorderFlusher)
				if !ok {
					t.Fatalf("recorder not a recorderFlusher")
				}
				c, ok := rf.ResponseWriter.(*testResponseWriterFlusher)
				if !ok {
					t.Fatalf("ResponseWriter not a testResponseWriterFlusher")
				}
				if c.flushed != tt.flushCount {
					t.Errorf("recorderFlusher.Flush() = %v, want %v", c.flushed, tt.flushCount)
				}
			}
		})
	}
}

func Test_recorderHijacker_Hijack(t *testing.T) {
	t.Parallel()

	type fields struct {
		recorder http.ResponseWriter
	}
	tests := []struct {
		name         string
		fields       fields
		wantHijacker bool
		hijackCount  int
	}{
		{
			name: "Hijacker",
			fields: fields{
				recorder: newResponseRecorder(&testResponseWriterHijacker{}),
			},
			wantHijacker: true,
			hijackCount:  1,
		},
		{
			name: "No hijacker",
			fields: fields{
				recorder: newResponseRecorder(&testResponseWriter{}),
			},
			wantHijacker: false,
			hijackCount:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := tt.fields.recorder
			h, gotHijacker := r.(http.Hijacker)
			if gotHijacker {
				_, _, _ = h.Hijack()
			}
			if gotHijacker != tt.wantHijacker {
				t.Fatalf("recorder foundHijacker = %v, want %v", gotHijacker, tt.wantHijacker)
			}

			if tt.wantHijacker {
				rh, ok := r.(*recorderHijacker)
				if !ok {
					t.Fatalf("recorder not a recorderHijacker")
				}
				c, ok := rh.ResponseWriter.(*testResponseWriterHijacker)
				if !ok {
					t.Fatalf("ResponseWriter not a testResponseWriterHijacker")
				}
				if c.hijacked != tt.hijackCount {
					t.Errorf("recorderHijacker.Hijack() = %v, want %v", c.hijacked, tt.hijackCount)
				}
			}
		})
	}
}

func Test_recorderFlusherHijacker_FlushHijack(t *testing.T) {
	t.Parallel()

	r := newResponseRecorder(&testResponseWriterFlusherHijacker{})

	f, gotFlusher := r.(http.Flusher)
	if !gotFlusher {
		t.Fatalf("expected http.Flusher")
	}
	f.Flush()

	h, gotHijacker := r.(http.Hijacker)
	if !gotHijacker {
		t.Fatalf("expected http.Hijacker")
	}
	_, _, _ = h.Hijack()

	rfh, ok := r.(*recorderFlusherHijacker)
	if !ok {
		t.Fatalf("recorder not a recorderFlusherHijacker")
	}
	c, ok := rfh.ResponseWriter.(*testResponseWriterFlusherHijacker)
	if !ok {
		t.Fatalf("ResponseWriter not a testResponseWriterFlusherHijacker")
	}
	if c.flushed != 1 {
		t.Errorf("expected 1 flush, got %d", c.flushed)
	}
	if c.hijacked != 1 {
		t.Errorf("expected 1 hijack, got %d", c.hijacked)
	}
}

type testResponseWriter struct{}

func (*testResponseWriter) Header() http.Header {
	return http.Header{}
}

func (*testResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (*testResponseWriter) WriteHeader(int) {
}

type testResponseWriterFlusher struct {
	testResponseWriter
	flushed int
}

func (t *testResponseWriterFlusher) Flush() {
	t.flushed++
}

type testResponseWriterHijacker struct {
	testResponseWriter
	hijacked int
}

func (t *testResponseWriterHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	t.hijacked++
	return nil, nil, nil
}

type testResponseWriterFlusherHijacker struct {
	testResponseWriter
	flushed  int
	hijacked int
}

func (t *testResponseWriterFlusherHijacker) Flush() {
	t.flushed++
}

func (t *testResponseWriterFlusherHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	t.hijacked++
	return nil, nil, nil
}
