// Package loggroup provides functionality for processing and batching OCI log events
// for efficient transmission to New Relic's logging API.
package loggroup

import (
	"encoding/json"
	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/newrelic/oci-log-integration/logs-function/logger"
	"github.com/newrelic/oci-log-integration/logs-function/util"
)

var log = logger.NewLogrusLogger(logger.WithDebugLevel())

// ProcessLogs processes OCI logging events and splits them into batches for New Relic ingestion.
// It adds instrumentation metadata to each batch and sends the batches through the provided channel.
// The function respects payload size limits to ensure compatibility with New Relic's API constraints.
// It returns the number of batches created, the number of log records sent, and the number of
// log records skipped (e.g. due to marshalling failures).
func ProcessLogs(OCILoggingEvent common.OCILoggingEvent, channel chan common.DetailedLogsBatch) (batchCount int, logCount int, skippedCount int) {
	attributes := common.LogAttributes{
		"instrumentation.provider": common.InstrumentationProvider,
		"instrumentation.name":     common.InstrumentationName,
		"instrumentation.version":  common.InstrumentationVersion,
	}

	log.WithField("recordCount", len(OCILoggingEvent)).Debug("processing OCI logging event")

	batchCount, logCount, skippedCount = splitLogsIntoBatches(OCILoggingEvent, common.MaxPayloadSize, attributes, channel)

	summary := log.WithField("batchCount", batchCount).
		WithField("logCount", logCount).
		WithField("skippedCount", skippedCount)
	if skippedCount > 0 {
		summary.Warn("finished processing OCI logging event with skipped records")
	} else {
		summary.Debug("finished processing OCI logging event")
	}

	return batchCount, logCount, skippedCount
}

// splitLogsIntoBatches splits the incoming logs into batches for processing.
// It loosely respects (if a single log entry exceeds the maximum payload size we still try to send it)
// the maximum payload size and sends each batch through the provided channel.
// It returns the number of batches created, the number of log records sent, and the number of
// log records skipped (e.g. due to marshalling failures).
func splitLogsIntoBatches(logs common.OCILoggingEvent, maxPayloadSize int, commonAttributes common.LogAttributes, channel chan common.DetailedLogsBatch) (batchCount int, logCount int, skippedCount int) {
	var currentBatch common.LogData
	currentBatchSize := 0

	for i, logData := range logs {
		logBytes, err := json.Marshal(logData)
		if err != nil {
			log.WithField("recordIndex", i).WithField("error", err).Warn("could not marshal detailed log for size estimation, skipping record")
			skippedCount++
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
			batchCount++
			logCount += len(currentBatch)
			currentBatch = common.LogData{logData}
			currentBatchSize = logSize
		} else {
			currentBatch = append(currentBatch, logData)
			currentBatchSize += logSize
		}
	}

	if len(currentBatch) > 0 {
		util.ProduceMessageToChannel(channel, currentBatch, commonAttributes)
		batchCount++
		logCount += len(currentBatch)
	}

	return batchCount, logCount, skippedCount
}
