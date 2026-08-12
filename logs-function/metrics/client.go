package metrics

import (
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/newrelic/newrelic-client-go/v2/pkg/config"
	nrmetrics "github.com/newrelic/newrelic-client-go/v2/pkg/metrics"
	"github.com/newrelic/newrelic-client-go/v2/pkg/region"

	"github.com/newrelic/oci-log-integration/logs-function/common"
)

// ClientAPI is the subset of newrelic-client-go's Metrics client this package depends on.
type ClientAPI interface {
	CreateMetricEntry(metricEntry interface{}) error
}

// LicenseKeyFunc supplies the New Relic license key on demand. Kept as a callback (rather
// than importing util directly) so this package has no dependency on the util package.
type LicenseKeyFunc func() (string, error)

var (
	clientCacheMu   sync.Mutex
	cachedClient    ClientAPI
	cachedClientErr error
	clientCachedAt  time.Time
)

// NewClient returns a TTL-cached New Relic Metrics API client, mirroring the caching
// pattern already used for the logs client in util.NewNRClient.
func NewClient(getLicenseKey LicenseKeyFunc) (ClientAPI, error) {
	clientCacheMu.Lock()
	defer clientCacheMu.Unlock()

	if cachedClient != nil && time.Since(clientCachedAt) < clientTTL() {
		return cachedClient, cachedClientErr
	}

	cachedClient, cachedClientErr = createClient(getLicenseKey)
	clientCachedAt = time.Now()

	return cachedClient, cachedClientErr
}

func clientTTL() time.Duration {
	ttlSeconds := common.DefaultClientTTL
	if envTTL := os.Getenv(common.ClientTTL); envTTL != "" {
		if parsed, err := strconv.Atoi(envTTL); err == nil && parsed > 0 {
			ttlSeconds = parsed
		}
	}
	return time.Duration(ttlSeconds) * time.Second
}

func createClient(getLicenseKey LicenseKeyFunc) (ClientAPI, error) {
	nrRegion, _ := region.Get(region.Name(os.Getenv(common.NewRelicRegion)))
	cfg := config.Config{Compression: config.Compression.Gzip}

	if err := cfg.SetRegion(nrRegion); err != nil {
		return nil, err
	}

	licenseKey, err := getLicenseKey()
	if err != nil {
		return nil, err
	}
	cfg.LicenseKey = licenseKey

	client := nrmetrics.New(cfg)
	return &client, nil
}
