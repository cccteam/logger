package logger

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
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

// NewCliLogger returns a function that executes the given function and creates a top-level parent log.
// The provided command string is used to identify the CLI execution in the logs.
func NewCliLogger(e Exporter) func(ctx context.Context, command string, f func(context.Context) error) error {
	return e.CliRunner()
}

// Exporter is the interface for implementing a middleware to export logs to some destination
type Exporter interface {
	Middleware() func(http.Handler) http.Handler
	CliRunner() func(ctx context.Context, command string, f func(context.Context) error) error
}

func requestSize(length string) int64 {
	l, err := strconv.Atoi(length)
	if err != nil {
		return 0
	}

	return int64(l)
}

func newResponseRecorder(w http.ResponseWriter) responseRecorder {
	_, isFlusher := w.(http.Flusher)
	_, isHijacker := w.(http.Hijacker)

	switch {
	case isFlusher && isHijacker:
		return &recorderFlusherHijacker{
			recorder: recorder{ResponseWriter: w},
		}
	case isFlusher:
		return &recorderFlusher{
			recorder: recorder{ResponseWriter: w},
		}
	case isHijacker:
		return &recorderHijacker{
			recorder: recorder{ResponseWriter: w},
		}
	default:
		return &recorder{
			ResponseWriter: w,
		}
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

type recorderHijacker struct {
	recorder
}

func (r *recorderHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		conn, w, err := h.Hijack()
		if err != nil {
			return conn, w, errors.Wrap(err, "http.Hijacker.Hijack")
		}

		return conn, w, nil
	}

	return nil, nil, http.ErrNotSupported
}

type recorderFlusherHijacker struct {
	recorder
}

func (r *recorderFlusherHijacker) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *recorderFlusherHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		conn, w, err := h.Hijack()
		if err != nil {
			return conn, w, errors.Wrap(err, "http.Hijacker.Hijack")
		}

		return conn, w, nil
	}

	return nil, nil, http.ErrNotSupported
}

// generateID provides an id that matches the trace id format
func generateID() string {
	t := [16]byte{}

	_, _ = rand.Read(t[:])

	return hex.EncodeToString(t[:])
}
