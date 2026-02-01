package models

// IncidentStatus represents the current state of an incident
type IncidentStatus string

const (
	StatusOpen     IncidentStatus = "OPEN"
	StatusAcked    IncidentStatus = "ACKED"
	StatusResolved IncidentStatus = "RESOLVED"
)

// IncidentState holds the durable state of an incident workflow
type IncidentState struct {
	Service         string         `json:"service"`
	Status          IncidentStatus `json:"status"`
	Alerts          []string       `json:"alerts"`
	EscalationLevel int            `json:"escalation_level"`
	AckedBy         string         `json:"acked_by,omitempty"`
	ResolvedBy      string         `json:"resolved_by,omitempty"`
}

// Signal payloads

// AddAlertSignal is the payload for adding an alert to an incident
type AddAlertSignal struct {
	AlertID string `json:"alert_id"`
}

// AckSignal is the payload for acknowledging an incident
type AckSignal struct {
	Responder string `json:"responder"`
}

// ResolveSignal is the payload for resolving an incident
type ResolveSignal struct {
	Responder string `json:"responder"`
}

// NotifyInput is the input for the Notify activity
type NotifyInput struct {
	Service   string `json:"service"`
	Message   string `json:"message"`
	Level     int    `json:"level"`
	AlertID   string `json:"alert_id,omitempty"`
	Responder string `json:"responder,omitempty"`
}
