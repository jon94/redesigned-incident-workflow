package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"incident-orchestrator/internal/activities"
	"incident-orchestrator/internal/models"
)

const (
	// Signal names
	SignalAddAlert = "add-alert"
	SignalAck      = "ack"
	SignalResolve  = "resolve"

	// Query names
	QueryState = "state"

	// Escalation timer duration
	EscalationTimeout = 30 * time.Second
)

// IncidentWorkflow manages the lifecycle of an incident using durable execution.
func IncidentWorkflow(ctx workflow.Context, service string) (*models.IncidentState, error) {
	logger := workflow.GetLogger(ctx)

	// Initialize incident state
	state := &models.IncidentState{
		Service:         service,
		Status:          models.StatusOpen,
		Alerts:          []string{},
		EscalationLevel: 0,
	}

	// Activity options with retry policy
	activityOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOpts)

	// Register query handler for workflow state
	err := workflow.SetQueryHandler(ctx, QueryState, func() (*models.IncidentState, error) {
		return state, nil
	})
	if err != nil {
		return nil, err
	}

	// Signal channels
	addAlertChan := workflow.GetSignalChannel(ctx, SignalAddAlert)
	ackChan := workflow.GetSignalChannel(ctx, SignalAck)
	resolveChan := workflow.GetSignalChannel(ctx, SignalResolve)

	// Send initial notification
	err = workflow.ExecuteActivity(ctx, activities.SendNotification, models.NotifyInput{
		Service: service,
		Message: fmt.Sprintf("Incident opened for service: %s", service),
		Level:   state.EscalationLevel,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to send initial notification", "error", err)
	}

	// Track if we need to create a new timer
	needNewTimer := true
	var timerFuture workflow.Future

	// Main event loop - continues until incident is resolved
	for state.Status != models.StatusResolved {
		// Create escalation timer if needed and status is OPEN
		if state.Status == models.StatusOpen && needNewTimer {
			timerFuture = workflow.NewTimer(ctx, EscalationTimeout)
			needNewTimer = false
		}

		selector := workflow.NewSelector(ctx)

		// Handle AddAlert signal
		selector.AddReceive(addAlertChan, func(c workflow.ReceiveChannel, more bool) {
			var signal models.AddAlertSignal
			c.Receive(ctx, &signal)

			logger.Info("Received AddAlert signal", "alertID", signal.AlertID)
			state.Alerts = append(state.Alerts, signal.AlertID)

			// Notify about new alert
			_ = workflow.ExecuteActivity(ctx, activities.SendNotification, models.NotifyInput{
				Service: service,
				Message: fmt.Sprintf("New alert added to incident: %s", signal.AlertID),
				Level:   state.EscalationLevel,
				AlertID: signal.AlertID,
			}).Get(ctx, nil)
		})

		// Handle Ack signal
		selector.AddReceive(ackChan, func(c workflow.ReceiveChannel, more bool) {
			var signal models.AckSignal
			c.Receive(ctx, &signal)

			if state.Status == models.StatusOpen {
				logger.Info("Incident acknowledged", "responder", signal.Responder)
				state.Status = models.StatusAcked
				state.AckedBy = signal.Responder

				// Notify about acknowledgment
				_ = workflow.ExecuteActivity(ctx, activities.SendNotification, models.NotifyInput{
					Service:   service,
					Message:   fmt.Sprintf("Incident acknowledged by %s", signal.Responder),
					Level:     state.EscalationLevel,
					Responder: signal.Responder,
				}).Get(ctx, nil)
			}
		})

		// Handle Resolve signal
		selector.AddReceive(resolveChan, func(c workflow.ReceiveChannel, more bool) {
			var signal models.ResolveSignal
			c.Receive(ctx, &signal)

			logger.Info("Incident resolved", "responder", signal.Responder)
			state.Status = models.StatusResolved
			state.ResolvedBy = signal.Responder

			// Notify about resolution
			_ = workflow.ExecuteActivity(ctx, activities.SendNotification, models.NotifyInput{
				Service:   service,
				Message:   fmt.Sprintf("Incident resolved by %s", signal.Responder),
				Level:     state.EscalationLevel,
				Responder: signal.Responder,
			}).Get(ctx, nil)
		})

		// Handle escalation timer (only if OPEN and timer exists)
		if state.Status == models.StatusOpen && timerFuture != nil {
			selector.AddFuture(timerFuture, func(f workflow.Future) {
				// Check if timer completed (not cancelled by status change)
				if state.Status != models.StatusOpen {
					return
				}

				err := f.Get(ctx, nil)
				if err != nil {
					return
				}

				// Escalate
				state.EscalationLevel++
				logger.Info("Escalating incident", "level", state.EscalationLevel)

				// Notify about escalation
				_ = workflow.ExecuteActivity(ctx, activities.SendNotification, models.NotifyInput{
					Service: service,
					Message: fmt.Sprintf("ESCALATION: Incident not acknowledged after %v, escalating to level %d",
						EscalationTimeout, state.EscalationLevel),
					Level: state.EscalationLevel,
				}).Get(ctx, nil)

				// Signal that we need a new timer on next iteration
				needNewTimer = true
			})
		}

		selector.Select(ctx)
	}

	logger.Info("Incident workflow completed",
		"service", service,
		"finalStatus", state.Status,
		"totalAlerts", len(state.Alerts),
		"escalationLevel", state.EscalationLevel,
	)

	return state, nil
}
