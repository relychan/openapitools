package openapiclient

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/failsafe-go/failsafe-go/retrypolicy"
	"github.com/hasura/goenvconf"
	"github.com/relychan/gohttpc"
	"github.com/relychan/gohttpc/authc/authscheme"
)

type clientOptions struct {
	*gohttpc.ClientOptions
}

// ClientOption abstracts a function to modify proxy client options.
type ClientOption func(*clientOptions)

// WithClientOptions create an option to set the client options of gohttpc client.
func WithClientOptions(options *gohttpc.ClientOptions) ClientOption {
	return func(co *clientOptions) {
		co.ClientOptions = options
	}
}

// WithHTTPClient create an option to set the HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(co *clientOptions) {
		co.HTTPClient = httpClient
	}
}

// WithTraceHighCardinalityPath enables high cardinality path on traces.
func WithTraceHighCardinalityPath(enabled bool) ClientOption {
	return func(co *clientOptions) {
		co.TraceHighCardinalityPath = enabled
	}
}

// WithMetricHighCardinalityPath enables high cardinality path on metrics.
func WithMetricHighCardinalityPath(enabled bool) ClientOption {
	return func(co *clientOptions) {
		co.MetricHighCardinalityPath = enabled
	}
}

// WithCustomAttributesFunc sets the function to add custom attributes to spans and metrics.
func WithCustomAttributesFunc(fn gohttpc.CustomAttributesFunc) ClientOption {
	return func(co *clientOptions) {
		co.CustomAttributesFunc = fn
	}
}

// WithRetry creates an option to set the default retry policy.
func WithRetry(retry retrypolicy.RetryPolicy[*http.Response]) ClientOption {
	return func(co *clientOptions) {
		co.Retry = retry
	}
}

// WithTimeout creates an option to set the default timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(co *clientOptions) {
		co.Timeout = timeout
	}
}

// WithLogLevel creates an option to set the level for printing logs.
func WithLogLevel(level slog.Level) ClientOption {
	return func(co *clientOptions) {
		co.LogLevel = level
	}
}

// WithAuthenticator creates an option to set the default authenticator.
func WithAuthenticator(authenticator authscheme.HTTPClientAuthenticator) ClientOption {
	return func(co *clientOptions) {
		co.Authenticator = authenticator
	}
}

// EnableClientTrace creates an option to enable the HTTP client trace.
func EnableClientTrace(enabled bool) ClientOption {
	return func(co *clientOptions) {
		co.ClientTraceEnabled = enabled
	}
}

// AllowTraceRequestHeaders creates an option to set allowed headers for tracing.
func AllowTraceRequestHeaders(keys []string) ClientOption {
	return func(co *clientOptions) {
		co.AllowedTraceRequestHeaders = keys
	}
}

// AllowTraceResponseHeaders creates an option to set allowed headers for tracing.
func AllowTraceResponseHeaders(keys []string) ClientOption {
	return func(co *clientOptions) {
		co.AllowedTraceResponseHeaders = keys
	}
}

// WithUserAgent creates an option to set the user agent.
func WithUserAgent(userAgent string) ClientOption {
	return func(co *clientOptions) {
		co.UserAgent = userAgent
	}
}

// WithGetEnvFunc returns a function to set the GetEnvFunc getter to [HTTPClientAuthenticatorOptions].
func WithGetEnvFunc(getter goenvconf.GetEnvFunc) ClientOption {
	return func(co *clientOptions) {
		if getter == nil {
			return
		}

		co.GetEnv = getter
	}
}
