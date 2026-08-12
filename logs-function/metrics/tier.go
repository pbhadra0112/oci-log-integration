// Package metrics accumulates and forwards the forwarder.* custom metrics described in
// the OCI Log Forwarder observability design (Phase 2) directly from this function's
// own process, using the same New Relic client library already used to forward logs.
package metrics

import (
	"os"

	"github.com/newrelic/oci-log-integration/logs-function/common"
)

// Tier represents how much custom-metric detail a customer has opted into.
type Tier int

// Tier values are ordered so a higher tier always includes everything in a lower tier.
const (
	TierNone Tier = iota
	TierBasic
	TierAdvanced
)

// ParseTier converts a FORWARDER_METRICS_TIER value into a Tier, defaulting to TierNone
// for anything unrecognized so a typo never silently upgrades a customer's billing tier.
func ParseTier(value string) Tier {
	switch value {
	case common.MetricsTierBasic:
		return TierBasic
	case common.MetricsTierAdvanced:
		return TierAdvanced
	default:
		return TierNone
	}
}

// CurrentTier reads the configured tier from the FORWARDER_METRICS_TIER environment variable.
func CurrentTier() Tier {
	return ParseTier(os.Getenv(common.MetricsTier))
}

// enabledFor reports whether a metric declared at tier t should be emitted under the
// customer's configured tier.
func (t Tier) enabledFor(configured Tier) bool {
	return configured != TierNone && configured >= t
}
