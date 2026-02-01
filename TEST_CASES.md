# Test Cases — Incident Orchestrator

This file documents manual test scenarios for verifying the incident workflow.

---

## Prerequisites

Before running any tests, ensure:

1. **Temporal server is running:**
   ```bash
   temporal server start-dev --ui-port 8233
   ```
   - This starts the Temporal development server locally
   - Web UI available at: http://localhost:8233
   - gRPC endpoint: localhost:7233

2. **Worker is running:**
   ```bash
   cd incident-orchestrator
   go run cmd/worker/main.go
   ```
   - The worker polls Temporal for tasks
   - It executes workflows and activities
   - Keep this running in a separate terminal

---

## Command Reference

### `go run cmd/starter/main.go -cmd=start -service=<name>`

**What it does:**
- Creates a new workflow instance in Temporal
- Workflow ID is deterministic: `incident-<service>` (e.g., `incident-payment-api`)
- Initial state: `status=OPEN`, `alerts=[]`, `escalationLevel=0`
- Starts a 5-minute escalation timer
- Sends an "Incident opened" notification

**Temporal API used:** `client.ExecuteWorkflow()`

**Example:**
```bash
go run cmd/starter/main.go -cmd=start -service=payment-api
```

**Expected output:**
```
Started incident workflow for service: payment-api
  Workflow ID: incident-payment-api
  Run ID: <uuid>
```

---

### `go run cmd/starter/main.go -cmd=alert -service=<name> -alert=<alert-id>`

**What it does:**
- Sends an `AddAlert` signal to a running workflow
- The workflow receives the signal and appends the alert ID to its `alerts` list
- Sends a "New alert added" notification
- Does NOT affect the escalation timer

**Temporal API used:** `client.SignalWorkflow()` with signal name `add-alert`

**Example:**
```bash
go run cmd/starter/main.go -cmd=alert -service=payment-api -alert=ALERT-001
```

**Expected output:**
```
Sent AddAlert signal: alertID=ALERT-001 to incident-payment-api
```

**State change:** `alerts: [] → ["ALERT-001"]`

---

### `go run cmd/starter/main.go -cmd=ack -service=<name> -responder=<name>`

**What it does:**
- Sends an `Ack` signal to a running workflow
- Only works if status is `OPEN` (ignored if already `ACKED` or `RESOLVED`)
- Changes status from `OPEN` → `ACKED`
- Records who acknowledged (`acked_by` field)
- **Cancels the escalation timer** (no more escalations)
- Sends an "Incident acknowledged" notification

**Temporal API used:** `client.SignalWorkflow()` with signal name `ack`

**Example:**
```bash
go run cmd/starter/main.go -cmd=ack -service=payment-api -responder=alice
```

**Expected output:**
```
Sent Ack signal: responder=alice to incident-payment-api
```

**State change:** `status: OPEN → ACKED`, `acked_by: "" → "alice"`

---

### `go run cmd/starter/main.go -cmd=resolve -service=<name> -responder=<name>`

**What it does:**
- Sends a `Resolve` signal to a running workflow
- Works from any status (`OPEN` or `ACKED`)
- Changes status to `RESOLVED`
- Records who resolved (`resolved_by` field)
- Cancels escalation timer if still running
- Sends an "Incident resolved" notification
- **Workflow completes and exits**

**Temporal API used:** `client.SignalWorkflow()` with signal name `resolve`

**Example:**
```bash
go run cmd/starter/main.go -cmd=resolve -service=payment-api -responder=alice
```

**Expected output:**
```
Sent Resolve signal: responder=alice to incident-payment-api
```

**State change:** `status: ACKED → RESOLVED`, `resolved_by: "" → "alice"`

---

### `go run cmd/starter/main.go -cmd=status -service=<name>`

**What it does:**
- Queries the current state of a running workflow (read-only)
- Does NOT modify the workflow in any way
- Returns the full `IncidentState` as JSON

**Temporal API used:** `client.QueryWorkflow()` with query name `state`

**Example:**
```bash
go run cmd/starter/main.go -cmd=status -service=payment-api
```

**Expected output:**
```json
Incident State:
{
  "service": "payment-api",
  "status": "OPEN",
  "alerts": ["ALERT-001", "ALERT-002"],
  "escalation_level": 0
}
```

---

## Test Case 1: Happy Path (Basic Flow)

**Objective:** Verify the complete incident lifecycle works correctly.

```bash
# 1. Start a new incident
go run cmd/starter/main.go -cmd=start -service=test-service

# 2. Add some alerts
go run cmd/starter/main.go -cmd=alert -service=test-service -alert=ALERT-001
go run cmd/starter/main.go -cmd=alert -service=test-service -alert=ALERT-002

# 3. Check status (should be OPEN with 2 alerts)
go run cmd/starter/main.go -cmd=status -service=test-service

# 4. Acknowledge the incident
go run cmd/starter/main.go -cmd=ack -service=test-service -responder=alice

# 5. Check status (should be ACKED)
go run cmd/starter/main.go -cmd=status -service=test-service

# 6. Resolve the incident
go run cmd/starter/main.go -cmd=resolve -service=test-service -responder=alice
```

**Expected final state:**
```json
{
  "service": "test-service",
  "status": "RESOLVED",
  "alerts": ["ALERT-001", "ALERT-002"],
  "escalation_level": 0,
  "acked_by": "alice",
  "resolved_by": "alice"
}
```

---

## Test Case 2: Escalation Timer

**Objective:** Verify that unacknowledged incidents escalate after 30 seconds.

> **Current timeout:** `EscalationTimeout = 30 * time.Second` in `internal/workflows/incident_workflow.go`
>
> **Important:** If you've already run this test, use a NEW service name (e.g., `esc-test-2`, `esc-test-3`)
> because existing workflows retain their original timer values even after code changes.

```bash
# 1. Start a new incident (use a unique name each time you test!)
go run cmd/starter/main.go -cmd=start -service=esc-test-1

# 2. Check status immediately
go run cmd/starter/main.go -cmd=status -service=esc-test-1
# Expected: escalation_level: 0

# 3. Wait 30 seconds...

# 4. Check status again
go run cmd/starter/main.go -cmd=status -service=esc-test-1
# Expected: escalation_level: 1

# 5. Wait another 30 seconds...

# 6. Check status
go run cmd/starter/main.go -cmd=status -service=esc-test-1
# Expected: escalation_level: 2

# 7. Acknowledge to stop further escalations
go run cmd/starter/main.go -cmd=ack -service=esc-test-1 -responder=bob

# 8. Resolve
go run cmd/starter/main.go -cmd=resolve -service=esc-test-1 -responder=bob
```

**Worker logs should show:**
```
[NOTIFY] Service: esc-test-1 | Level: 1 | Message: ESCALATION: Incident not acknowledged after 30s, escalating to level 1
```

---

## Test Case 3: Durability Test (Worker Crash Recovery)

**Objective:** Prove that Temporal's durable execution survives worker crashes.

This is the most important test — it demonstrates that even if your worker process 
dies, the workflow state is preserved and execution resumes correctly.

### Step-by-step instructions:

```bash
# ============================================
# TERMINAL 1: Temporal Server (keep running)
# ============================================
temporal server start-dev --ui-port 8233


# ============================================
# TERMINAL 2: Worker (we will kill this)
# ============================================
cd incident-orchestrator
go run cmd/worker/main.go


# ============================================
# TERMINAL 3: Test commands
# ============================================
cd incident-orchestrator

# 1. Start an incident
go run cmd/starter/main.go -cmd=start -service=durability-test

# 2. Add an alert
go run cmd/starter/main.go -cmd=alert -service=durability-test -alert=ALERT-001

# 3. Check status
go run cmd/starter/main.go -cmd=status -service=durability-test
# Expected: status=OPEN, alerts=["ALERT-001"]

# 4. ⚠️  NOW KILL THE WORKER IN TERMINAL 2 (Ctrl+C)
#    The workflow is now "paused" - no worker to execute it

# 5. Try to query status (THIS WILL FAIL - queries need a worker!)
go run cmd/starter/main.go -cmd=status -service=durability-test
# Expected: Error - queries require a running worker to execute workflow code
# Note: Check the Temporal UI instead - the state is preserved there

# 6. Send a signal while worker is down (THIS WORKS - signals are queued!)
go run cmd/starter/main.go -cmd=alert -service=durability-test -alert=ALERT-002
# Signal is queued in Temporal, waiting for a worker

# 7. ⚠️  RESTART THE WORKER IN TERMINAL 2
go run cmd/worker/main.go

# 8. Check status (the queued signal was processed!)
go run cmd/starter/main.go -cmd=status -service=durability-test
# Expected: alerts=["ALERT-001", "ALERT-002"] ← Both alerts are there!

# 9. Complete the workflow
go run cmd/starter/main.go -cmd=ack -service=durability-test -responder=charlie
go run cmd/starter/main.go -cmd=resolve -service=durability-test -responder=charlie
```

### What this proves:

| Scenario | Result |
|----------|--------|
| Worker dies mid-workflow | Workflow state preserved in Temporal |
| Signals sent while worker is down | ✅ Queued and delivered when worker restarts |
| Queries while worker is down | ❌ Fail - queries need worker to run workflow code |
| Worker restarts | Workflow resumes from exact point it stopped |
| Timer was running when worker died | Timer continues correctly after restart |

### Verify in Web UI:

1. Open http://localhost:8233
2. Find workflow `durability-test`
3. Look at the **Event History** tab — you'll see all events including:
   - `WorkflowExecutionStarted`
   - `ActivityTaskCompleted` (notifications)
   - `WorkflowExecutionSignaled` (your signals)
   - `TimerStarted` / `TimerCanceled`
   - `WorkflowExecutionCompleted`

---

## Test Case 4: Direct Resolve (Skip Ack)

**Objective:** Verify that you can resolve an incident directly without acknowledging first.

```bash
# 1. Start incident
go run cmd/starter/main.go -cmd=start -service=direct-resolve-test

# 2. Resolve immediately (skip ack)
go run cmd/starter/main.go -cmd=resolve -service=direct-resolve-test -responder=dave

# 3. Verify final state
# Note: Cannot query after workflow completes, check Web UI instead
```

**Expected:** Workflow completes with `status=RESOLVED`, `acked_by=""` (empty).

---

## Cleanup

After testing, workflows that completed are stored in Temporal's history.
You can view them in the Web UI at http://localhost:8233.

To start fresh:
1. Stop Temporal server
2. Delete `temporal.db` (if using file-based storage)
3. Restart Temporal server
