// Package common provides common constants structs and variables.
package common

// InstrumentationProvider is a parameter necessary for Entity Synthesis at New Relic.
const InstrumentationProvider = "oci"

// InstrumentationName is a parameter necessary for Entity Synthesis at New Relic.
const InstrumentationName = "log-function"

// SecretOCID is the environment variable name for the OCI secret OCID.
const SecretOCID = "SECRET_OCID"

// VaultRegion is the environment variable name for the OCI vault region.
const VaultRegion = "VAULT_REGION"

// NumberOfWorkers defines the number of concurrent worker goroutines for processing log batches.
const NumberOfWorkers = 6

// NewRelicRegion is the name of the environment variable for the New Relic region.
const NewRelicRegion = "NEW_RELIC_REGION"

// DebugEnabled is the name of the environment variable for enabling debug mode.
const DebugEnabled = "DEBUG_ENABLED"

// ClientTTL is the name of the environment variable for setting the NewRelic client cache TTL in seconds.
const ClientTTL = "CLIENT_TTL"

// DefaultClientTTL is the default TTL for the NewRelic client cache in seconds (10 minutes = 600 seconds).
const DefaultClientTTL = 600

// MaxPayloadSize is the maximum size of a payload.
// Reference: https://docs.newrelic.com/docs/logs/log-api/introduction-log-api/#limits
const MaxPayloadSize = 1 * 1024 * 1024 // 1 mb

// Secret field names
const LicenseKey = "licenseKey"

// Message channel size
const MessageChannelSize = 10

// MetricsTier is the name of the environment variable that selects which tier of
// custom forwarder.* metrics is emitted (none/basic/advanced).
const MetricsTier = "FORWARDER_METRICS_TIER"

// Metrics tier values accepted by MetricsTier.
const (
	MetricsTierNone     = "none"
	MetricsTierBasic    = "basic"
	MetricsTierAdvanced = "advanced"
)

// FunctionNameEnvVar is the environment variable the Fn/OCI Functions runtime injects
// automatically at invocation time with the function's name.
const FunctionNameEnvVar = "FN_FN_NAME"
