package metrics

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/stretchr/testify/assert"
)

func resetClientCache() {
	clientCacheMu.Lock()
	defer clientCacheMu.Unlock()
	cachedClient = nil
	cachedClientErr = nil
	clientCachedAt = time.Time{}
}

func TestNewClient_PropagatesLicenseKeyError(t *testing.T) {
	resetClientCache()
	defer os.Unsetenv(common.NewRelicRegion)
	assert.NoError(t, os.Setenv(common.NewRelicRegion, "us"))

	wantErr := errors.New("secret fetch failed")
	client, err := NewClient(func() (string, error) { return "", wantErr })

	assert.Nil(t, client)
	assert.Equal(t, wantErr, err)
}

func TestNewClient_CachesSuccessfulClient(t *testing.T) {
	resetClientCache()
	defer os.Unsetenv(common.NewRelicRegion)
	assert.NoError(t, os.Setenv(common.NewRelicRegion, "us"))

	calls := 0
	getLicenseKey := func() (string, error) {
		calls++
		return "test-license-key", nil
	}

	client1, err := NewClient(getLicenseKey)
	assert.NoError(t, err)
	assert.NotNil(t, client1)

	client2, err := NewClient(getLicenseKey)
	assert.NoError(t, err)
	assert.Same(t, client1, client2)
	assert.Equal(t, 1, calls, "license key should only be fetched once while cache is valid")
}
