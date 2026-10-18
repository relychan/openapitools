// Copyright 2026 RelyChan Pte. Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package openapiclient

import (
	"errors"
	"net/http"

	"github.com/hasura/gotel"
)

var tracer = gotel.NewTracer("openapitools")

var (
	ErrServerURLRequired = errors.New("server url is required")
	ErrMetadataRequired  = errors.New("metadata for the proxy client is required")
	ErrNoAvailableServer = errors.New(
		"failed to initialize servers. Require at least 1 server has URL",
	)
)

type tracingResponseWriter interface {
	// BytesWritten returns the total number of bytes sent to the client.
	BytesWritten() int
}

type headerTrackingResponseWriter interface {
	// HeaderWritten checks if the response header was written to the client.
	HeaderWritten() bool
}

// trackingResponseWriter wraps an [http.ResponseWriter] to track whether
// the response header and body were written.
type trackingResponseWriter struct {
	http.ResponseWriter

	bytesWritten  int
	headerWritten bool
}

var (
	_ tracingResponseWriter        = (*trackingResponseWriter)(nil)
	_ headerTrackingResponseWriter = (*trackingResponseWriter)(nil)
	_ http.Flusher                 = (*trackingResponseWriter)(nil)
)

// newTrackingResponseWriter wraps the response writer if it is not a [trackingResponseWriter] yet.
func newTrackingResponseWriter(writer http.ResponseWriter) *trackingResponseWriter {
	if tw, ok := writer.(*trackingResponseWriter); ok {
		return tw
	}

	return &trackingResponseWriter{ResponseWriter: writer}
}

// WriteHeader sends an HTTP response header with the provided status code.
func (w *trackingResponseWriter) WriteHeader(statusCode int) {
	if w.headerWritten {
		return
	}

	// Informational headers (1xx) except 101 Switching Protocols do not commit the response.
	if statusCode >= 100 && statusCode < 200 && statusCode != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(statusCode)

		return
	}

	w.headerWritten = true
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write writes the data to the connection as part of an HTTP reply.
func (w *trackingResponseWriter) Write(data []byte) (int, error) {
	w.headerWritten = true

	n, err := w.ResponseWriter.Write(data)
	w.bytesWritten += n

	return n, err
}

// Flush sends any buffered data to the client.
func (w *trackingResponseWriter) Flush() {
	// Flushing commits the response header.
	w.headerWritten = true

	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

// Unwrap returns the inner response writer. It is used by [http.ResponseController].
func (w *trackingResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// BytesWritten returns the total number of bytes sent to the client.
func (w *trackingResponseWriter) BytesWritten() int {
	return w.bytesWritten
}

// HeaderWritten checks if the response header was written to the client.
func (w *trackingResponseWriter) HeaderWritten() bool {
	return w.headerWritten
}
