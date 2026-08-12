package metrics

import (
	"os"
	"testing"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/stretchr/testify/assert"
)

func TestParseTier(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected Tier
	}{
		{"basic", common.MetricsTierBasic, TierBasic},
		{"advanced", common.MetricsTierAdvanced, TierAdvanced},
		{"none", common.MetricsTierNone, TierNone},
		{"empty defaults to none", "", TierNone},
		{"unrecognized defaults to none", "bogus", TierNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ParseTier(tt.value))
		})
	}
}

func TestCurrentTier(t *testing.T) {
	defer os.Unsetenv(common.MetricsTier)

	assert.NoError(t, os.Setenv(common.MetricsTier, common.MetricsTierAdvanced))
	assert.Equal(t, TierAdvanced, CurrentTier())

	assert.NoError(t, os.Unsetenv(common.MetricsTier))
	assert.Equal(t, TierNone, CurrentTier())
}

func TestTierEnabledFor(t *testing.T) {
	tests := []struct {
		name       string
		metric     Tier
		configured Tier
		expected   bool
	}{
		{"basic metric, none configured", TierBasic, TierNone, false},
		{"basic metric, basic configured", TierBasic, TierBasic, true},
		{"basic metric, advanced configured", TierBasic, TierAdvanced, true},
		{"advanced metric, basic configured", TierAdvanced, TierBasic, false},
		{"advanced metric, advanced configured", TierAdvanced, TierAdvanced, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.metric.enabledFor(tt.configured))
		})
	}
}
