// Package util provides generic utility functions.
package util

import (
	"context"

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

// ProduceMessageToChannel sends a log batch to a channel for further processing. It returns
// false instead of blocking forever if ctx is cancelled first -- e.g. all consumer workers are
// stuck on a slow New Relic API call and the channel buffer is full -- so the caller can bail
// out of processing the rest of the invocation instead of hanging until the OCI function's own
// hard timeout kills it.
func ProduceMessageToChannel(ctx context.Context, channel chan BatchMessage, currentBatch common.LogData, attributes common.LogAttributes, sizeBytes int) bool {
	msg := BatchMessage{
		Batch: common.DetailedLogsBatch{{
			CommonData: common.Common{
				Attributes: attributes,
			},
			Entries: currentBatch,
		}},
		SizeBytes: sizeBytes,
	}

	select {
	case channel <- msg:
		return true
	case <-ctx.Done():
		return false
	}
}
