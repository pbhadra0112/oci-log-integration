package unmarshal

import (
	"bytes"
	"testing"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/stretchr/testify/assert"
)

// TestUnmarshalJSONOCILoggingData is a unit test function that tests the unmarshaling of a JSON OCI Logging Data event.
// It verifies that the unmarshaled event has the correct structure.
func TestUnmarshalJSONOCILoggingData(t *testing.T) {
	input := []byte(`[
		{"timestamp":"2023-01-01T12:00:00Z","level":"INFO","message":"Application started successfully","service":"web-server","compartmentId":"ocid1.compartment.test"},
		{"timestamp":"2023-01-01T12:01:00Z","level":"ERROR","message":"Database connection failed","service":"web-server","error":"connection timeout","compartmentId":"ocid1.compartment.test"}
	]`)

	expected := Event{
		EventType: OCI_LOGGING,
		OCILoggingEvent: common.OCILoggingEvent{
			map[string]interface{}{
				"timestamp":     "2023-01-01T12:00:00Z",
				"level":         "INFO",
				"message":       "Application started successfully",
				"service":       "web-server",
				"compartmentId": "ocid1.compartment.test",
			},
			map[string]interface{}{
				"timestamp":     "2023-01-01T12:01:00Z",
				"level":         "ERROR",
				"message":       "Database connection failed",
				"service":       "web-server",
				"error":         "connection timeout",
				"compartmentId": "ocid1.compartment.test",
			},
		},
	}

	var event Event
	err := event.Unmarshal(bytes.NewReader(input), nil)
	assert.NoError(t, err)

	assert.Equal(t, expected.EventType, event.EventType)
	assert.Equal(t, expected.OCILoggingEvent, event.OCILoggingEvent)
}

// TestUnmarshalSingleOCILoggingEvent tests unmarshaling a single nested JSON log event
func TestUnmarshalSingleOCILoggingEvent(t *testing.T) {
	input := []byte(`[
		{"timestamp":"2023-01-01T12:00:00Z","level":"WARN","message":"High memory usage detected","service":"api-gateway","memoryUsage":"85%","tenantId":"ocid1.tenant.test"}
	]`)

	expected := Event{
		EventType: OCI_LOGGING,
		OCILoggingEvent: common.OCILoggingEvent{
			map[string]interface{}{
				"timestamp":   "2023-01-01T12:00:00Z",
				"level":       "WARN",
				"message":     "High memory usage detected",
				"service":     "api-gateway",
				"memoryUsage": "85%",
				"tenantId":    "ocid1.tenant.test",
			},
		},
	}

	var event Event
	err := event.Unmarshal(bytes.NewReader(input), nil)
	assert.NoError(t, err)

	assert.Equal(t, expected.EventType, event.EventType)
	assert.Equal(t, expected.OCILoggingEvent, event.OCILoggingEvent)
}

// TestUnmarshalComplexOCILoggingEvent tests unmarshaling complex nested JSON with OCI-specific metadata
func TestUnmarshalComplexOCILoggingEvent(t *testing.T) {
	input := []byte(`[
		{
			"timestamp":"2023-01-01T12:00:00Z",
			"level":"INFO",
			"message":"Request processed",
			"service":"compute-instance",
			"oci":{
				"compartmentId":"ocid1.compartment.test",
				"availabilityDomain":"AD-1",
				"region":"us-phoenix-1"
			},
			"request":{
				"method":"POST",
				"path":"/api/v1/instances",
				"duration":"150ms"
			},
			"user":{
				"tenantId":"ocid1.tenant.test",
				"userId":"ocid1.user.test"
			}
		},
		{
			"timestamp":"2023-01-01T12:02:00Z",
			"level":"DEBUG",
			"message":"Cache miss",
			"service":"object-storage",
			"oci":{
				"bucket":"test-bucket",
				"namespace":"test-namespace",
				"region":"us-ashburn-1"
			},
			"cache":{
				"key":"user-profile-123",
				"ttl":3600
			},
			"metadata":{
				"size":"2.4MB",
				"contentType":"application/json"
			}
		}
	]`)

	expected := Event{
		EventType: OCI_LOGGING,
		OCILoggingEvent: common.OCILoggingEvent{
			map[string]interface{}{
				"timestamp": "2023-01-01T12:00:00Z",
				"level":     "INFO",
				"message":   "Request processed",
				"service":   "compute-instance",
				"oci": map[string]interface{}{
					"compartmentId":      "ocid1.compartment.test",
					"availabilityDomain": "AD-1",
					"region":             "us-phoenix-1",
				},
				"request": map[string]interface{}{
					"method":   "POST",
					"path":     "/api/v1/instances",
					"duration": "150ms",
				},
				"user": map[string]interface{}{
					"tenantId": "ocid1.tenant.test",
					"userId":   "ocid1.user.test",
				},
			},
			map[string]interface{}{
				"timestamp": "2023-01-01T12:02:00Z",
				"level":     "DEBUG",
				"message":   "Cache miss",
				"service":   "object-storage",
				"oci": map[string]interface{}{
					"bucket":    "test-bucket",
					"namespace": "test-namespace",
					"region":    "us-ashburn-1",
				},
				"cache": map[string]interface{}{
					"key": "user-profile-123",
					"ttl": float64(3600),
				},
				"metadata": map[string]interface{}{
					"size":        "2.4MB",
					"contentType": "application/json",
				},
			},
		},
	}

	var event Event
	err := event.Unmarshal(bytes.NewReader(input), nil)
	assert.NoError(t, err)

	assert.Equal(t, expected.EventType, event.EventType)
	assert.Equal(t, expected.OCILoggingEvent, event.OCILoggingEvent)
}
