// Package unmarshal provides functions to unmarshal events from various sources such as OCI Logging.
package unmarshal

import (
	"encoding/json"
	"io"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/newrelic/oci-log-integration/logs-function/logger"
)

// Defines the event types
const (
	OCI_LOGGING = "ociLogging" // OCI_LOGGING represents the event type for Oracle Cloud Infrastructure logging events.
)

var log = logger.NewLogrusLogger(logger.WithDebugLevel())

// Event represents the unified event structure.
type Event struct {
	EventType       string                 // EventType represents the type of the event.
	OCILoggingEvent common.OCILoggingEvent // OCILoggingEvent represents the Oracle Cloud Infrastructure logging events.
}

// Unmarshal unmarshals the JSON data into the Event struct.
func (event *Event) Unmarshal(in io.Reader) error {
	payloadBytes, err := io.ReadAll(in)
	if err != nil {
		log.Panicf("Error reading incoming payload: %v\n", err)
	}

	log.WithField("payloadSizeBytes", len(payloadBytes)).Debug("read incoming payload")

	var incomingLogEvent common.OCILoggingEvent
	if err := json.Unmarshal(payloadBytes, &incomingLogEvent); err == nil {
		event.EventType = OCI_LOGGING
		event.OCILoggingEvent = incomingLogEvent
		log.WithField("eventType", event.EventType).
			WithField("recordCount", len(incomingLogEvent)).
			Debug("successfully unmarshalled incoming payload")
	} else {
		log.WithField("payloadSizeBytes", len(payloadBytes)).
			WithField("error", err).
			Error("failed to decode incoming log events payload")
		log.Panicf("Error decoding incoming log events payload: %v", err)
	}

	return nil
}
