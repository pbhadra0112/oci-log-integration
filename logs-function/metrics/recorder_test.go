package metrics

import (
	"os"
	"testing"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockClient struct {
	mock.Mock
}

func (m *mockClient) CreateMetricEntry(metricEntry interface{}) error {
	args := m.Called(metricEntry)
	return args.Error(0)
}

func setTier(t *testing.T, tier string) {
	t.Helper()
	if tier == "" {
		assert.NoError(t, os.Unsetenv(common.MetricsTier))
	} else {
		assert.NoError(t, os.Setenv(common.MetricsTier, tier))
	}
	t.Cleanup(func() {
		assert.NoError(t, os.Unsetenv(common.MetricsTier))
	})
}

func TestRecorder_NilSafe(t *testing.T) {
	var r *Recorder

	assert.NotPanics(t, func() {
		r.Count(TierBasic, "forwarder.invocations", 1, nil)
		r.Summary(TierBasic, "forwarder.pipeline.lag", 1.5, nil)
		r.SetDimension("compartment", "prod")
		err := r.Flush(&mockClient{})
		assert.NoError(t, err)
	})
	assert.Equal(t, TierNone, r.Tier())
}

func TestRecorder_CountGatedByTier(t *testing.T) {
	setTier(t, common.MetricsTierBasic)
	r := NewRecorder(nil)

	r.Count(TierBasic, "forwarder.invocations", 1, map[string]interface{}{"status": "success"})
	r.Count(TierAdvanced, "forwarder.bytes.received", 100, nil) // should be dropped, above configured tier

	assert.Len(t, r.counts, 1)
	assert.Len(t, r.summaries, 0)
}

func TestRecorder_CountAccumulates(t *testing.T) {
	setTier(t, common.MetricsTierBasic)
	r := NewRecorder(nil)

	attrs := map[string]interface{}{"status": "success"}
	r.Count(TierBasic, "forwarder.records.delivered", 5, attrs)
	r.Count(TierBasic, "forwarder.records.delivered", 3, attrs)

	key := metricKey("forwarder.records.delivered", attrs)
	assert.Equal(t, float64(8), r.counts[key].value)
}

func TestRecorder_SummaryAggregates(t *testing.T) {
	setTier(t, common.MetricsTierBasic)
	r := NewRecorder(nil)

	r.Summary(TierBasic, "forwarder.delivery.duration", 1.0, nil)
	r.Summary(TierBasic, "forwarder.delivery.duration", 3.0, nil)
	r.Summary(TierBasic, "forwarder.delivery.duration", 2.0, nil)

	key := metricKey("forwarder.delivery.duration", nil)
	p := r.summaries[key]
	assert.Equal(t, 3, p.count)
	assert.Equal(t, 6.0, p.sum)
	assert.Equal(t, 1.0, p.min)
	assert.Equal(t, 3.0, p.max)
}

func TestRecorder_FlushNoopWhenTierNone(t *testing.T) {
	setTier(t, common.MetricsTierNone)
	r := NewRecorder(nil)
	r.Count(TierBasic, "forwarder.invocations", 1, nil)

	client := &mockClient{}
	err := r.Flush(client)

	assert.NoError(t, err)
	client.AssertNotCalled(t, "CreateMetricEntry", mock.Anything)
}

func TestRecorder_FlushNoopWhenEmpty(t *testing.T) {
	setTier(t, common.MetricsTierBasic)
	r := NewRecorder(nil)

	client := &mockClient{}
	err := r.Flush(client)

	assert.NoError(t, err)
	client.AssertNotCalled(t, "CreateMetricEntry", mock.Anything)
}

func TestRecorder_FlushSendsPayload(t *testing.T) {
	setTier(t, common.MetricsTierBasic)
	r := NewRecorder(map[string]interface{}{"cloud": "oci"})
	r.Count(TierBasic, "forwarder.invocations", 1, map[string]interface{}{"status": "success"})
	r.Summary(TierBasic, "forwarder.delivery.duration", 1.5, nil)

	client := &mockClient{}
	client.On("CreateMetricEntry", mock.Anything).Return(nil)

	err := r.Flush(client)

	assert.NoError(t, err)
	client.AssertNumberOfCalls(t, "CreateMetricEntry", 1)

	payload := client.Calls[0].Arguments[0].([]map[string]interface{})
	assert.Len(t, payload, 1)
	commonSection, ok := payload[0]["common"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "oci", commonSection["attributes"].(map[string]interface{})["cloud"])

	metricsList, ok := payload[0]["metrics"].([]map[string]interface{})
	assert.True(t, ok)
	assert.Len(t, metricsList, 2)
}

// The Metric API requires interval.ms on count/summary data points and rejects a data
// point outright if "attributes" is present but JSON null (as opposed to omitted or an
// object) -- both failure modes are accepted with 202 and silently dropped, so there's no
// error to notice without an explicit assertion on the wire payload.
func TestRecorder_FlushDataPointsIncludeIntervalMs(t *testing.T) {
	setTier(t, common.MetricsTierBasic)
	r := NewRecorder(nil)
	r.Count(TierBasic, "forwarder.records.received", 2, nil)
	r.Summary(TierBasic, "forwarder.delivery.duration", 1.5, nil)

	client := &mockClient{}
	client.On("CreateMetricEntry", mock.Anything).Return(nil)
	assert.NoError(t, r.Flush(client))

	payload := client.Calls[0].Arguments[0].([]map[string]interface{})
	metricsList := payload[0]["metrics"].([]map[string]interface{})
	assert.Len(t, metricsList, 2)
	for _, dp := range metricsList {
		intervalMs, ok := dp["interval.ms"].(int64)
		assert.True(t, ok, "interval.ms must be present on %s", dp["name"])
		assert.Positive(t, intervalMs)
	}
}

func TestRecorder_FlushOmitsAttributesWhenEmptyInsteadOfNull(t *testing.T) {
	setTier(t, common.MetricsTierBasic)
	r := NewRecorder(nil)
	r.Count(TierBasic, "forwarder.records.received", 2, nil)
	r.Count(TierBasic, "forwarder.invocations", 1, map[string]interface{}{"status": "success"})

	client := &mockClient{}
	client.On("CreateMetricEntry", mock.Anything).Return(nil)
	assert.NoError(t, r.Flush(client))

	payload := client.Calls[0].Arguments[0].([]map[string]interface{})
	metricsList := payload[0]["metrics"].([]map[string]interface{})

	var withNilAttrs, withAttrs map[string]interface{}
	for _, dp := range metricsList {
		if dp["name"] == "forwarder.records.received" {
			withNilAttrs = dp
		} else {
			withAttrs = dp
		}
	}

	_, present := withNilAttrs["attributes"]
	assert.False(t, present, "attributes key must be omitted, not set to null, when there are no attributes")

	assert.Equal(t, map[string]interface{}{"status": "success"}, withAttrs["attributes"])
}

func TestRecorder_SetDimensionIgnoresEmpty(t *testing.T) {
	setTier(t, common.MetricsTierBasic)
	r := NewRecorder(map[string]interface{}{"cloud": "oci"})

	r.SetDimension("compartment", "")
	r.SetDimension("log_group", nil)
	r.SetDimension("log_source_type", "audit")

	assert.NotContains(t, r.commonAttrs, "compartment")
	assert.NotContains(t, r.commonAttrs, "log_group")
	assert.Equal(t, "audit", r.commonAttrs["log_source_type"])
}
