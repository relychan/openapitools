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
	"testing"
	"time"

	"github.com/failsafe-go/failsafe-go/retrypolicy"
	"github.com/relychan/gohttpc"
	"github.com/relychan/gohttpc/authc/authscheme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
)

func newTestClientOptions() *clientOptions {
	return &clientOptions{
		ClientOptions: gohttpc.NewClientOptions(),
	}
}

func TestWithClientOptions(t *testing.T) {
	opts := gohttpc.NewClientOptions()
	opts.UserAgent = "test-agent"

	co := newTestClientOptions()
	WithClientOptions(opts)(co)

	assert.Equal(t, opts, co.ClientOptions)
	assert.Equal(t, "test-agent", co.UserAgent)
}

func TestWithHTTPClient(t *testing.T) {
	httpClient := &http.Client{Timeout: 5 * time.Second}

	co := newTestClientOptions()
	WithHTTPClient(httpClient)(co)

	assert.Equal(t, httpClient, co.HTTPClient)
}

func TestWithTraceHighCardinalityPath(t *testing.T) {
	co := newTestClientOptions()

	WithTraceHighCardinalityPath(true)(co)
	assert.True(t, co.TraceHighCardinalityPath)

	WithTraceHighCardinalityPath(false)(co)
	assert.False(t, co.TraceHighCardinalityPath)
}

func TestWithMetricHighCardinalityPath(t *testing.T) {
	co := newTestClientOptions()

	WithMetricHighCardinalityPath(true)(co)
	assert.True(t, co.MetricHighCardinalityPath)

	WithMetricHighCardinalityPath(false)(co)
	assert.False(t, co.MetricHighCardinalityPath)
}

func TestWithCustomAttributesFunc(t *testing.T) {
	called := false
	fn := gohttpc.CustomAttributesFunc(func(_ gohttpc.Requester) []attribute.KeyValue {
		called = true
		return nil
	})

	co := newTestClientOptions()
	WithCustomAttributesFunc(fn)(co)

	require.NotNil(t, co.CustomAttributesFunc)
	co.CustomAttributesFunc(nil)
	assert.True(t, called)
}

func TestWithRetry(t *testing.T) {
	policy := retrypolicy.NewBuilder[*http.Response]().WithMaxRetries(3).Build()

	co := newTestClientOptions()
	WithRetry(policy)(co)

	assert.Equal(t, policy, co.Retry)
}

func TestWithTimeout(t *testing.T) {
	co := newTestClientOptions()
	WithTimeout(30 * time.Second)(co)

	assert.Equal(t, 30*time.Second, co.Timeout)
}

type mockAuthenticator struct{}

func (m *mockAuthenticator) Authenticate(_ *http.Request, _ ...authscheme.AuthenticateOption) error {
	return nil
}

func (m *mockAuthenticator) Close() error { return nil }

func TestWithAuthenticator(t *testing.T) {
	auth := &mockAuthenticator{}

	co := newTestClientOptions()
	WithAuthenticator(auth)(co)

	assert.Equal(t, auth, co.Authenticator)
}

func TestEnableClientTrace(t *testing.T) {
	co := newTestClientOptions()

	EnableClientTrace(true)(co)
	assert.True(t, co.ClientTraceEnabled)

	EnableClientTrace(false)(co)
	assert.False(t, co.ClientTraceEnabled)
}

func TestAllowTraceRequestHeaders(t *testing.T) {
	keys := []string{"X-Request-ID", "X-Correlation-ID"}

	co := newTestClientOptions()
	AllowTraceRequestHeaders(keys)(co)

	assert.Equal(t, keys, co.AllowedTraceRequestHeaders)
}

func TestAllowTraceResponseHeaders(t *testing.T) {
	keys := []string{"X-Response-ID", "ETag"}

	co := newTestClientOptions()
	AllowTraceResponseHeaders(keys)(co)

	assert.Equal(t, keys, co.AllowedTraceResponseHeaders)
}

func TestWithUserAgent(t *testing.T) {
	co := newTestClientOptions()
	WithUserAgent("my-service/1.0")(co)

	assert.Equal(t, "my-service/1.0", co.UserAgent)
}

func TestWithGetEnvFunc(t *testing.T) {
	t.Run("non-nil getter is applied", func(t *testing.T) {
		getter := func(key string) (string, error) {
			return "value-" + key, nil
		}

		co := newTestClientOptions()
		WithGetEnvFunc(getter)(co)

		require.NotNil(t, co.GetEnv)
		val, err := co.GetEnv("FOO")
		require.NoError(t, err)
		assert.Equal(t, "value-FOO", val)
	})

	t.Run("nil getter is ignored", func(t *testing.T) {
		co := newTestClientOptions()
		original := func(key string) (string, error) {
			return "original-" + key, nil
		}
		co.GetEnv = original

		WithGetEnvFunc(nil)(co)

		require.NotNil(t, co.GetEnv)
		val, err := co.GetEnv("BAR")
		require.NoError(t, err)
		assert.Equal(t, "original-BAR", val)
	})
}

func TestWithRetry_Build(t *testing.T) {
	policy := retrypolicy.NewBuilder[*http.Response]().
		WithMaxRetries(2).
		AbortOnErrors(errors.New("abort")).
		Build()

	co := newTestClientOptions()
	WithRetry(policy)(co)

	assert.NotNil(t, co.Retry)
	assert.Equal(t, policy, co.Retry)
}
