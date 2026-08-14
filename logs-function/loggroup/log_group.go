// Package loggroup provides functionality for processing and batching OCI log events
// for efficient transmission to New Relic's logging API.
package loggroup

import (
	"encoding/json"
	"time"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/newrelic/oci-log-integration/logs-function/logger"
	"github.com/newrelic/oci-log-integration/logs-function/metrics"
	"github.com/newrelic/oci-log-integration/logs-function/util"
)

var log = logger.NewLogrusLogger(logger.WithDebugLevel())

// ProcessLogs processes OCI logging events and splits them into batches for New Relic ingestion.
// It adds instrumentation metadata to each batch and sends the batches through the provided channel.
// The function respects payload size limits to ensure compatibility with New Relic's API constraints.
// rec may be nil.
func ProcessLogs(OCILoggingEvent common.OCILoggingEvent, channel chan util.BatchMessage, rec *metrics.Recorder) {
	attributes := common.LogAttributes{
		"instrumentation.provider": common.InstrumentationProvider,
		"instrumentation.name":     common.InstrumentationName,
		"instrumentation.version":  common.InstrumentationVersion,
	}

	splitLogsIntoBatches(OCILoggingEvent, common.MaxPayloadSize, attributes, channel, rec)
}

// splitLogsIntoBatches splits the incoming logs into batches for processing.
// It loosely respects (if a single log entry exceeds the maximum payload size we still try to send it)
// the maximum payload size and sends each batch through the provided channel.
func splitLogsIntoBatches(logs common.OCILoggingEvent, maxPayloadSize int, commonAttributes common.LogAttributes, channel chan util.BatchMessage, rec *metrics.Recorder) {
	var currentBatch common.LogData
	currentBatchSize := 0

	for _, logData := range logs {
		if env := metrics.ExtractEnvelope(logData); env.HasTime {
			rec.Summary(metrics.TierBasic, "forwarder.pipeline.lag", time.Since(env.Time).Seconds(), nil)
		}

		logBytes, err := json.Marshal(logData)
		if err != nil {
			rec.Count(metrics.TierAdvanced, "forwarder.serialize.errors", 1, nil)
			log.Warnf("Warning: Could not marshal detailed log for size estimation: %v", err)
			continue
		}
		logSize := len(logBytes)

		// OCI has a 1MB limit per log line; a single entry that alone exceeds maxPayloadSize
		// is still pushed to New Relic (see below), but flagged here. Checked unconditionally
		// per record rather than only when a record happens to start a new batch -- otherwise
		// an oversized record arriving after another batch was already flushed would slip
		// through uncounted.
		if logSize > maxPayloadSize {
			rec.Count(metrics.TierAdvanced, "forwarder.records.oversized", 1, nil)
		}

		if len(currentBatch) == 0 {
			currentBatch = common.LogData{logData}
			currentBatchSize = logSize
		} else if currentBatchSize+logSize > maxPayloadSize && len(currentBatch) > 0 {
			produceBatch(channel, currentBatch, commonAttributes, currentBatchSize, rec)
			currentBatch = common.LogData{logData}
			currentBatchSize = logSize
		} else {
			currentBatch = append(currentBatch, logData)
			currentBatchSize += logSize
		}
	}

	if len(currentBatch) > 0 {
		produceBatch(channel, currentBatch, commonAttributes, currentBatchSize, rec)
	}
}

// produceBatch sends a completed batch to the channel and records its advanced-tier
// batching metrics.
func produceBatch(channel chan util.BatchMessage, batch common.LogData, commonAttributes common.LogAttributes, batchSize int, rec *metrics.Recorder) {
	util.ProduceMessageToChannel(channel, batch, commonAttributes, batchSize)
	rec.Count(metrics.TierAdvanced, "forwarder.batches.created", 1, nil)
	rec.Summary(metrics.TierAdvanced, "forwarder.batch.size_bytes", float64(batchSize), nil)
}
