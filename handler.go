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

type statusWriter struct {
	http.ResponseWriter
	status int
	length int64
}

func (w *statusWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}

	return w.status
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}

	n, err := w.ResponseWriter.Write(b)
	w.length += int64(n)
	if err != nil {
		return n, errors.Wrap(err, "http.ResponseWriter.Write()")
	}

	return n, nil
}

// generateID provides an id that matches the trace id format
func generateID() string {
	t := [16]byte{}

	_, _ = rand.Read(t[:])

	return hex.EncodeToString(t[:])
}
