# logger

[![GoDoc](https://img.shields.io/badge/pkg.go.dev-doc-blue)](http://pkg.go.dev/github.com/cccteam/logger)

**logger** is an HTTP request logger that implements correlated logging to one of several supported platforms. Each HTTP request is logged as the parent log, with all logs generated during the request as child logs.

The Logging destination is configured with an Exporter. This package provides Exporters for **Google Cloud Logging**
and **Console Logging**.

The _**GoogleCloudExporter**_ will also correlate logs to **Cloud Trace** if you instrument your code with tracing.
