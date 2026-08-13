package util

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"sync"
	"time"

	ociCommon "github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/secrets"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/newrelic/oci-log-integration/logs-function/logger"
)

var log = logger.NewLogrusLogger(logger.WithDebugLevel())

// OCISecretsManagerAPI is an interface for interacting with OCI Secrets Manager.
type OCISecretsManagerAPI interface {
	GetSecretBundle(ctx context.Context, request secrets.GetSecretBundleRequest) (secrets.GetSecretBundleResponse, error)
	SetRegion(regionId string)
}

// getSecretFromOCIVault retrieves a secret from OCI Vault.
// It returns the secret string and an error if any.
func getSecretFromOCIVault(ctx context.Context, secretsClient OCISecretsManagerAPI, secretOCID string, vaultRegion string) (string, error) {
	// Check if the passed secret OCID is empty
	if secretOCID == "" {
		log.Panicf("secret OCID is empty")
	}

	// Check if the vault region is empty
	if vaultRegion == "" {
		log.Panicf("vault region is empty")
	}

	// Set the region for secrets client
	secretsClient.SetRegion(vaultRegion)

	getSecretBundleRequest := secrets.GetSecretBundleRequest{
		SecretId: ociCommon.String(secretOCID),
	}

	scResponse, err := secretsClient.GetSecretBundle(ctx, getSecretBundleRequest)
	if err != nil {
		return "", fmt.Errorf("failed to fetch secret bundle: %w", err)
	}
	log.Debug("successfully fetched secret from OCI vault")

	secretContent, ok := scResponse.SecretBundleContent.(secrets.Base64SecretBundleContentDetails)
	if !ok {
		log.WithField("secretOCID", secretOCID).Error("unexpected secret content type")
		return "", fmt.Errorf("unexpected secret content type")
	}

	if secretContent.Content == nil {
		log.WithField("secretOCID", secretOCID).Error("secret content is nil")
		return "", fmt.Errorf("secret content is nil")
	}

	decodedSecret, err := base64.StdEncoding.DecodeString(*secretContent.Content)
	if err != nil {
		log.WithField("error", err).WithField("secretOCID", secretOCID).Error("failed to base64 decode secret content")
		return "", fmt.Errorf("failed to decode secret content: %w", err)
	}

	return string(decodedSecret), nil
}

// newOCISecretsManagerClient creates a new OCI Secrets Manager client.
// It returns an OCISecretsManagerAPI client and an error if any.
func newOCISecretsManagerClient() (OCISecretsManagerAPI, error) {
	var provider ociCommon.ConfigurationProvider
	var err error

	provider, err = auth.ResourcePrincipalConfigurationProvider()
	if err != nil {
		log.WithField("error", err).Error("failed to create resource principal configuration provider")
		return nil, fmt.Errorf("failed to create resource principal configuration provider: %w", err)
	}

	secretsClient, err := secrets.NewSecretsClientWithConfigurationProvider(provider)
	if err != nil {
		log.WithField("error", err).Error("failed to create OCI secrets client")
		return nil, fmt.Errorf("failed to create OCI secrets client: %w", err)
	}

	return &secretsClient, nil
}

var (
	licenseKeyCacheMu   sync.Mutex
	cachedLicenseKey    string
	cachedLicenseKeyErr error
	licenseKeyCachedAt  time.Time
)

// GetLicenseKey returns the license key from the OCI Secrets Manager. TTL-cached (same
// window as the New Relic client caches, see getClientTTL) so that the logs client and the
// metrics client -- both of which call this -- share a single Vault fetch per cache window
// instead of one each.
func GetLicenseKey() (key string, err error) {
	licenseKeyCacheMu.Lock()
	defer licenseKeyCacheMu.Unlock()

	if !licenseKeyCachedAt.IsZero() && time.Since(licenseKeyCachedAt) < getClientTTL() {
		return cachedLicenseKey, cachedLicenseKeyErr
	}

	cachedLicenseKey, cachedLicenseKeyErr = fetchLicenseKey()
	licenseKeyCachedAt = time.Now()

	return cachedLicenseKey, cachedLicenseKeyErr
}

// fetchLicenseKey does the actual Vault round-trip, uncached.
func fetchLicenseKey() (string, error) {
	ctx := context.Background()
	log.Debug("fetching license key from OCI vault")

	secretOCID := os.Getenv(common.SecretOCID)
	vaultRegion := os.Getenv(common.VaultRegion)

	secretsClient, err := newOCISecretsManagerClient()
	if err != nil {
		return "", err
	}

	secretValue, err := getSecretFromOCIVault(ctx, secretsClient, secretOCID, vaultRegion)
	if err != nil {
		return "", err
	}

	return secretValue, nil
}
