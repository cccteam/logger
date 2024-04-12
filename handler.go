package logger

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"

	"github.com/go-playground/errors/v5"
)

// NewRequestLogger returns a middleware that logs the request and injects a Logger into
// the context. This Logger can be used during the life of the request, and all logs
// generated will be correlated to the request log.
//
// If not configured, request logs are sent to stderr by default.
func NewRequestLogger(e Exporter) func(http.Handler) http.Handler {
	return e.Middleware()
}

// Exporter is the interface for implementing a middleware to export logs to some destination
type Exporter interface {
	Middleware() func(http.Handler) http.Handler
}

func requestSize(length string) int64 {
	l, err := strconv.Atoi(length)
	if err != nil {
		return 0
	}

	return int64(l)
}

func newResponseRecorder(w http.ResponseWriter) responseRecorder {
	if _, ok := w.(http.Flusher); ok {
		return &recorderFlusher{
			recorder: recorder{
				ResponseWriter: w,
			},
		}
	}

	return &recorder{
		ResponseWriter: w,
	}
}

type responseRecorder interface {
	http.ResponseWriter
	Status() int
	WriteHeader(status int)
	Write(b []byte) (int, error)
	Length() int64
}

type recorder struct {
	http.ResponseWriter
	status int
	length int64
}

func (r *recorder) Status() int {
	if r.status == 0 {
		return http.StatusOK
	}

	return r.status
}

func (r *recorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *recorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.length += int64(n)
	if err != nil {
		return n, errors.Wrap(err, "http.ResponseWriter.Write()")
	}

	return n, nil
}

func (r *recorder) Length() int64 {
	return r.length
}

type recorderFlusher struct {
	recorder
}

func (r *recorderFlusher) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// generateID provides an id that matches the trace id format
func generateID() string {
	t := [16]byte{}

	_, _ = rand.Read(t[:])

	return hex.EncodeToString(t[:])
}
