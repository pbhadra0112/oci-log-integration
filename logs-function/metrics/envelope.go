package metrics

import "time"

// Envelope holds the fields OCI's Logging CloudEvents-style envelope carries on every log
// record: https://docs.oracle.com/en-us/iaas/Content/Logging/Reference/top_level_logging_format.htm
//
// Only the time field is extracted for now (needed for forwarder.pipeline.lag). The
// compartment/log-group/source-type dimensions the observability doc suggests are deferred
// to a follow-up PR alongside the advanced metric tier.
type Envelope struct {
	Time    time.Time
	HasTime bool
}

// ExtractEnvelope defensively pulls the time field out of a raw log record. Real-world
// records vary in shape, so a missing or unexpected field is left at its zero value rather
// than causing an error.
func ExtractEnvelope(record map[string]interface{}) Envelope {
	var env Envelope
	if record == nil {
		return env
	}

	if v, ok := record["time"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			env.Time = t
			env.HasTime = true
		}
	}

	return env
}
