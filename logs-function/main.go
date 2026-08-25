// Package main implements an Oracle Cloud Infrastructure (OCI) Function that processes
// OCI Logging events and forwards them to New Relic's logging platform. The function
// handles event unmarshaling, batching, and concurrent processing for optimal performance.
package main

import (
	"context"
	"io"
	"runtime/debug"
	"sync"

	"github.com/fnproject/fdk-go"
	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/newrelic/oci-log-integration/logs-function/logger"
	"github.com/newrelic/oci-log-integration/logs-function/loggroup"
	"github.com/newrelic/oci-log-integration/logs-function/unmarshal"
	"github.com/newrelic/oci-log-integration/logs-function/util"
)

var log = logger.NewLogrusLogger(logger.WithDebugLevel())

func main() {
	log.Debug("Setting up function handler")
	handler := func(ctx context.Context, in io.Reader, out io.Writer) {
		handleFunction(ctx, in, out)
	}
	fdk.Handle(fdk.HandlerFunc(handler))
}

// handleFunction processes OCI logging events and forwards them to New Relic.
// It creates the NewRelic client on each invocation (like your working simple function).
func handleFunction(ctx context.Context, in io.Reader, out io.Writer) {
	defer func() {
		if r := recover(); r != nil {
			log.WithField("panic", r).
				WithField("stack", string(debug.Stack())).
				Error("function invocation failed")
			panic(r)
		}
	}()

	// Create NewRelic client during function invocation, not startup
	nrClient, err := util.NewNRClient()
	if err != nil {
		log.Panicf("error initializing newrelic client: %v", err)
	}

	handleFunctionWithClient(ctx, in, out, nrClient)
}

// handleFunctionWithClient processes OCI logging events and forwards them to New Relic.
// It unmarshals incoming events, starts worker goroutines to process log batches concurrently,
// and waits for all processing to complete before returning.
func handleFunctionWithClient(ctx context.Context, in io.Reader, _ io.Writer, nrClient util.NewRelicClientAPI) {
	event := unmarshal.Event{}
	if err := event.Unmarshal(in); err != nil {
		log.Panicf("Error unmarshalling event: %v", err)
	}

	channel := make(chan common.DetailedLogsBatch, common.MessageChannelSize)
	var wg sync.WaitGroup
	wg.Add(common.NumberOfWorkers)

	// Start multiple worker goroutines to process log batches concurrently
	for i := 0; i < common.NumberOfWorkers; i++ {
		go util.ConsumeLogBatches(ctx, channel, &wg, nrClient)
	}

	var batchCount, logCount, skippedCount int
	switch event.EventType {
	case unmarshal.OCI_LOGGING:
		batchCount, logCount, skippedCount = loggroup.ProcessLogs(event.OCILoggingEvent, channel)
	default:
		log.Warnf("Unknown event type: %s", event.EventType)
	}

	// Close channel after processing to signal completion
	close(channel)
	// Wait for goroutines to finish processing
	wg.Wait()

	log.WithField("batchCount", batchCount).
		WithField("logCount", logCount).
		WithField("skippedCount", skippedCount).
		Debug("invocation complete")
}
