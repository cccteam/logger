package logger

import (
	"errors"
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
			got := NewRequestLogger(tt.args.e)
			if diff := deep.Equal(got(next), tt.want(next)); diff != nil {
				t.Errorf("NewRequestLogger() = %v", diff)
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := requestSize(tt.args.length); got != tt.want {
				t.Errorf("requestSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_statusWriter_Status(t *testing.T) {
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := &statusWriter{
				status: tt.fields.status,
			}
			if got := w.Status(); got != tt.want {
				t.Errorf("statusWriter.Status() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_statusWriter_WriteHeader(t *testing.T) {
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := &statusWriter{
				ResponseWriter: tt.fields.ResponseWriter,
			}
			w.WriteHeader(tt.args.status)
			if got := w.Status(); got != tt.want {
				t.Errorf("statusWriter.Status() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_statusWriter_Write(t *testing.T) {
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
				ResponseWriter: &responseRecorder{err: errors.New("Bang")},
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := &statusWriter{
				ResponseWriter: tt.fields.ResponseWriter,
				status:         tt.fields.status,
			}
			got, err := w.Write(tt.args.b)
			if (err != nil) != tt.wantErr {
				t.Fatalf("statusWriter.Write() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.wantLength {
				t.Errorf("statusWriter.Write() = %v, wantLength %v", got, tt.wantLength)
			}
			if got := w.Status(); got != tt.wantStatus {
				t.Errorf("statusWriter.Status() = %v, wantStatus %v", got, tt.wantStatus)
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := generateID(); len(got)/2 != tt.wantLen {
				t.Errorf("generateID() = %v, want len=%v", got, tt.wantLen)
			}
		})
	}
}

type responseRecorder struct {
	http.ResponseWriter
	err error
}

func (rw *responseRecorder) Write(buf []byte) (int, error) {
	return len(buf), rw.err
}
