// Package util provides generic utility functions.
package util

import (
	"github.com/newrelic/oci-log-integration/logs-function/common"
)

// BatchMessage carries a log batch to ConsumeLogBatches along with its size in bytes,
// pre-computed by the caller while building the batch (an approximation: the sum of each
// entry's own marshaled size, not counting the DetailedLog/Common envelope overhead). This
// lets forwarder.bytes.delivered reuse that value instead of re-marshaling the whole batch.
type BatchMessage struct {
	Batch     common.DetailedLogsBatch
	SizeBytes int
}

// ProduceMessageToChannel sends a log batch to a channel for further processing.
func ProduceMessageToChannel(channel chan BatchMessage, currentBatch common.LogData, attributes common.LogAttributes, sizeBytes int) {
	channel <- BatchMessage{
		Batch: common.DetailedLogsBatch{{
			CommonData: common.Common{
				Attributes: attributes,
			},
			Entries: currentBatch,
		}},
		SizeBytes: sizeBytes,
	}
}
