// Package metricstest provides shared test doubles for exercising code that flushes
// metrics.Recorder data, so callers (loggroup, util) don't each redefine the same mock.
package metricstest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/newrelic/oci-log-integration/logs-function/metrics"
)

// MockClient is a mock implementation of metrics.ClientAPI.
type MockClient struct {
	mock.Mock
}

// CreateMetricEntry records the call and returns the configured error, per testify/mock.
func (m *MockClient) CreateMetricEntry(metricEntry interface{}) error {
	args := m.Called(metricEntry)
	return args.Error(0)
}

// FlushedPayload flushes rec through a MockClient and returns the raw payload that would
// have been sent to New Relic's Metric API, for tests that need to assert on more than just
// which metric names fired (e.g. an actual value or attribute).
func FlushedPayload(t *testing.T, rec *metrics.Recorder) []map[string]interface{} {
	t.Helper()
	client := &MockClient{}
	client.On("CreateMetricEntry", mock.Anything).Return(nil)
	assert.NoError(t, rec.Flush(client))

	return client.Calls[0].Arguments[0].([]map[string]interface{})
}

// FlushedMetricNames flushes rec through a MockClient and returns the set of metric names
// that were sent.
func FlushedMetricNames(t *testing.T, rec *metrics.Recorder) map[string]bool {
	t.Helper()
	payload := FlushedPayload(t, rec)
	metricsList := payload[0]["metrics"].([]map[string]interface{})

	names := map[string]bool{}
	for _, m := range metricsList {
		names[m["name"].(string)] = true
	}
	return names
}
