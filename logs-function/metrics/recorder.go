package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Recorder accumulates forwarder.* metric observations for a single function invocation.
// It is created once per invocation and threaded through the unmarshal -> batch -> deliver
// path, then flushed once at the end. All methods are safe to call on a nil *Recorder (a
// no-op), so callers that don't care about metrics (e.g. most existing unit tests) can pass
// nil instead of threading a real Recorder through.
type Recorder struct {
	mu          sync.Mutex
	tier        Tier
	commonAttrs map[string]interface{}
	counts      map[string]*countPoint
	summaries   map[string]*summaryPoint
	createdAt   time.Time
}

type countPoint struct {
	name  string
	attrs map[string]interface{}
	value float64
}

type summaryPoint struct {
	name  string
	attrs map[string]interface{}
	count int
	sum   float64
	min   float64
	max   float64
}

// NewRecorder creates a Recorder for one invocation, freezing the configured tier and the
// invocation's common dimensions (cloud, region, version, function_name, ...) at creation time.
func NewRecorder(commonAttrs map[string]interface{}) *Recorder {
	if commonAttrs == nil {
		commonAttrs = map[string]interface{}{}
	}
	return &Recorder{
		tier:        CurrentTier(),
		commonAttrs: commonAttrs,
		counts:      make(map[string]*countPoint),
		summaries:   make(map[string]*summaryPoint),
		createdAt:   time.Now(),
	}
}

// Tier returns the tier this recorder was configured with.
func (r *Recorder) Tier() Tier {
	if r == nil {
		return TierNone
	}
	return r.tier
}

// Count adds value to a counter metric, gated by tier. attrs may be nil.
func (r *Recorder) Count(tier Tier, name string, value float64, attrs map[string]interface{}) {
	if r == nil || !tier.enabledFor(r.tier) {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := metricKey(name, attrs)
	p, ok := r.counts[key]
	if !ok {
		p = &countPoint{name: name, attrs: attrs}
		r.counts[key] = p
	}
	p.value += value
}

// Summary records one observation of a distribution-style metric (durations, sizes, lag),
// gated by tier. Observations sharing the same name+attrs within an invocation are
// aggregated into a single New Relic "summary" data point (count/sum/min/max) at flush time.
func (r *Recorder) Summary(tier Tier, name string, value float64, attrs map[string]interface{}) {
	if r == nil || !tier.enabledFor(r.tier) {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := metricKey(name, attrs)
	p, ok := r.summaries[key]
	if !ok {
		p = &summaryPoint{name: name, attrs: attrs, min: value, max: value}
		r.summaries[key] = p
	}
	p.count++
	p.sum += value
	if value < p.min {
		p.min = value
	}
	if value > p.max {
		p.max = value
	}
}

// SetDimension merges an additional common attribute (e.g. compartment, log_group) into
// every metric this invocation flushes.
func (r *Recorder) SetDimension(key string, value interface{}) {
	if r == nil || value == nil || value == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.commonAttrs[key] = value
}

// Flush builds the New Relic dimensional-metrics payload for everything recorded this
// invocation and posts it once via client. It is a no-op if the tier is TierNone, if nothing
// was recorded, or if the recorder/client is nil.
func (r *Recorder) Flush(client ClientAPI) error {
	if r == nil || client == nil || r.tier == TierNone {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.counts) == 0 && len(r.summaries) == 0 {
		return nil
	}

	now := time.Now().Unix()
	// The Metric API silently drops count/summary data points that omit interval.ms (it's a
	// required field for those types, not just recommended) -- rather than erroring, so this
	// is easy to miss. Use the recorder's own lifetime as the window it's reporting over.
	intervalMs := time.Since(r.createdAt).Milliseconds()
	if intervalMs < 1 {
		intervalMs = 1
	}
	dataPoints := make([]map[string]interface{}, 0, len(r.counts)+len(r.summaries))

	for _, p := range r.counts {
		dp := map[string]interface{}{
			"name":        p.name,
			"type":        "count",
			"value":       p.value,
			"timestamp":   now,
			"interval.ms": intervalMs,
		}
		// The Metric API rejects a data point outright if "attributes" is present but
		// null (as opposed to omitted or an object), so only set it when non-empty.
		if len(p.attrs) > 0 {
			dp["attributes"] = p.attrs
		}
		dataPoints = append(dataPoints, dp)
	}

	for _, p := range r.summaries {
		dp := map[string]interface{}{
			"name":      p.name,
			"type":      "summary",
			"timestamp": now,
			"value": map[string]interface{}{
				"count": p.count,
				"sum":   p.sum,
				"min":   p.min,
				"max":   p.max,
			},
			"interval.ms": intervalMs,
		}
		if len(p.attrs) > 0 {
			dp["attributes"] = p.attrs
		}
		dataPoints = append(dataPoints, dp)
	}

	payload := []map[string]interface{}{
		{
			"common": map[string]interface{}{
				"timestamp":  now,
				"attributes": r.commonAttrs,
			},
			"metrics": dataPoints,
		},
	}

	return client.CreateMetricEntry(payload)
}

func metricKey(name string, attrs map[string]interface{}) string {
	if len(attrs) == 0 {
		return name
	}

	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(name)
	for _, k := range keys {
		fmt.Fprintf(&b, "|%s=%v", k, attrs[k])
	}
	return b.String()
}
