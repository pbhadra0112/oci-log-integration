// Package util provides utility functions for New Relic client operations,
// secret management, and message processing for the OCI log integration.
package util

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/newrelic/newrelic-client-go/v2/pkg/config"
	logging "github.com/newrelic/newrelic-client-go/v2/pkg/logs"
	"github.com/newrelic/newrelic-client-go/v2/pkg/region"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/newrelic/oci-log-integration/logs-function/metrics"
)

// Global variables for caching the NewRelic client with TTL support
var (
	cachedNRClient  NewRelicClientAPI
	nrClientError   error
	clientCacheTime time.Time
)

// NewRelicClientAPI is an interface that defines the methods for interacting with the New Relic Logs API.
type NewRelicClientAPI interface {
	CreateLogEntry(logEntry interface{}) error
}

// ConsumeLogBatches consumes log batches from a channel and creates log entries using the provided NewRelicClientAPI.
// The function returns when the channel is closed or the context is cancelled. rec may be nil.
func ConsumeLogBatches(ctx context.Context, channel <-chan common.DetailedLogsBatch, wg *sync.WaitGroup, nrClientAPI NewRelicClientAPI, rec *metrics.Recorder) {
	// Defer the Done() method of the WaitGroup to indicate that the goroutine has finished processing
	defer wg.Done()

	for {
		select {
		case batch, ok := <-channel:
			if !ok {
				return
			}

			recordCount := countBatchEntries(batch)

			start := time.Now()
			err := nrClientAPI.CreateLogEntry(batch)
			duration := time.Since(start).Seconds()

			if err != nil {
				log.Errorf("error posting Log entry: %v", err)
				rec.Summary(metrics.TierBasic, "forwarder.delivery.duration", duration, map[string]interface{}{"status": "error"})
				rec.Count(metrics.TierBasic, "forwarder.records.dropped", float64(recordCount), map[string]interface{}{"reason": "delivery_error"})
				// Continue processing other batches instead of terminating
				continue
			}

			rec.Summary(metrics.TierBasic, "forwarder.delivery.duration", duration, map[string]interface{}{"status": "success"})
			rec.Count(metrics.TierBasic, "forwarder.records.delivered", float64(recordCount), map[string]interface{}{"status": "success"})
		case <-ctx.Done():
			// Context has been cancelled, exit the goroutine
			return
		}
	}
}

// countBatchEntries counts the total number of log records across all DetailedLogs in a batch.
func countBatchEntries(batch common.DetailedLogsBatch) int {
	total := 0
	for _, detailedLog := range batch {
		total += len(detailedLog.Entries)
	}
	return total
}

// NewNRClient Initializes a new NRClient with debug level and region
// It returns a NewRelicClientAPI interface and an error if there is a problem setting the region.
// Uses TTL-based caching for performance in OCI Function environment.
func NewNRClient() (NewRelicClientAPI, error) {
	// Check if cache is still valid
	if cachedNRClient != nil {
		ttl := getClientTTL()
		if time.Since(clientCacheTime) < ttl {
			// Return cached client (even if there was an error before)
			log.Debug("Returning cached New Relic client")
			return cachedNRClient, nrClientError
		}
	}

	// Cache is invalid, expired, or doesn't exist - create new client
	log.Debug("Initializing/refreshing New Relic client")
	cachedNRClient, nrClientError = createNRClient()
	clientCacheTime = time.Now()

	if nrClientError == nil {
		log.Debug("New Relic client initialized successfully")
	}

	return cachedNRClient, nrClientError
}

// getClientTTL returns the TTL for the client cache from environment variable or default (600 seconds = 10 minutes)
func getClientTTL() time.Duration {
	ttlSeconds := common.DefaultClientTTL // Default TTL in seconds

	if envTTL := os.Getenv(common.ClientTTL); envTTL != "" {
		if parsedTTL, err := strconv.Atoi(envTTL); err == nil && parsedTTL > 0 {
			ttlSeconds = parsedTTL
		}
	}

	return time.Duration(ttlSeconds) * time.Second
}

// createNRClient creates a new NewRelic client instance
func createNRClient() (NewRelicClientAPI, error) {
	nrRegion, err := region.Get(region.Name(os.Getenv(common.NewRelicRegion)))
	if err != nil {
		log.Warnf("could not resolve NEW_RELIC_REGION %q, falling back to default region: %v", os.Getenv(common.NewRelicRegion), err)
	}
	var nrClient logging.Logs
	cfg := config.Config{
		Compression: config.Compression.Gzip,
	}

	if os.Getenv(common.DebugEnabled) == "true" {
		cfg.LogLevel = "debug"
	} else {
		cfg.LogLevel = "info"
	}

	if err := cfg.SetRegion(nrRegion); err != nil {
		return &nrClient, err
	}

	licenseKey, err := GetLicenseKey()
	cfg.LicenseKey = licenseKey
	nrClient = logging.New(cfg)
	return &nrClient, err
}
