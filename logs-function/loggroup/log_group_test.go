package loggroup

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/newrelic/oci-log-integration/logs-function/metrics"
	"github.com/newrelic/oci-log-integration/logs-function/metrics/metricstest"
	"github.com/newrelic/oci-log-integration/logs-function/util"
	"github.com/stretchr/testify/assert"
)

// TestProcessLogs tests the ProcessLogs function
func TestProcessLogs(t *testing.T) {
	tests := []struct {
		name               string
		ociLoggingEvent    common.OCILoggingEvent
		expectedBatches    int
		expectedAttributes map[string]interface{}
		description        string
	}{
		{
			name: "single log entry",
			ociLoggingEvent: common.OCILoggingEvent{
				map[string]interface{}{
					"timestamp": "2023-01-01T12:00:00Z",
					"level":     "INFO",
					"message":   "Test message",
					"service":   "test-service",
				},
			},
			expectedBatches: 1,
			expectedAttributes: map[string]interface{}{
				"instrumentation.provider": common.InstrumentationProvider,
				"instrumentation.name":     common.InstrumentationName,
				"instrumentation.version":  common.InstrumentationVersion,
			},
			description: "Should process single log entry and create one batch",
		},
		{
			name: "multiple log entries",
			ociLoggingEvent: common.OCILoggingEvent{
				map[string]interface{}{
					"timestamp": "2023-01-01T12:00:00Z",
					"level":     "INFO",
					"message":   "Message 1",
					"service":   "service-1",
				},
				map[string]interface{}{
					"timestamp": "2023-01-01T12:00:01Z",
					"level":     "ERROR",
					"message":   "Message 2",
					"service":   "service-2",
				},
				map[string]interface{}{
					"timestamp": "2023-01-01T12:00:02Z",
					"level":     "WARN",
					"message":   "Message 3",
					"service":   "service-3",
				},
			},
			expectedBatches: 1,
			expectedAttributes: map[string]interface{}{
				"instrumentation.provider": common.InstrumentationProvider,
				"instrumentation.name":     common.InstrumentationName,
				"instrumentation.version":  common.InstrumentationVersion,
			},
			description: "Should process multiple log entries into batches",
		},
		{
			name:            "empty log event",
			ociLoggingEvent: common.OCILoggingEvent{},
			expectedBatches: 0,
			expectedAttributes: map[string]interface{}{
				"instrumentation.provider": common.InstrumentationProvider,
				"instrumentation.name":     common.InstrumentationName,
				"instrumentation.version":  common.InstrumentationVersion,
			},
			description: "Should handle empty log events without creating batches",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := make(chan util.BatchMessage, 10)

			ProcessLogs(context.Background(), tt.ociLoggingEvent, channel, nil)

			close(channel)
			var batches []common.DetailedLogsBatch
			for msg := range channel {
				batches = append(batches, msg.Batch)
			}

			assert.Len(t, batches, tt.expectedBatches, "Expected %d batches, got %d", tt.expectedBatches, len(batches))

			if tt.expectedBatches > 0 && len(batches) > 0 {
				for _, batch := range batches {
					assert.Len(t, batch, 1, "Each batch should contain one DetailedLog")
					detailedLog := batch[0]

					for key, expectedValue := range tt.expectedAttributes {
						actualValue, exists := detailedLog.CommonData.Attributes[key]
						assert.True(t, exists, "Attribute %s should exist", key)
						assert.Equal(t, expectedValue, actualValue, "Attribute %s should have correct value", key)
					}

					assert.NotEmpty(t, detailedLog.Entries, "DetailedLog should have entries")
				}
			}
		})
	}
}

// TestSplitLogsIntoBatches tests the splitLogsIntoBatches function
func TestSplitLogsIntoBatches(t *testing.T) {
	tests := []struct {
		name            string
		logs            common.OCILoggingEvent
		maxPayloadSize  int
		expectedBatches int
		description     string
	}{
		{
			name: "single small log fits in one batch",
			logs: common.OCILoggingEvent{
				map[string]interface{}{
					"level":   "INFO",
					"message": "Small message",
				},
			},
			maxPayloadSize:  1000,
			expectedBatches: 1,
			description:     "Single small log should fit in one batch",
		},
		{
			name: "multiple small logs fit in one batch",
			logs: common.OCILoggingEvent{
				map[string]interface{}{
					"level":   "INFO",
					"message": "Message 1",
				},
				map[string]interface{}{
					"level":   "INFO",
					"message": "Message 2",
				},
				map[string]interface{}{
					"level":   "INFO",
					"message": "Message 3",
				},
			},
			maxPayloadSize:  1000,
			expectedBatches: 1,
			description:     "Multiple small logs should fit in one batch",
		},
		{
			name: "logs exceed max payload size create multiple batches",
			logs: common.OCILoggingEvent{
				map[string]interface{}{
					"level":   "INFO",
					"message": "This is a longer message that will help us test payload size limits and batching behavior",
				},
				map[string]interface{}{
					"level":   "ERROR",
					"message": "Another longer message to test batching when payload size is exceeded",
				},
				map[string]interface{}{
					"level":   "WARN",
					"message": "Yet another message to ensure we create multiple batches",
				},
			},
			maxPayloadSize:  100,
			expectedBatches: 3,
			description:     "Logs exceeding payload size should create multiple batches",
		},
		{
			name:            "empty logs create no batches",
			logs:            common.OCILoggingEvent{},
			maxPayloadSize:  1000,
			expectedBatches: 0,
			description:     "Empty logs should create no batches",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := make(chan util.BatchMessage, 10)

			commonAttributes := common.LogAttributes{
				"test.attribute": "test.value",
			}

			splitLogsIntoBatches(context.Background(), tt.logs, tt.maxPayloadSize, commonAttributes, channel, nil)

			close(channel)
			var batches []common.DetailedLogsBatch
			for msg := range channel {
				batches = append(batches, msg.Batch)
			}

			assert.Len(t, batches, tt.expectedBatches, "Expected %d batches, got %d", tt.expectedBatches, len(batches))

			if tt.expectedBatches > 0 {
				totalLogs := 0
				for _, batch := range batches {
					assert.Len(t, batch, 1, "Each batch should contain one DetailedLog")
					detailedLog := batch[0]
					totalLogs += len(detailedLog.Entries)

					assert.Equal(t, "test.value", detailedLog.CommonData.Attributes["test.attribute"])
				}
				assert.Equal(t, len(tt.logs), totalLogs, "All logs should be included across batches")
			}
		})
	}
}

// TestSplitLogsIntoBatchesPayloadSizeAccuracy tests payload size calculation accuracy
func TestSplitLogsIntoBatchesPayloadSizeAccuracy(t *testing.T) {
	logs := common.OCILoggingEvent{
		map[string]interface{}{
			"msg": "a",
		},
		map[string]interface{}{
			"message": "This is a medium-sized log entry that should fit within reasonable payload limits",
		},
	}

	channel := make(chan util.BatchMessage, 10)
	commonAttributes := common.LogAttributes{
		"test": "value",
	}

	splitLogsIntoBatches(context.Background(), logs, 50, commonAttributes, channel, nil)

	close(channel)
	var batches []common.DetailedLogsBatch
	for msg := range channel {
		batches = append(batches, msg.Batch)
	}

	assert.Len(t, batches, 2, "Should create 2 batches due to payload size limits")
}

// TestProcessLogsWithChannel tests the channel communication
func TestProcessLogsWithChannel(t *testing.T) {
	logs := common.OCILoggingEvent{
		map[string]interface{}{
			"timestamp": "2023-01-01T12:00:00Z",
			"level":     "INFO",
			"message":   "Test message 1",
		},
		map[string]interface{}{
			"timestamp": "2023-01-01T12:00:01Z",
			"level":     "ERROR",
			"message":   "Test message 2",
		},
	}

	channel := make(chan util.BatchMessage, 5)

	ProcessLogs(context.Background(), logs, channel, nil)

	select {
	case msg := <-channel:
		batch := msg.Batch
		assert.NotEmpty(t, batch, "Should receive a non-empty batch")
		assert.Len(t, batch, 1, "Batch should contain one DetailedLog")

		detailedLog := batch[0]
		assert.Equal(t, common.InstrumentationProvider, detailedLog.CommonData.Attributes["instrumentation.provider"])
		assert.Equal(t, common.InstrumentationName, detailedLog.CommonData.Attributes["instrumentation.name"])
		assert.Equal(t, common.InstrumentationVersion, detailedLog.CommonData.Attributes["instrumentation.version"])
		assert.Len(t, detailedLog.Entries, 2, "Should contain both log entries")

	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for batch from channel")
	}

	close(channel)
}

// TestProcessLogsAttributes tests that correct attributes are set
func TestProcessLogsAttributes(t *testing.T) {
	logs := common.OCILoggingEvent{
		map[string]interface{}{
			"level":   "INFO",
			"message": "Test",
		},
	}

	channel := make(chan util.BatchMessage, 1)

	ProcessLogs(context.Background(), logs, channel, nil)

	close(channel)
	msg := <-channel
	batch := msg.Batch

	assert.Len(t, batch, 1)
	detailedLog := batch[0]

	expectedAttributes := map[string]interface{}{
		"instrumentation.provider": common.InstrumentationProvider,
		"instrumentation.name":     common.InstrumentationName,
		"instrumentation.version":  common.InstrumentationVersion,
	}

	for key, expectedValue := range expectedAttributes {
		actualValue, exists := detailedLog.CommonData.Attributes[key]
		assert.True(t, exists, "Attribute %s should exist", key)
		assert.Equal(t, expectedValue, actualValue, "Attribute %s should have correct value", key)
	}

	assert.Len(t, detailedLog.CommonData.Attributes, len(expectedAttributes), "Should only have expected attributes")
}

// TestSplitLogsIntoBatches_PipelineLag verifies forwarder.pipeline.lag is recorded when a
// record's OCI Logging envelope carries a time field.
func TestSplitLogsIntoBatches_PipelineLag(t *testing.T) {
	assert.NoError(t, os.Setenv(common.MetricsTier, common.MetricsTierBasic))
	defer os.Unsetenv(common.MetricsTier)

	rec := metrics.NewRecorder(nil)
	channel := make(chan util.BatchMessage, 10)

	logs := common.OCILoggingEvent{
		map[string]interface{}{
			"time":    time.Now().Add(-5 * time.Second).UTC().Format(time.RFC3339),
			"message": "hi",
		},
	}

	splitLogsIntoBatches(context.Background(), logs, 1000, common.LogAttributes{}, channel, rec)
	close(channel)
	for range channel {
	}

	names := metricstest.FlushedMetricNames(t, rec)
	assert.True(t, names["forwarder.pipeline.lag"], "expected forwarder.pipeline.lag to be recorded")
}

// TestSplitLogsIntoBatches_BatchingMetrics verifies forwarder.batches.created and
// forwarder.batch.size_bytes are recorded when a batch is produced.
func TestSplitLogsIntoBatches_BatchingMetrics(t *testing.T) {
	assert.NoError(t, os.Setenv(common.MetricsTier, common.MetricsTierAdvanced))
	defer os.Unsetenv(common.MetricsTier)

	rec := metrics.NewRecorder(nil)
	channel := make(chan util.BatchMessage, 10)

	logs := common.OCILoggingEvent{
		map[string]interface{}{"message": "one"},
		map[string]interface{}{"message": "two"},
	}

	splitLogsIntoBatches(context.Background(), logs, 1000, common.LogAttributes{}, channel, rec)
	close(channel)
	for range channel {
	}

	names := metricstest.FlushedMetricNames(t, rec)
	assert.True(t, names["forwarder.batches.created"])
	assert.True(t, names["forwarder.batch.size_bytes"])
}

// TestSplitLogsIntoBatches_RecordsOversized verifies forwarder.records.oversized fires
// when a single log entry alone exceeds maxPayloadSize.
func TestSplitLogsIntoBatches_RecordsOversized(t *testing.T) {
	assert.NoError(t, os.Setenv(common.MetricsTier, common.MetricsTierAdvanced))
	defer os.Unsetenv(common.MetricsTier)

	rec := metrics.NewRecorder(nil)
	channel := make(chan util.BatchMessage, 10)

	logs := common.OCILoggingEvent{
		map[string]interface{}{"message": "this single log entry is deliberately longer than the tiny max payload size configured below"},
	}

	splitLogsIntoBatches(context.Background(), logs, 10, common.LogAttributes{}, channel, rec)
	close(channel)
	for range channel {
	}

	names := metricstest.FlushedMetricNames(t, rec)
	assert.True(t, names["forwarder.records.oversized"])
}

// TestSplitLogsIntoBatches_RecordsOversized_NotFirstInStream verifies
// forwarder.records.oversized still fires for an oversized record that arrives after an
// earlier (non-oversized) record already started a batch -- oversized-ness is a per-record
// property, not something that only gets checked when a record happens to start a batch.
func TestSplitLogsIntoBatches_RecordsOversized_NotFirstInStream(t *testing.T) {
	assert.NoError(t, os.Setenv(common.MetricsTier, common.MetricsTierAdvanced))
	defer os.Unsetenv(common.MetricsTier)

	rec := metrics.NewRecorder(nil)
	channel := make(chan util.BatchMessage, 10)

	logs := common.OCILoggingEvent{
		map[string]interface{}{"message": "small"},
		map[string]interface{}{"message": "this second log entry is deliberately longer than the tiny max payload size configured below"},
	}

	splitLogsIntoBatches(context.Background(), logs, 30, common.LogAttributes{}, channel, rec)
	close(channel)
	for range channel {
	}

	names := metricstest.FlushedMetricNames(t, rec)
	assert.True(t, names["forwarder.records.oversized"], "the second, oversized record should still be counted even though it isn't first in the stream")
}

// TestSplitLogsIntoBatches_SerializeErrors verifies forwarder.serialize.errors fires when
// a log record can't be marshaled for size estimation.
func TestSplitLogsIntoBatches_SerializeErrors(t *testing.T) {
	assert.NoError(t, os.Setenv(common.MetricsTier, common.MetricsTierAdvanced))
	defer os.Unsetenv(common.MetricsTier)

	rec := metrics.NewRecorder(nil)
	channel := make(chan util.BatchMessage, 10)

	logs := common.OCILoggingEvent{
		map[string]interface{}{"unmarshalable": make(chan int)},
	}

	splitLogsIntoBatches(context.Background(), logs, 1000, common.LogAttributes{}, channel, rec)
	close(channel)
	for range channel {
	}

	names := metricstest.FlushedMetricNames(t, rec)
	assert.True(t, names["forwarder.serialize.errors"])
}
