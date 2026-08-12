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
func ProcessLogs(OCILoggingEvent common.OCILoggingEvent, channel chan common.DetailedLogsBatch, rec *metrics.Recorder) {
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
func splitLogsIntoBatches(logs common.OCILoggingEvent, maxPayloadSize int, commonAttributes common.LogAttributes, channel chan common.DetailedLogsBatch, rec *metrics.Recorder) {
	var currentBatch common.LogData
	currentBatchSize := 0

	for _, logData := range logs {
		if env := metrics.ExtractEnvelope(logData); env.HasTime {
			rec.Summary(metrics.TierBasic, "forwarder.pipeline.lag", time.Since(env.Time).Seconds(), nil)
		}

		logBytes, err := json.Marshal(logData)
		if err != nil {
			log.Warnf("Warning: Could not marshal detailed log for size estimation: %v", err)
			continue
		}
		logSize := len(logBytes)

		// this case handles the case where a single log entry is larger than the maxpayload size.
		// In this case OCI has a 1MB limit per log line, we try to push this to New Relic anyway
		if len(currentBatch) == 0 {
			currentBatch = common.LogData{logData}
			currentBatchSize = logSize
		} else if currentBatchSize+logSize > maxPayloadSize && len(currentBatch) > 0 {
			util.ProduceMessageToChannel(channel, currentBatch, commonAttributes)
			currentBatch = common.LogData{logData}
			currentBatchSize = logSize
		} else {
			currentBatch = append(currentBatch, logData)
			currentBatchSize += logSize
		}
	}

	if len(currentBatch) > 0 {
		util.ProduceMessageToChannel(channel, currentBatch, commonAttributes)
	}
}
