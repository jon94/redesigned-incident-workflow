package activities

import (
	"context"
	"log"

	"incident-orchestrator/internal/models"
)

// SendNotification simulates sending notifications to Slack/PagerDuty.
// This is a stub that logs the notification - in production this would
// integrate with real notification services.
//
// This activity is designed to be idempotent/duplicate-tolerant.
func SendNotification(ctx context.Context, input models.NotifyInput) error {
	log.Printf("[NOTIFY] Service: %s | Level: %d | Message: %s",
		input.Service,
		input.Level,
		input.Message,
	)

	if input.AlertID != "" {
		log.Printf("[NOTIFY]   Alert ID: %s", input.AlertID)
	}

	if input.Responder != "" {
		log.Printf("[NOTIFY]   Responder: %s", input.Responder)
	}

	// Stub: In production, this would call Slack/PagerDuty APIs
	// The activity should be idempotent - duplicate calls with the same
	// input should not cause duplicate notifications (use message IDs, etc.)

	return nil
}
