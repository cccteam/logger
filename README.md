# logger

[![GoDoc](https://img.shields.io/badge/pkg.go.dev-doc-blue)](http://pkg.go.dev/github.com/cccteam/logger)

**logger** is an HTTP request logger that implements correlated logging to one of several supported platforms. Each HTTP request is logged as the parent log, with all logs generated during the request as child logs.

The Logging destination is configured with an Exporter. This package provides Exporters for **Google Cloud Logging**, **AWS Logging**,
and **Console Logging**.

The _**GoogleCloudExporter**_ will also correlate logs to **Cloud Trace** if you instrument your code with tracing.

The _**ConsoleExporter**_ is useful for local development and debugging.

The _**AWSExporter**_ will also correlate logs to **AWS X-Ray** if you instrument your code with tracing and have logs sent to Cloudwatch. Note that additional configuration in the tracing is required to enable the correlation. In the tracing configuration, you must set the log group names to where the logs are being sent.

- Open telemetry documentation for [AWS logs](https://opentelemetry.io/docs/specs/otel/resource/semantic_conventions/cloud_provider/aws/logs/)
- X-Ray documentation for [log correlation](https://aws-otel.github.io/docs/getting-started/x-ray#using-config-to-set-cloud-watch-log-group-names)
