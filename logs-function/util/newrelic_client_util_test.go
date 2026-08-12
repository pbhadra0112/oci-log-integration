// Package util provides utility functions for the OCI log integration New Relic client operations.

package util

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/newrelic/oci-log-integration/logs-function/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockMetricsClient is a mock for metrics.ClientAPI, used to assert what ConsumeLogBatches/
// NewNRClient record via the Recorder without a real New Relic Metric API call.
type mockMetricsClient struct {
	mock.Mock
}

func (m *mockMetricsClient) CreateMetricEntry(metricEntry interface{}) error {
	args := m.Called(metricEntry)
	return args.Error(0)
}

// flushedMetricNames flushes rec through a mock metrics client and returns the set of metric
// names that were sent, plus the payload's common attributes.
func flushedMetricNames(t *testing.T, rec *metrics.Recorder) (map[string]bool, map[string]interface{}) {
	t.Helper()
	client := &mockMetricsClient{}
	client.On("CreateMetricEntry", mock.Anything).Return(nil)
	assert.NoError(t, rec.Flush(client))

	payload := client.Calls[0].Arguments[0].([]map[string]interface{})
	metricsList := payload[0]["metrics"].([]map[string]interface{})
	commonAttrs := payload[0]["common"].(map[string]interface{})["attributes"].(map[string]interface{})

	names := map[string]bool{}
	for _, m := range metricsList {
		names[m["name"].(string)] = true
	}
	return names, commonAttrs
}

// Test helper function to reset NewRelic client cache
func resetNRClient() {
	cachedNRClient = nil
	nrClientError = nil
	clientCacheTime = time.Time{}
}

// MockNRClient is a mock type for the Logs interface.
type MockNRClient struct {
	mock.Mock
}

// CreateLogEntry is a mock method that satisfies the Logs interface.
func (m *MockNRClient) CreateLogEntry(batch interface{}) error {
	args := m.Called(batch)
	return args.Error(0)
}

// TestConsumeLogBatches tests the ConsumeLogBatches function.
func TestConsumeLogBatches(t *testing.T) {
	mockNRClient := new(MockNRClient)
	mockNRClient.On("CreateLogEntry", mock.Anything).Return(nil)

	channel := make(chan common.DetailedLogsBatch, 1)
	wg := new(sync.WaitGroup)

	logBatch := []common.DetailedLog{{
		CommonData: common.Common{
			Attributes: common.LogAttributes{
				"compartmentId": "ocid1.compartment.oc1..aaaaaaaa",
				"tenantId":      "ocid1.tenancy.oc1..bbbbbbbbb",
				"region":        "us-ashburn-1",
			},
		},
	}}

	channel <- logBatch

	ctx := context.TODO()
	wg.Add(1)
	go ConsumeLogBatches(ctx, channel, wg, mockNRClient, nil)
	close(channel)
	wg.Wait()
	mockNRClient.AssertNumberOfCalls(t, "CreateLogEntry", 1)
}

// TestGetClientTTL tests the getClientTTL function
func TestGetClientTTL(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		expectedTTL time.Duration
	}{
		{
			name:        "Default TTL when no env var",
			envValue:    "",
			expectedTTL: 600 * time.Second,
		},
		{
			name:        "Custom TTL from env var",
			envValue:    "300",
			expectedTTL: 300 * time.Second,
		},
		{
			name:        "Invalid TTL falls back to default",
			envValue:    "invalid",
			expectedTTL: 600 * time.Second,
		},
		{
			name:        "Zero TTL falls back to default",
			envValue:    "0",
			expectedTTL: 600 * time.Second,
		},
		{
			name:        "Negative TTL falls back to default",
			envValue:    "-5",
			expectedTTL: 600 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				err := os.Setenv(common.ClientTTL, tt.envValue)
				if err != nil {
					t.Fatalf("Failed to set environment variable: %v", err)
				}
				defer func() {
					err := os.Unsetenv(common.ClientTTL)
					if err != nil {
						t.Errorf("Failed to unset environment variable: %v", err)
					}
				}()
			} else {
				err := os.Unsetenv(common.ClientTTL)
				if err != nil {
					t.Errorf("Failed to unset environment variable: %v", err)
				}
			}

			actualTTL := getClientTTL()
			assert.Equal(t, tt.expectedTTL, actualTTL)
		})
	}
}

// TestNewNRClient_CacheLogic tests the caching logic of NewNRClient
func TestNewNRClient_CacheLogic(t *testing.T) {
	resetNRClient()

	err := os.Setenv(common.NewRelicRegion, "us")
	if err != nil {
		t.Fatalf("Failed to set NewRelicRegion environment variable: %v", err)
	}
	err = os.Setenv(common.ClientTTL, "60")
	if err != nil {
		t.Fatalf("Failed to set ClientTTL environment variable: %v", err)
	}
	defer func() {
		err := os.Unsetenv(common.NewRelicRegion)
		if err != nil {
			t.Errorf("Failed to unset NewRelicRegion environment variable: %v", err)
		}
		err = os.Unsetenv(common.ClientTTL)
		if err != nil {
			t.Errorf("Failed to unset ClientTTL environment variable: %v", err)
		}
	}()

	_, _ = NewNRClient()
	firstCacheTime := clientCacheTime
	assert.False(t, firstCacheTime.IsZero(), "Cache time should be set after first call")

	_, _ = NewNRClient()
	secondCacheTime := clientCacheTime
	assert.Equal(t, firstCacheTime, secondCacheTime, "Cache time should not change for cached response")
}

// TestNewNRClient_CacheExpiration tests that cache expires correctly
func TestNewNRClient_CacheExpiration(t *testing.T) {
	resetNRClient()

	err := os.Setenv(common.ClientTTL, "1")
	if err != nil {
		t.Fatalf("Failed to set ClientTTL environment variable: %v", err)
	}
	err = os.Setenv(common.NewRelicRegion, "us")
	if err != nil {
		t.Fatalf("Failed to set NewRelicRegion environment variable: %v", err)
	}
	defer func() {
		err := os.Unsetenv(common.ClientTTL)
		if err != nil {
			t.Errorf("Failed to unset ClientTTL environment variable: %v", err)
		}
		err = os.Unsetenv(common.NewRelicRegion)
		if err != nil {
			t.Errorf("Failed to unset NewRelicRegion environment variable: %v", err)
		}
	}()

	_, _ = NewNRClient()
	firstCacheTime := clientCacheTime

	time.Sleep(2 * time.Second)

	_, _ = NewNRClient()
	secondCacheTime := clientCacheTime

	assert.True(t, secondCacheTime.After(firstCacheTime), "Cache should have been refreshed after TTL expiration")
}

// TestConsumeLogBatches_ErrorHandling tests error handling in log processing
func TestConsumeLogBatches_ErrorHandling(t *testing.T) {
	mockNRClient := new(MockNRClient)

	mockNRClient.On("CreateLogEntry", mock.Anything).Return(assert.AnError)

	channel := make(chan common.DetailedLogsBatch, 2)
	wg := new(sync.WaitGroup)

	// Send two batches
	logBatch1 := []common.DetailedLog{{
		CommonData: common.Common{
			Attributes: common.LogAttributes{
				"compartmentId": "ocid1.compartment.oc1..aaaaaaaa",
			},
		},
	}}
	logBatch2 := []common.DetailedLog{{
		CommonData: common.Common{
			Attributes: common.LogAttributes{
				"compartmentId": "ocid1.compartment.oc1..bbbbbbbbb",
			},
		},
	}}

	channel <- logBatch1
	channel <- logBatch2

	ctx := context.TODO()
	wg.Add(1)
	go ConsumeLogBatches(ctx, channel, wg, mockNRClient, nil)
	close(channel)
	wg.Wait()

	mockNRClient.AssertNumberOfCalls(t, "CreateLogEntry", 2)
}

// TestConsumeLogBatches_RecordsDeliveredMetrics verifies a successful delivery records
// forwarder.records.delivered and forwarder.delivery.duration.
func TestConsumeLogBatches_RecordsDeliveredMetrics(t *testing.T) {
	assert.NoError(t, os.Setenv(common.MetricsTier, common.MetricsTierBasic))
	defer os.Unsetenv(common.MetricsTier)

	mockNRClient := new(MockNRClient)
	mockNRClient.On("CreateLogEntry", mock.Anything).Return(nil)

	rec := metrics.NewRecorder(nil)
	channel := make(chan common.DetailedLogsBatch, 1)
	channel <- []common.DetailedLog{{Entries: common.LogData{
		map[string]interface{}{"message": "one"},
		map[string]interface{}{"message": "two"},
	}}}

	ctx := context.TODO()
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go ConsumeLogBatches(ctx, channel, wg, mockNRClient, rec)
	close(channel)
	wg.Wait()

	names, _ := flushedMetricNames(t, rec)
	assert.True(t, names["forwarder.records.delivered"])
	assert.True(t, names["forwarder.delivery.duration"])
	assert.False(t, names["forwarder.records.dropped"])
}

// TestConsumeLogBatches_RecordsDroppedMetrics verifies a failed delivery records
// forwarder.records.dropped instead of the success metrics.
func TestConsumeLogBatches_RecordsDroppedMetrics(t *testing.T) {
	assert.NoError(t, os.Setenv(common.MetricsTier, common.MetricsTierBasic))
	defer os.Unsetenv(common.MetricsTier)

	mockNRClient := new(MockNRClient)
	mockNRClient.On("CreateLogEntry", mock.Anything).Return(assert.AnError)

	rec := metrics.NewRecorder(nil)
	channel := make(chan common.DetailedLogsBatch, 1)
	channel <- []common.DetailedLog{{Entries: common.LogData{
		map[string]interface{}{"message": "one"},
	}}}

	ctx := context.TODO()
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go ConsumeLogBatches(ctx, channel, wg, mockNRClient, rec)
	close(channel)
	wg.Wait()

	names, _ := flushedMetricNames(t, rec)
	assert.True(t, names["forwarder.records.dropped"])
	assert.False(t, names["forwarder.records.delivered"])
}
