package util

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/secrets"
	"github.com/stretchr/testify/assert"

	"github.com/newrelic/oci-log-integration/logs-function/common"
)

// Mock OCI Secrets Manager client for testing
type mockOCISecretsClient struct {
	shouldError     bool
	secretContent   string
	region          string
	forceNilContent bool
	invalidBase64   bool
}

func (m *mockOCISecretsClient) GetSecretBundle(ctx context.Context, request secrets.GetSecretBundleRequest) (secrets.GetSecretBundleResponse, error) {
	if m.shouldError {
		return secrets.GetSecretBundleResponse{}, errors.New("mock OCI secrets error")
	}

	var content *string

	if m.forceNilContent {
		content = nil
	} else if m.invalidBase64 {
		invalidBase64 := "invalid!base64!"
		content = &invalidBase64
	} else {
		encodedContent := base64.StdEncoding.EncodeToString([]byte(m.secretContent))
		content = &encodedContent
	}

	response := secrets.GetSecretBundleResponse{
		SecretBundle: secrets.SecretBundle{
			SecretBundleContent: secrets.Base64SecretBundleContentDetails{
				Content: content,
			},
		},
	}

	return response, nil
}

func (m *mockOCISecretsClient) SetRegion(regionId string) {
	m.region = regionId
}

func TestGetSecretFromOCIVault(t *testing.T) {
	tests := []struct {
		name           string
		secretOCID     string
		vaultRegion    string
		secretContent  string
		shouldError    bool
		expectedSecret string
		expectedError  string
	}{
		{
			name:           "successful secret retrieval",
			secretOCID:     "ocid1.vaultsecret.test",
			vaultRegion:    "us-phoenix-1",
			secretContent:  "test-license-key",
			shouldError:    false,
			expectedSecret: "test-license-key",
			expectedError:  "",
		},
		{
			name:           "empty secret OCID",
			secretOCID:     "",
			vaultRegion:    "us-phoenix-1",
			secretContent:  "",
			shouldError:    false,
			expectedSecret: "",
			expectedError:  "secret OCID is empty",
		},
		{
			name:           "empty vault region",
			secretOCID:     "ocid1.vaultsecret.test",
			vaultRegion:    "",
			secretContent:  "",
			shouldError:    false,
			expectedSecret: "",
			expectedError:  "vault region is empty",
		},
		{
			name:           "OCI API error",
			secretOCID:     "ocid1.vaultsecret.test",
			vaultRegion:    "us-phoenix-1",
			secretContent:  "",
			shouldError:    true,
			expectedSecret: "",
			expectedError:  "failed to fetch secret bundle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockOCISecretsClient{
				shouldError:   tt.shouldError,
				secretContent: tt.secretContent,
			}

			if tt.secretOCID == "" || tt.vaultRegion == "" {
				assert.Panics(t, func() {
					_, _ = getSecretFromOCIVault(context.Background(), mockClient, tt.secretOCID, tt.vaultRegion)
				}, "Expected panic for %s", tt.name)
				return
			}

			secret, err := getSecretFromOCIVault(context.Background(), mockClient, tt.secretOCID, tt.vaultRegion)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got nil", tt.expectedError)
				} else if err.Error() != tt.expectedError && !contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', but got '%s'", tt.expectedError, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if secret != tt.expectedSecret {
					t.Errorf("Expected secret '%s', but got '%s'", tt.expectedSecret, secret)
				}
			}

			// Verify region was set correctly (if not empty)
			if tt.vaultRegion != "" && !tt.shouldError && tt.secretOCID != "" {
				if mockClient.region != tt.vaultRegion {
					t.Errorf("Expected region '%s', but got '%s'", tt.vaultRegion, mockClient.region)
				}
			}
		})
	}
}

func TestGetLicenseKeyWithMockClient(t *testing.T) {
	tests := []struct {
		name           string
		secretOCID     string
		vaultRegion    string
		secretContent  string
		shouldError    bool
		expectedSecret string
		expectedError  string
	}{
		{
			name:           "successful secret retrieval",
			secretOCID:     "ocid1.vaultsecret.test",
			vaultRegion:    "us-phoenix-1",
			secretContent:  "test-license-key",
			shouldError:    false,
			expectedSecret: "test-license-key",
			expectedError:  "",
		},
		{
			name:           "JSON secret content",
			secretOCID:     "ocid1.vaultsecret.test",
			vaultRegion:    "us-phoenix-1",
			secretContent:  `{"licenseKey": "json-license-key"}`,
			shouldError:    false,
			expectedSecret: "json-license-key",
			expectedError:  "",
		},
		{
			name:           "empty secret content",
			secretOCID:     "ocid1.vaultsecret.test",
			vaultRegion:    "us-phoenix-1",
			secretContent:  "",
			shouldError:    false,
			expectedSecret: "",
			expectedError:  "license key secret is empty",
		},
		{
			name:           "JSON without license key",
			secretOCID:     "ocid1.vaultsecret.test",
			vaultRegion:    "us-phoenix-1",
			secretContent:  `{"otherKey": "other-value"}`,
			shouldError:    false,
			expectedSecret: "",
			expectedError:  "license key is empty or not present in the secret",
		},
		{
			name:           "OCI secrets error",
			secretOCID:     "ocid1.vaultsecret.test",
			vaultRegion:    "us-phoenix-1",
			secretContent:  "",
			shouldError:    true,
			expectedSecret: "",
			expectedError:  "mock OCI secrets error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockOCISecretsClient{
				shouldError:   tt.shouldError,
				secretContent: tt.secretContent,
			}

			secret, err := getSecretFromOCIVault(context.Background(), mockClient, tt.secretOCID, tt.vaultRegion)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected OCI error, but got nil")
				} else if !contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', but got '%s'", tt.expectedError, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error from GetSecretFromOCIVault: %v", err)
				return
			}

			licenseKey, extractErr := extractLicenseKeyFromSecret(secret)

			if tt.expectedError != "" {
				if extractErr == nil {
					t.Errorf("Expected error containing '%s', but got nil", tt.expectedError)
				} else if !contains(extractErr.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', but got '%s'", tt.expectedError, extractErr.Error())
				}
			} else {
				if extractErr != nil {
					t.Errorf("Expected no error, but got: %v", extractErr)
				}
				if licenseKey != tt.expectedSecret {
					t.Errorf("Expected license key '%s', but got '%s'", tt.expectedSecret, licenseKey)
				}
			}
		})
	}
}

// resetLicenseKeyCache clears GetLicenseKey's TTL cache between test cases that expect a
// fresh Vault fetch, mirroring resetNRClient in newrelic_client_util_test.go.
func resetLicenseKeyCache() {
	licenseKeyCacheMu.Lock()
	defer licenseKeyCacheMu.Unlock()
	cachedLicenseKey = ""
	cachedLicenseKeyErr = nil
	licenseKeyCachedAt = time.Time{}
}

// Helper function to extract license key from secret (for testing)
func extractLicenseKeyFromSecret(secretValue string) (string, error) {
	if secretValue == "" {
		return "", errors.New("license key secret is empty")
	}

	var secretMap map[string]string
	if err := json.Unmarshal([]byte(secretValue), &secretMap); err != nil {
		return secretValue, nil
	}

	if licenseKey, exists := secretMap[common.LicenseKey]; exists && licenseKey != "" {
		return licenseKey, nil
	}

	return "", errors.New("license key is empty or not present in the secret")
}

func TestGetSecretFromOCIVault_EdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		secretOCID      string
		vaultRegion     string
		forceNilContent bool
		invalidBase64   bool
		shouldError     bool
		expectedError   string
	}{
		{
			name:            "nil secret content",
			secretOCID:      "ocid1.vaultsecret.test",
			vaultRegion:     "us-phoenix-1",
			forceNilContent: true,
			shouldError:     true,
			expectedError:   "secret content is nil",
		},
		{
			name:          "invalid base64 content",
			secretOCID:    "ocid1.vaultsecret.test",
			vaultRegion:   "us-phoenix-1",
			invalidBase64: true,
			shouldError:   true,
			expectedError: "failed to decode secret content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockOCISecretsClient{
				shouldError:     false,
				forceNilContent: tt.forceNilContent,
				invalidBase64:   tt.invalidBase64,
			}

			secret, err := getSecretFromOCIVault(context.Background(), mockClient, tt.secretOCID, tt.vaultRegion)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got nil", tt.expectedError)
				} else if !contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', but got '%s'", tt.expectedError, err.Error())
				}
				if secret != "" {
					t.Errorf("Expected empty secret on error, but got '%s'", secret)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
			}
		})
	}
}

// TestGetLicenseKey_CachesWithinTTL verifies repeated calls within the TTL window reuse the
// cached result instead of hitting Vault again, mirroring
// TestNewNRClient_CacheExpiration's approach for the logs client cache.
func TestGetLicenseKey_CachesWithinTTL(t *testing.T) {
	resetLicenseKeyCache()
	assert.NoError(t, os.Setenv(common.ClientTTL, "1"))
	defer os.Unsetenv(common.ClientTTL)

	_, _ = GetLicenseKey()
	firstCachedAt := licenseKeyCachedAt

	_, _ = GetLicenseKey()
	assert.Equal(t, firstCachedAt, licenseKeyCachedAt, "second call within TTL should not re-fetch")

	time.Sleep(2 * time.Second)

	_, _ = GetLicenseKey()
	assert.True(t, licenseKeyCachedAt.After(firstCachedAt), "call after TTL expiration should re-fetch")
}

func TestGetLicenseKeyError(t *testing.T) {
	originalSecretOCID := os.Getenv(common.SecretOCID)
	originalVaultRegion := os.Getenv(common.VaultRegion)

	defer func() {
		if originalSecretOCID != "" {
			err := os.Setenv(common.SecretOCID, originalSecretOCID)
			if err != nil {
				t.Errorf("Failed to restore SecretOCID environment variable: %v", err)
			}
		} else {
			err := os.Unsetenv(common.SecretOCID)
			if err != nil {
				t.Errorf("Failed to unset SecretOCID environment variable: %v", err)
			}
		}
		if originalVaultRegion != "" {
			err := os.Setenv(common.VaultRegion, originalVaultRegion)
			if err != nil {
				t.Errorf("Failed to restore VaultRegion environment variable: %v", err)
			}
		} else {
			err := os.Unsetenv(common.VaultRegion)
			if err != nil {
				t.Errorf("Failed to unset VaultRegion environment variable: %v", err)
			}
		}
	}()

	tests := []struct {
		name          string
		secretOCID    string
		vaultRegion   string
		expectedError string
	}{
		{
			name:          "missing secret OCID environment variable",
			secretOCID:    "",
			vaultRegion:   "us-phoenix-1",
			expectedError: "failed to create resource principal configuration provider",
		},
		{
			name:          "missing vault region environment variable",
			secretOCID:    "ocid1.vaultsecret.test",
			vaultRegion:   "",
			expectedError: "failed to create resource principal configuration provider",
		},
		{
			name:          "both environment variables missing",
			secretOCID:    "",
			vaultRegion:   "",
			expectedError: "failed to create resource principal configuration provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			if tt.secretOCID != "" {
				err := os.Setenv(common.SecretOCID, tt.secretOCID)
				if err != nil {
					t.Fatalf("Failed to set SecretOCID environment variable: %v", err)
				}
			} else {
				err := os.Unsetenv(common.SecretOCID)
				if err != nil {
					t.Errorf("Failed to unset SecretOCID environment variable: %v", err)
				}
			}
			if tt.vaultRegion != "" {
				err := os.Setenv(common.VaultRegion, tt.vaultRegion)
				if err != nil {
					t.Fatalf("Failed to set VaultRegion environment variable: %v", err)
				}
			} else {
				err := os.Unsetenv(common.VaultRegion)
				if err != nil {
					t.Errorf("Failed to unset VaultRegion environment variable: %v", err)
				}
			}

			resetLicenseKeyCache()
			key, err := GetLicenseKey()

			if err == nil {
				t.Errorf("Expected error, but got nil")
			} else if !contains(err.Error(), tt.expectedError) {
				t.Errorf("Expected error containing '%s', but got '%s'", tt.expectedError, err.Error())
			}
			if key != "" {
				t.Errorf("Expected empty key on error, but got '%s'", key)
			}
		})
	}
}

func contains(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr ||
		(len(str) > len(substr) &&
			(str[:len(substr)] == substr || str[len(str)-len(substr):] == substr ||
				func() bool {
					for i := 1; i < len(str)-len(substr)+1; i++ {
						if str[i:i+len(substr)] == substr {
							return true
						}
					}
					return false
				}())))
}
