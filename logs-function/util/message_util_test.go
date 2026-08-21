package util

import (
	"context"
	"testing"
	"time"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/stretchr/testify/assert"
)

// TestProduceMessageToChannel tests the ProduceMessageToChannel function
func TestProduceMessageToChannel(t *testing.T) {
	channel := make(chan BatchMessage, 1)

	currentBatch := common.LogData{
		map[string]interface{}{
			"message": map[string]interface{}{
				"level": "info",
				"text":  "test log message 1",
			},
		},
		map[string]interface{}{
			"message": map[string]interface{}{
				"level": "error",
				"text":  "test log message 2",
			},
		},
	}

	attributes := common.LogAttributes{
		"instrumentation.provider": common.InstrumentationProvider,
		"instrumentation.name":     common.InstrumentationName,
		"instrumentation.version":  common.InstrumentationVersion,
	}

	expectedMessage := BatchMessage{
		Batch: common.DetailedLogsBatch{{
			CommonData: common.Common{
				Attributes: attributes,
			},
			Entries: currentBatch,
		}},
		SizeBytes: 42,
	}
	sent := ProduceMessageToChannel(context.Background(), channel, currentBatch, attributes, 42)
	assert.True(t, sent)
	receivedMessage := <-channel

	assert.Equal(t, expectedMessage, receivedMessage)

	close(channel)
}

// TestProduceMessageToChannel_ContextCancelled verifies the send bails out instead of
// blocking forever when the channel is full and ctx is cancelled.
func TestProduceMessageToChannel_ContextCancelled(t *testing.T) {
	channel := make(chan BatchMessage) // unbuffered: the send below can never succeed on its own

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	sent := ProduceMessageToChannel(ctx, channel, common.LogData{}, common.LogAttributes{}, 1)

	assert.False(t, sent)
}
