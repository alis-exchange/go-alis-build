// Copyright 2022 The Alis Build Platform. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package alog provides structured logging that adheres to the principles of Structured Logging as described in the
Google documentation: https://cloud.google.com/logging/docs/structured-logging. It formats logs as LogEntry objects as
defined by Google, which can be found here: https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry.

When using this package in a Google Cloud environment (such as Google Kubernetes Engine, Cloud Run, or App Engine
flexible), the structured logs will be written as JSON objects to stdout or stderr. These logs will be picked up by
the Logging agent and sent to Cloud Logging as the jsonPayload of the LogEntry structure.

The package also includes a local environment mode that formats logs in a human-friendly way while developing locally.
*/
package alog //import "go.alis.build/alog"
