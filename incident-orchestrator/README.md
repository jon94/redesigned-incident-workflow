# Incident Orchestrator

A Temporal-based workflow orchestrator demonstrating incident lifecycle management using durable execution.

## Overview

This project models an incident response system using Temporal's core features:
- **Durable Execution**: Workflow state survives worker restarts
- **Timers**: Time-based escalation (30s if not acknowledged)
- **Signals**: Human interaction (AddAlert, Ack, Resolve)
- **Activities**: Side Effects with retries (notifications)

## This Demo does not include

- Kafka or message consumers
- Database schema or deduplication  
- HTTP APIs or UI
- Real integrations (Slack, PagerDuty, Git)

All side effects are stubbed/logged.

## Prerequisites

- Go 1.23
- [Temporal CLI](https://docs.temporal.io/cli) installed

## Quick Start

### 1. Start Temporal Server

```bash
./scripts/run-temporal.sh
```

Or manually:
```bash
temporal server start-dev --ui-port 8233
```

Web UI: http://localhost:8233

### 2. Install Dependencies

```bash
cd incident-orchestrator
go mod tidy
```

### 3. Start the Worker

In a new terminal:
```bash
go run cmd/worker/main.go
```

### 4. Run an Incident Scenario

```bash
# Start an incident for a service
go run cmd/starter/main.go -cmd=start -service=payment-api

# Add alerts
go run cmd/starter/main.go -cmd=alert -service=payment-api -alert=ALERT-001
go run cmd/starter/main.go -cmd=alert -service=payment-api -alert=ALERT-002

# Check status
go run cmd/starter/main.go -cmd=status -service=payment-api

# Acknowledge
go run cmd/starter/main.go -cmd=ack -service=payment-api -responder=alice

# Resolve
go run cmd/starter/main.go -cmd=resolve -service=payment-api -responder=alice
```

## Testing Durability

1. Start an incident workflow
2. Kill the worker (Ctrl+C)
3. Restart the worker
4. Send signals - the workflow continues from where it left off

## Workflow States

| Status | Description |
|--------|-------------|
| `OPEN` | Initial state, escalation timer active |
| `ACKED` | Acknowledged by responder, timer cancelled |
| `RESOLVED` | Incident closed, workflow completes |

## Project Structure

```
incident-orchestrator/
├── cmd/
│   ├── worker/main.go      # Temporal worker process
│   └── starter/main.go     # CLI for workflow operations
├── internal/
│   ├── models/types.go     # Data types and signal payloads
│   ├── workflows/          # Workflow definitions
│   │   └── incident_workflow.go
│   └── activities/         # Activity implementations
│       └── notify.go       # Notification (stubbed)
└── scripts/
    ├── run-temporal.sh     # Start Temporal dev server
```

## Signals

| Signal | Payload | Effect |
|--------|---------|--------|
| `add-alert` | `{alertID}` | Adds alert to incident |
| `ack` | `{responder}` | OPEN → ACKED, cancels timer |
| `resolve` | `{responder}` | Any → RESOLVED, workflow completes |

## Workflow ID Convention

Workflow ID: `incident-{service}`

This ensures one active incident per service. Use Temporal's workflow ID reuse policy to handle deduplication.
