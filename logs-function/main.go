// Package main implements an Oracle Cloud Infrastructure (OCI) Function that processes
// OCI Logging events and forwards them to New Relic's logging platform. The function
// handles event unmarshaling, batching, and concurrent processing for optimal performance.
package main

import (
	"context"
	"io"
	"os"
	"sync"

	"github.com/fnproject/fdk-go"
	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/newrelic/oci-log-integration/logs-function/logger"
	"github.com/newrelic/oci-log-integration/logs-function/loggroup"
	"github.com/newrelic/oci-log-integration/logs-function/metrics"
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

// handleFunction processes OCI logging events and forwards them to New Relic. It also
// accumulates this invocation's forwarder.* custom metrics and forwards them itself, via
// the same New Relic client library, right before returning.
func handleFunction(ctx context.Context, in io.Reader, out io.Writer) {
	rec := metrics.NewRecorder(commonMetricAttributes())

	defer func() {
		status := "success"
		panicked := recover()
		if panicked != nil {
			status = "error"
		}

		rec.Count(metrics.TierBasic, "forwarder.invocations", 1, map[string]interface{}{"status": status})
		flushMetrics(rec)

		if panicked != nil {
			panic(panicked)
		}
	}()

	// Create NewRelic client during function invocation, not startup
	nrClient, err := util.NewNRClient()
	if err != nil {
		log.Panicf("error initializing newrelic client: %v", err)
	}

	handleFunctionWithClient(ctx, in, out, nrClient, rec)
}

// handleFunctionWithClient processes OCI logging events and forwards them to New Relic.
// It unmarshals incoming events, starts worker goroutines to process log batches concurrently,
// and waits for all processing to complete before returning. rec may be nil.
func handleFunctionWithClient(ctx context.Context, in io.Reader, _ io.Writer, nrClient util.NewRelicClientAPI, rec *metrics.Recorder) {
	event := unmarshal.Event{}
	if err := event.Unmarshal(in, rec); err != nil {
		log.Panicf("Error unmarshalling event: %v", err)
	}

	channel := make(chan common.DetailedLogsBatch, common.MessageChannelSize)
	var wg sync.WaitGroup
	wg.Add(common.NumberOfWorkers)

	// Start multiple worker goroutines to process log batches concurrently
	for i := 0; i < common.NumberOfWorkers; i++ {
		go util.ConsumeLogBatches(ctx, channel, &wg, nrClient, rec)
	}

	switch event.EventType {
	case unmarshal.OCI_LOGGING:
		loggroup.ProcessLogs(event.OCILoggingEvent, channel, rec)
	default:
		log.Warnf("Unknown event type: %s", event.EventType)
	}

	// Close channel after processing to signal completion
	close(channel)
	// Wait for goroutines to finish processing
	wg.Wait()
}

// flushMetrics forwards this invocation's accumulated custom metrics to New Relic's Metric
// API, reusing the same license key already fetched for the logs client.
func flushMetrics(rec *metrics.Recorder) {
	if rec == nil || rec.Tier() == metrics.TierNone {
		return
	}

	client, err := metrics.NewClient(util.GetLicenseKey)
	if err != nil {
		log.Warnf("could not initialize metrics client, skipping metrics flush: %v", err)
		return
	}

	if err := rec.Flush(client); err != nil {
		log.Warnf("failed to flush custom metrics: %v", err)
	}
}

// commonMetricAttributes returns the dimensions attached to every custom metric this
// invocation emits.
func commonMetricAttributes() map[string]interface{} {
	return map[string]interface{}{
		"cloud":         common.InstrumentationProvider,
		"region":        os.Getenv(common.VaultRegion),
		"version":       common.InstrumentationVersion,
		"function_name": os.Getenv(common.FunctionNameEnvVar),
	}
}
