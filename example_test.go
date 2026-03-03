package logger

import (
	"context"
	"fmt"
	"net/http"
)

// This example demonstrates creating a ConsoleExporter and using it
// to create an HTTP middleware that logs requests to the console.
func ExampleNewConsoleExporter() {
	exporter := NewConsoleExporter()

	// Use the exporter to create HTTP middleware
	middleware := NewRequestLogger(exporter)

	// Apply the middleware to an HTTP handler
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use the logger from the request context
		FromReq(r).Info("handling request")
		w.WriteHeader(http.StatusOK)
	}))

	// Use the handler with an HTTP server
	_ = handler
}

// This example demonstrates creating a ConsoleExporter with color disabled.
func ExampleConsoleExporter_NoColor() {
	exporter := NewConsoleExporter().NoColor(true)

	_ = exporter
}

// This example demonstrates using the ConsoleExporter to create a Logger
// for use outside of HTTP middleware, such as in background jobs.
func ExampleConsoleExporter_NewLogger() {
	exporter := NewConsoleExporter().NoColor(true)

	// Create a logger for a background job
	l := exporter.NewLogger(context.Background())

	// Use the logger as usual
	l.Info("background job started")
	l.Debugf("processing %d items", 42)
	l.Warn("something might be wrong")
	l.Error("something went wrong")
}

// This example demonstrates creating an AWSExporter for logging
// to AWS CloudWatch. When logAll is true, all requests are logged
// even if no child logs were written during the request.
func ExampleNewAWSExporter() {
	exporter := NewAWSExporter(true)

	// Use the exporter to create HTTP middleware
	middleware := NewRequestLogger(exporter)

	// Apply the middleware to an HTTP handler
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		FromReq(r).Info("handling request")
		w.WriteHeader(http.StatusOK)
	}))

	_ = handler
}

// This example demonstrates using the AWSExporter to create a Logger
// for use outside of HTTP middleware, such as in background jobs.
func ExampleAWSExporter_NewLogger() {
	exporter := NewAWSExporter(true)

	// Create a logger for a background job
	l := exporter.NewLogger(context.Background())

	// Use the logger as usual
	l.Info("background job started")
}

// This example demonstrates creating middleware from an Exporter using
// NewRequestLogger. The middleware injects a Logger into the request
// context that can be retrieved with FromReq or FromCtx.
func ExampleNewRequestLogger() {
	exporter := NewConsoleExporter()

	// Create the middleware
	middleware := NewRequestLogger(exporter)

	// Build a handler that uses the logger
	myHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Retrieve the logger and use it
		l := FromReq(r)
		l.Info("request received")
		l.Infof("method: %s, path: %s", r.Method, r.URL.Path)

		w.WriteHeader(http.StatusOK)
	})

	// Wrap the handler with logging middleware
	http.Handle("/", middleware(myHandler))
}

// This example demonstrates retrieving a Logger from a context.
// This is useful when you have a context but not an HTTP request,
// such as in service layers or helper functions.
func ExampleFromCtx() {
	exporter := NewConsoleExporter().NoColor(true)

	// Create a logger and associate it with a context
	l := exporter.NewLogger(context.Background())
	ctx := NewCtx(context.Background(), l)

	// Later, retrieve the logger from the context
	FromCtx(ctx).Info("retrieved from context")
}

// This example demonstrates retrieving a Logger from an HTTP request.
// This is the typical usage inside HTTP handlers when using the
// logging middleware.
func ExampleFromReq() {
	exporter := NewConsoleExporter()
	middleware := NewRequestLogger(exporter)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Retrieve the logger from the request
		l := FromReq(r)
		l.Info("processing request")

		w.WriteHeader(http.StatusOK)
	}))

	_ = handler
}

// This example demonstrates associating a Logger with a new context
// using NewCtx. This is useful for propagating the logger into
// goroutines or function calls that accept a context.
func ExampleNewCtx() {
	exporter := NewConsoleExporter().NoColor(true)

	// Create a logger
	l := exporter.NewLogger(context.Background())

	// Associate it with a new context
	ctx := NewCtx(context.Background(), l)

	// The logger can be retrieved from the new context
	FromCtx(ctx).Info("logger propagated via context")

	// This is particularly useful when passing context to other functions
	exampleDoWork(ctx)
}

func exampleDoWork(ctx context.Context) {
	FromCtx(ctx).Info("doing work")
}

// This example demonstrates using WithAttributes to add metadata
// to child log entries. Attributes are added to individual log
// lines rather than the parent request log.
func ExampleLogger_WithAttributes() {
	exporter := NewConsoleExporter().NoColor(true)
	l := exporter.NewLogger(context.Background())

	// Create a logger with additional attributes
	attrLogger := l.WithAttributes().
		AddAttribute("userID", "user-123").
		AddAttribute("action", "login").
		Logger()

	// All logs from this logger will include the attributes
	attrLogger.Info("user action recorded")
}

// This example demonstrates using AddRequestAttribute to add metadata
// to the parent request log entry. These attributes appear on the
// top-level request log, not on individual child log lines.
func ExampleLogger_AddRequestAttribute() {
	exporter := NewConsoleExporter()
	middleware := NewRequestLogger(exporter)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := FromReq(r)

		// Add attributes to the parent request log
		l.AddRequestAttribute("userID", "user-123").
			AddRequestAttribute("tenant", "acme-corp")

		l.Info("processing request")
		w.WriteHeader(http.StatusOK)
	}))

	_ = handler
}

// This example demonstrates retrieving the trace ID from the logger.
// The trace ID can be used to correlate logs across services.
func ExampleLogger_TraceID() {
	exporter := NewConsoleExporter()
	middleware := NewRequestLogger(exporter)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := FromReq(r)

		// Get the trace ID for correlation
		traceID := l.TraceID()
		fmt.Printf("trace: %s\n", traceID)

		w.WriteHeader(http.StatusOK)
	}))

	_ = handler
}
