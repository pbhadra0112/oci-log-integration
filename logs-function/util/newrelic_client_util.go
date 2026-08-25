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
// The function returns when the channel is closed or the context is cancelled.
func ConsumeLogBatches(ctx context.Context, channel <-chan common.DetailedLogsBatch, wg *sync.WaitGroup, nrClientAPI NewRelicClientAPI) {
	// Defer the Done() method of the WaitGroup to indicate that the goroutine has finished processing
	defer wg.Done()

	for {
		select {
		case batch, ok := <-channel:
			if !ok {
				return
			}
			if err := nrClientAPI.CreateLogEntry(batch); err != nil {
				entryCount := 0
				for _, detailedLog := range batch {
					entryCount += len(detailedLog.Entries)
				}
				log.WithField("entryCount", entryCount).
					WithField("error", err).
					Error("error posting log entry, batch dropped")
				// Continue processing other batches instead of terminating
				continue
			}
		case <-ctx.Done():
			// Context has been cancelled, exit the goroutine
			return
		}
	}
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
	regionName := os.Getenv(common.NewRelicRegion)
	nrRegion, _ := region.Get(region.Name(regionName))
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
		log.WithField("region", regionName).WithField("error", err).Error("failed to set New Relic region")
		return &nrClient, err
	}

	licenseKey, err := GetLicenseKey()
	if err != nil {
		log.WithField("error", err).Error("failed to fetch New Relic license key")
		return &nrClient, err
	}
	cfg.LicenseKey = licenseKey
	nrClient = logging.New(cfg)
	return &nrClient, err
}
